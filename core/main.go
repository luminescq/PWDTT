package core

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ═══════════════════════════════════════════════════
// EVENT TYPES — типы событий от ядра к Bridge
// ═══════════════════════════════════════════════════

type EventType string

const (
	EventState EventType = "state" // подключение/отключение
	EventLog   EventType = "log"   // лог-сообщения
	EventStats EventType = "stats" // статистика трафика
	EventError EventType = "error" // ошибки
	EventEvent EventType = "event" // события (wg_config, captcha, ready)
)

// Event — событие от ядра.
type Event struct {
	Type EventType

	// state
	Status string // "connecting", "connected", "disconnected"

	// log
	Level   string // "INFO", "WARN", "ERROR", "DEBUG"
	Message string

	// event
	Name   string   // "wg_config", "captcha_required", "ready"
	Data   string   // JSON строка
	TurnIPs []string // IP-адреса TURN-серверов (для exclude routes)

	// stats
	Active    int32
	BytesUp   int64
	BytesDown int64
}

// ═══════════════════════════════════════════════════
// CONFIG — параметры запуска ядра (заменяют flag)
// ═══════════════════════════════════════════════════

type Config struct {
	PeerAddr    string   // адрес:порт VPS сервера
	Password    string   // пароль подключения
	Hashes      []string // хеши VK-звонков
	DeviceID    string   // уникальный ID устройства
	Workers     int      // количество воркеров (кратно 9)
	Listen      string   // локальный адрес (default 127.0.0.1:9000)
	TurnHost    string   // переопределение IP TURN
	TurnPort    string   // переопределение порта TURN
	CaptchaMode string   // auto/rjs/wv
	ObfsMode    string   // audio/video
	Fingerprint string   // chrome/android/ios/safari/firefox
	TurnTCP     bool     // использовать TCP транспорт
}

// ═══════════════════════════════════════════════════
// CORE — главный объект ядра
// ═══════════════════════════════════════════════════

type Core struct {
	cfg               Config
	ctx               context.Context
	cancel            context.CancelFunc
	pauseFlag         int32
	CaptchaResultChan chan string
	captchaMode       atomic.Value
	vkAuthMode        atomic.Value
	events            chan Event
	startOnce         sync.Once // guard от двойного Start()
	startErr          atomic.Value // *startFailure — ошибка первого Start(), если он провалился
	once              sync.Once // guard от двойного Stop()
}

// startFailure — обёртка для atomic.Value (чтобы разные error-типы не паниковали).
type startFailure struct{ err error }

// activeCore — текущий запущенный экземпляр ядра.
// Нужен для доступа из session.go, vk_auth.go и др. файлов.
var (
	activeCore     *Core
	activeCoreMu   sync.RWMutex
)

func setActiveCore(c *Core) {
	activeCoreMu.Lock()
	activeCore = c
	activeCoreMu.Unlock()
}

func getActiveCore() *Core {
	activeCoreMu.RLock()
	defer activeCoreMu.RUnlock()
	return activeCore
}

// New создаёт ядро с конфигом.
func New(cfg Config) *Core {
	if cfg.Listen == "" {
		cfg.Listen = "127.0.0.1:9000"
	}
	if cfg.DeviceID == "" {
		cfg.DeviceID = "unknown"
	}
	if cfg.Workers <= 0 {
		cfg.Workers = 24
	}
	if cfg.ObfsMode == "" {
		cfg.ObfsMode = "audio"
	}

	ctx, cancel := context.WithCancel(context.Background())
	c := &Core{
		cfg:               cfg,
		ctx:               ctx,
		cancel:            cancel,
		CaptchaResultChan: make(chan string, 1),
		events:            make(chan Event, 1024), // увеличен буфер для логов
	}
	c.captchaMode.Store(normalizeCaptchaMode(cfg.CaptchaMode))
	c.vkAuthMode.Store(normalizeVKAuthMode("vkcalls"))
	return c
}

// Start запускает ядро. Возвращает канал событий (закрывается при завершении).
// Повторный вызов возвращает ту же ошибку, если первый Start провалился —
// иначе звонящий получил бы «успех» без работающего ядра.
func (c *Core) Start() (<-chan Event, error) {
	var startErr error
	c.startOnce.Do(func() {
		startErr = c.start()
		if startErr != nil {
			c.startErr.Store(&startFailure{err: startErr})
		}
	})
	if startErr != nil {
		return nil, startErr
	}
	if f, ok := c.startErr.Load().(*startFailure); ok {
		return nil, fmt.Errorf("core start previously failed: %w", f.err)
	}
	return c.events, nil
}

func (c *Core) start() error {
	// Если start() упадёт до запуска success-горутины (валидация, resolve,
	// listen) — глобальный логгер останется направленным на брошенное ядро,
	// чей events-канал никто не читает, и все последующие log.Printf
	// будут молча пропадать. Возвращаем stderr на любом error-пути.
	startFailed := true
	defer func() {
		if startFailed {
			log.SetOutput(os.Stderr)
		}
	}()

	setupGlobalResolver()

	// Все log.Printf → в канал событий (с фильтрацией)
	// Без таймстемпа — UI добавляет своё время
	log.SetFlags(0)
	log.SetOutput(&logWriter{core: c})

	// Подменяем глобальный CaptchaResultChan на канал из Core
	CaptchaResultChan = c.CaptchaResultChan

	// Устанавливаем activeCore для доступа из session/vk_auth
	setActiveCore(c)

	if c.cfg.Fingerprint != "" {
		SetActiveFingerprint(c.cfg.Fingerprint)
	}

	ctx := c.ctx
	cancel := c.cancel

	// Валидация
	if c.cfg.PeerAddr == "" {
		cancel()
		return fmt.Errorf("PeerAddr is required")
	}
	if len(c.cfg.Hashes) == 0 {
		cancel()
		return fmt.Errorf("Hashes are required")
	}
	if c.cfg.Password == "" {
		cancel()
		return fmt.Errorf("Password is required")
	}

	// Резолв пира
	cleanPeerAddr := strings.TrimSpace(c.cfg.PeerAddr)
	var peer *net.UDPAddr
	var err error
	for i := 0; i < 15; i++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		peer, err = net.ResolveUDPAddr("udp", cleanPeerAddr)
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	if err != nil {
		cancel()
		return fmt.Errorf("resolve peer: %w", err)
	}

	// WRAP ключ
	wrapKey, err := deriveWrapKey(c.cfg.Password)
	if err != nil {
		cancel()
		return fmt.Errorf("derive wrap key: %w", err)
	}

	// Нормализация воркеров
	n := c.cfg.Workers
	if n > 108 {
		n = 108
	}
	if n < workersPerGroup {
		n = workersPerGroup
	}
	n = (n / workersPerGroup) * workersPerGroup

	tp := &TurnParams{
		Host:         c.cfg.TurnHost,
		Port:         c.cfg.TurnPort,
		Hashes:       c.cfg.Hashes,
		WrapKey:      wrapKey,
		ObfsMode:     c.cfg.ObfsMode,
		TCPTransport: c.cfg.TurnTCP,
	}

	// Локальный UDP сокет
	var localConn net.PacketConn
	actualListenAddr := c.cfg.Listen
	for i := 0; i < 5; i++ {
		localConn, err = net.ListenPacket("udp", actualListenAddr)
		if err == nil {
			break
		}
		log.Printf("[ЯДРО] Порт %s занят, жду... (%d/5)", actualListenAddr, i+1)
		time.Sleep(1 * time.Second)
	}
	if err != nil {
		actualListenAddr = "127.0.0.1:0"
		localConn, err = net.ListenPacket("udp", actualListenAddr)
		if err != nil {
			cancel()
			return fmt.Errorf("listen: %w", err)
		}
	}
	if uc, ok := localConn.(*net.UDPConn); ok {
		_ = uc.SetReadBuffer(socketBufSize)
		_ = uc.SetWriteBuffer(socketBufSize)
	}
	context.AfterFunc(ctx, func() { _ = localConn.Close() })

	_, localPort, _ := net.SplitHostPort(localConn.LocalAddr().String())
	if localPort == "" {
		localPort = "9000"
	}

	numGroups := n / workersPerGroup

	// Статистика
	stats := NewStats()
	shutdownCh := make(chan struct{})
	go func() {
		<-ctx.Done()
		close(shutdownCh)
	}()
	go stats.RunLoop(shutdownCh,
		func(level, msg string) {
			c.emit(Event{Type: EventLog, Level: level, Message: msg})
		},
		func(rx, tx int64, workers int32) {
			c.emit(Event{Type: EventStats, Active: workers, BytesUp: rx, BytesDown: tx})
		},
	)

	// Диспетчер
	disp := NewDispatcher(ctx, localConn, stats)

	// Конфиг WireGuard
	configCh := make(chan string, 1)
	turnIPsCh := make(chan []string, 1)
	go func() {
		select {
		case rawConf, ok := <-configCh:
			if !ok || rawConf == "" {
				return
			}
			finalConf := patchMTU(rawConf)
			var turnIPs []string
			select {
			case ips := <-turnIPsCh:
				turnIPs = ips
			default:
			}
			c.emit(Event{Type: EventEvent, Name: "wg_config", Data: finalConf, TurnIPs: turnIPs})
		case <-ctx.Done():
		}
	}()

	// Старт
	c.emit(Event{Type: EventState, Status: "connecting"})

	startFailed = false // дальнейшая уборка логгера — на success-горутине

	go func() {
		defer close(c.events)
		defer log.SetOutput(os.Stderr) // восстанавливаем лог в stderr
		defer disp.Shutdown()
		defer cancel()

		var wg sync.WaitGroup
		workerIDCounter := 1
		var prevWaitCreds <-chan struct{}
		var prevWaitSpawn <-chan struct{}

		for g := 0; g < numGroups; g++ {
			isFirst := g == 0
			var myWaitCreds <-chan struct{}
			var mySignalCreds chan<- struct{}
			var myWaitSpawn <-chan struct{}
			var mySignalSpawn chan<- struct{}

			if g > 0 {
				myWaitCreds = prevWaitCreds
				myWaitSpawn = prevWaitSpawn
			}
			if g < numGroups-1 {
				chC := make(chan struct{})
				mySignalCreds = chC
				prevWaitCreds = chC
				chS := make(chan struct{})
				mySignalSpawn = chS
				prevWaitSpawn = chS
			}

			ids := make([]int, workersPerGroup)
			for i := range ids {
				ids[i] = workerIDCounter
				workerIDCounter++
			}

			gID := g + 1
			var cc chan<- string
			if isFirst {
				cc = configCh
			}

			wg.Add(1)
			go func(groupID int, isFirstGroup bool, configChan chan<- string, workerIds []int, startHashIndex int, waitC, waitS <-chan struct{}, sigC, sigS chan<- struct{}) {
				defer wg.Done()
				WorkerGroup(ctx, groupID, startHashIndex, tp, peer, disp, localPort,
					isFirstGroup, configChan, turnIPsCh, workerIds, &c.pauseFlag,
					c.cfg.DeviceID, c.cfg.Password, stats, waitC, sigC, waitS, sigS)
			}(gID, isFirst, cc, ids, g, myWaitCreds, myWaitSpawn, mySignalCreds, mySignalSpawn)
		}

		wg.Wait()
		close(configCh)
		log.Println("[ЯДРО] Все воркеры завершены")
	}()

	return nil
}

// Stop останавливает ядро.
func (c *Core) Stop() {
	c.once.Do(func() {
		if c.cancel != nil {
			c.cancel()
		}
	})
}

// Pause приостанавливает воркеры.
func (c *Core) Pause() { atomic.StoreInt32(&c.pauseFlag, 1) }

// Resume возобновляет воркеры.
func (c *Core) Resume() { atomic.StoreInt32(&c.pauseFlag, 0) }

// SolveCaptcha передаёт токен капчи в ядро.
// Атомарно: drain старого + запись нового.
func (c *Core) SolveCaptcha(token string) {
	// Дренируем устаревший токен
	select {
	case <-c.CaptchaResultChan:
	default:
	}
	// Записываем новый (если канал полон — дропаем старый)
	select {
	case c.CaptchaResultChan <- token:
	default:
		<-c.CaptchaResultChan
		c.CaptchaResultChan <- token
	}
}

// ═══════════════════════════════════════════════════
// INTERNAL
// ═══════════════════════════════════════════════════

func (c *Core) emit(ev Event) {
	defer func() { recover() }() // ловим send on closed channel
	select {
	case c.events <- ev:
	default:
		// Канал полон — дропаем старые логи чтобы освободить место
		select {
		case <-c.events:
		default:
		}
		select {
		case c.events <- ev:
		default:
		}
	}
}

func (c *Core) getCaptchaMode() string {
	mode, _ := c.captchaMode.Load().(string)
	if mode == "" {
		return "auto"
	}
	return mode
}

func patchMTU(conf string) string {
	if strings.Contains(conf, "MTU =") {
		return conf
	}
	lines := strings.Split(conf, "\n")
	var out []string
	for _, line := range lines {
		out = append(out, line)
		if strings.TrimSpace(line) == "[Interface]" {
			out = append(out, "MTU = 1280")
		}
	}
	return strings.Join(out, "\n")
}

// ═══════════════════════════════════════════════════
// HELPER FUNCTIONS — нормализация режимов
// ═══════════════════════════════════════════════════

func normalizeCaptchaMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "auto", "rjs", "wv":
		return strings.ToLower(strings.TrimSpace(mode))
	default:
		return "auto"
	}
}

func normalizeVKAuthMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "legacy":
		return "legacy"
	default:
		return "vkcalls"
	}
}

func drainCaptchaResult() {
	select {
	case <-CaptchaResultChan:
	default:
	}
}

// getVKAuthMode возвращает текущий режим VK авторизации.
func (c *Core) getVKAuthMode() string {
	mode, _ := c.vkAuthMode.Load().(string)
	if mode == "" {
		return "vkcalls"
	}
	return mode
}

// getCaptchaMode возвращает текущий режим капчи (для совместимости с пакетом).
var _ = getCaptchaMode

func getCaptchaMode() string {
	if ac := getActiveCore(); ac != nil {
		return ac.getCaptchaMode()
	}
	return "auto"
}

// ═══════════════════════════════════════════════════
// PACKAGE-LEVEL FUNCTIONS — делегируют в activeCore
// ═══════════════════════════════════════════════════

// emitReady — вызывается из session.go.
func emitReady() {
	if ac := getActiveCore(); ac != nil {
		ac.emitReady()
	}
}

// getVKAuthMode — вызывается из vk_auth.go.
func getVKAuthMode() string {
	if ac := getActiveCore(); ac != nil {
		return ac.getVKAuthMode()
	}
	return "vkcalls"
}

// CaptchaResultChan — глобальный канал для совместимости с vk_auth.go.
// При Start() подменяется на канал из Core.
var CaptchaResultChan = make(chan string, 1)

// ═══════════════════════════════════════════════════
// LOG WRITER — перенаправляет log.Printf в канал событий с фильтрацией
// Файловая запись — в Store (через Bridge)

type logWriter struct {
	core *Core
}

func (lw *logWriter) Write(p []byte) (int, error) {
	msg := strings.TrimRight(string(p), "\n")
	if msg == "" {
		return len(p), nil
	}
	level := classifyLevel(msg)

	// ВСЕ ошибки — показываем
	if level == "ERROR" {
		lw.core.emit(Event{Type: EventLog, Level: level, Message: msg})
		return len(p), nil
	}

	// === ПОКАЗЫВАЕМ В UI ===

	if strings.Contains(msg, "Ошибка кредов") ||
		strings.Contains(msg, "VK Auth] Failed") ||
		strings.Contains(msg, "КАПЧА] v2 попытка") || strings.Contains(msg, "КАПЧА] Превышен лимит") ||
		strings.Contains(msg, "[READY]") || strings.Contains(msg, "[WG]") ||
		strings.Contains(msg, "Туннель активен") || strings.Contains(msg, "Туннель готов") ||
		strings.Contains(msg, "[STATS]") || strings.Contains(msg, "[ЯДРО]") {
		lw.core.emit(Event{Type: EventLog, Level: level, Message: msg})
		return len(p), nil
	}

	// === ПРОПУСКАЕМ ДЛЯ UI (но Bridge пишет в файл) ===
	lw.core.emit(Event{Type: EventLog, Level: "SKIP", Message: msg})
	return len(p), nil
}

func classifyLevel(msg string) string {
	low := strings.ToLower(msg)
	switch {
	case strings.Contains(low, "fatal") || strings.Contains(low, "ошибка") || strings.Contains(low, "error"):
		return "ERROR"
	case strings.Contains(low, "warn") || strings.Contains(low, "не удалось") || strings.Contains(low, "retry"):
		return "WARN"
	default:
		return "INFO"
	}
}
