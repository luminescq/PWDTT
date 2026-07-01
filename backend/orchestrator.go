package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"wg-turn-client/core"
)

// wailsLogWriter перехватывает log.Printf и направляет в Wails-события.
// Буферизует записи и флашит каждые 100ms чтобы не блокировать core.
// Параллельно пишет полный лог в файл ~/.config/pwdtt/logs/<session>.log
type wailsLogWriter struct {
	ctx  context.Context
	mu   sync.Mutex
	buf  []logEntry
	stop chan struct{}
	file *os.File
}

const maxLogBuf = 500

type logEntry struct{ level, msg string }

func newSessionLogFile(peerIP string) *os.File {
	dir := filepath.Join(configDir(), "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil
	}
	ts := time.Now().Format("2006-01-02_15-04-05")
	name := ts + "_" + peerIP + ".log"
	f, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil
	}
	return f
}

func (w *wailsLogWriter) start() {
	w.stop = make(chan struct{})
	go func() {
		t := time.NewTicker(100 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				w.flush()
			case <-w.stop:
				w.flush()
				return
			}
		}
	}()
}

func (w *wailsLogWriter) flush() {
	w.mu.Lock()
	if len(w.buf) == 0 {
		w.mu.Unlock()
		return
	}
	batch := w.buf
	w.buf = nil
	w.mu.Unlock()
	for _, e := range batch {
		runtime.EventsEmit(w.ctx, "log", e.level, e.msg)
	}
}

func (w *wailsLogWriter) Write(p []byte) (int, error) {
	msg := strings.TrimRight(string(p), "\n")
	if len(msg) > 20 && msg[4] == '/' {
		msg = strings.TrimSpace(msg[20:])
	}
	level := classifyLevel(msg)

	// Пишем в файл сразу (без буфера)
	if w.file != nil {
		ts := time.Now().Format("15:04:05")
		fmt.Fprintf(w.file, "[%s] [%s] %s\n", ts, level, msg)
	}

	w.mu.Lock()
	if len(w.buf) >= maxLogBuf {
		// Дропаем старейшую запись чтобы не расти бесконечно
		w.buf = w.buf[1:]
	}
	w.buf = append(w.buf, logEntry{level, msg})
	w.mu.Unlock()
	return len(p), nil
}

func classifyLevel(msg string) string {
	low := strings.ToLower(msg)
	switch {
	case strings.Contains(low, "fatal_auth") ||
		strings.Contains(low, "ошибка") ||
		strings.Contains(low, "error") ||
		strings.Contains(low, "fatal") ||
		strings.Contains(low, "фатальн"):
		return "ERROR"
	case strings.Contains(low, "warn") ||
		strings.Contains(low, "не удалось") ||
		strings.Contains(low, "повторим") ||
		strings.Contains(low, "повторяем") ||
		strings.Contains(low, "retry"):
		return "WARN"
	case strings.Contains(low, "debug") ||
		strings.Contains(low, "obfs") ||
		strings.Contains(low, "unwrap") ||
		strings.Contains(low, "wrap:"):
		return "DEBUG"
	default:
		return "INFO"
	}
}

func configDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		base = os.Getenv("HOME")
	}
	dir := filepath.Join(base, "pwdtt")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

func profilePath(name string) string {
	return filepath.Join(configDir(), "profiles", name+".json")
}

// ProfileData — хранится в ~/.config/pwdtt/profiles/<name>.json
type ProfileData struct {
	PeerAddr string   `json:"peer"`
	Password string   `json:"password"`
	Hashes   []string `json:"hashes"`
	Listen   string   `json:"listen,omitempty"`
	TurnHost string   `json:"turn,omitempty"`
	TurnPort string   `json:"port,omitempty"`
	DeviceID string   `json:"device_id,omitempty"`
}

// ConnectParams — runtime параметры от UI.
type ConnectParams struct {
	Profile     string   `json:"profile"`
	CaptchaMode string   `json:"captchaMode"`
	Workers     int      `json:"workers,omitempty"`
	MTU         int      `json:"mtu,omitempty"`
	Hashes      []string `json:"hashes,omitempty"`
}

func loadProfile(name string) (*ProfileData, error) {
	data, err := os.ReadFile(profilePath(name))
	if err != nil {
		return nil, fmt.Errorf("profile %q: %w", name, err)
	}
	var p ProfileData
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("profile %q parse: %w", name, err)
	}
	return &p, nil
}

type coreSession struct {
	c      *core.Core
	doneCh <-chan core.Event // закрывается когда core завершился
}

// Orchestrator — тонкий прокси между Wails UI и core.
// Два состояния: sess != nil / nil.
type Orchestrator struct {
	appCtx        context.Context
	mu            sync.Mutex
	sess          *coreSession
	prevLogWriter io.Writer
	onTray        func(connected bool, rx, tx int64, workers int32)
}

func NewOrchestrator(ctx context.Context, onTray func(bool, int64, int64, int32)) *Orchestrator {
	return &Orchestrator{appCtx: ctx, onTray: onTray}
}

func (o *Orchestrator) Start(p ConnectParams) error {
	o.mu.Lock()
	if o.sess != nil {
		o.mu.Unlock()
		return fmt.Errorf("already running")
	}
	// Резервируем слот
	placeholder := &coreSession{}
	o.sess = placeholder
	o.mu.Unlock()

	sess, err := o.launch(p)
	if err != nil {
		o.mu.Lock()
		if o.sess == placeholder {
			o.sess = nil
		}
		o.mu.Unlock()
		return err
	}

	o.mu.Lock()
	o.sess = sess
	o.mu.Unlock()
	return nil
}

func (o *Orchestrator) launch(p ConnectParams) (*coreSession, error) {
	// Перехватываем стандартный логгер → Wails события
	if _, already := log.Writer().(*wailsLogWriter); !already {
		o.prevLogWriter = log.Writer()
	}
	lw := &wailsLogWriter{ctx: o.appCtx, file: newSessionLogFile(p.Profile)}
	lw.start()
	log.SetOutput(lw)

	prof, err := loadProfile(p.Profile)
	if err != nil {
		return nil, err
	}

	workers := p.Workers
	if workers <= 0 {
		workers = 9
	}

	cfg := core.Config{
		PeerAddr:    prof.PeerAddr,
		Password:    prof.Password,
		Hashes:      prof.Hashes,
		Listen:      prof.Listen,
		TurnHost:    prof.TurnHost,
		TurnPort:    prof.TurnPort,
		DeviceID:    prof.DeviceID,
		Workers:     workers,
		CaptchaMode: p.CaptchaMode,
		MTU:         p.MTU,
	}
	if len(p.Hashes) > 0 {
		cfg.Hashes = p.Hashes
	}

	c := core.New(cfg)
	events, err := c.Start()
	if err != nil {
		return nil, fmt.Errorf("core start: %w", err)
	}

	sess := &coreSession{c: c, doneCh: events}
	go o.forwardEvents(sess)
	return sess, nil
}

func (o *Orchestrator) forwardEvents(sess *coreSession) {
	var connected bool
	for ev := range sess.doneCh {
		switch ev.Type {
		case core.EventState:
			connected = ev.Status == "running"
			runtime.EventsEmit(o.appCtx, "state_changed", ev.Status, "")
			runtime.EventsEmit(o.appCtx, "log", "INFO", fmt.Sprintf("[СОСТОЯНИЕ] %s", ev.Status))
			if !connected && o.onTray != nil {
				o.onTray(false, 0, 0, 0)
			}
		case core.EventStats:
			if o.onTray != nil {
				o.onTray(connected, ev.RxBytes, ev.TxBytes, ev.Workers)
			}
		case core.EventLog:
			runtime.EventsEmit(o.appCtx, "log", ev.Level, ev.Message)
			if strings.Contains(ev.Message, "FATAL_AUTH") {
				friendly := ev.Message
				switch {
				case strings.Contains(friendly, "неверный пароль"):
					friendly = "Неверный пароль подключения"
				case strings.Contains(friendly, "истёк"):
					friendly = "Срок действия пароля истёк"
				case strings.Contains(friendly, "другому устройству"):
					friendly = "Пароль привязан к другому устройству"
				case strings.Contains(friendly, "запрещён"):
					friendly = "Доступ запрещён сервером"
				}
				runtime.EventsEmit(o.appCtx, "error", friendly)
				go func() {
					if sess.c != nil {
						sess.c.Stop()
					}
				}()
			}
		case core.EventError:
			runtime.EventsEmit(o.appCtx, "error", ev.Message)
			runtime.EventsEmit(o.appCtx, "log", "ERROR", fmt.Sprintf("[ОШИБКА] %s", ev.Message))
		case core.EventEvent:
			if ev.Name == "wg_config" {
				turnIPs := sess.c.GetTurnIPs()
				runtime.EventsEmit(o.appCtx, "log", "INFO", "[WG] Применение конфига...")
				if err := applyWGConfig(ev.Data, turnIPs); err != nil {
					msg := fmt.Sprintf("[WG] Ошибка применения конфига: %v", err)
					runtime.EventsEmit(o.appCtx, "error", msg)
					runtime.EventsEmit(o.appCtx, "log", "ERROR", msg)
				} else {
					connected = true
					runtime.EventsEmit(o.appCtx, "state_changed", "running", "")
					runtime.EventsEmit(o.appCtx, "log", "INFO", "[WG] Конфиг применён, туннель активен ✓")
					if o.onTray != nil {
						o.onTray(true, 0, 0, 0)
					}
				}
			}
			runtime.EventsEmit(o.appCtx, "event", ev.Name, ev.Data)
		}
	}
	// Канал закрыт — core завершился
	teardownWG()
	// Останавливаем буферизованный логгер и восстанавливаем оригинальный
	if lw, ok := log.Writer().(*wailsLogWriter); ok {
		select {
		case <-lw.stop:
		default:
			close(lw.stop)
		}
		if lw.file != nil {
			lw.file.Close()
		}
	}
	if o.prevLogWriter != nil {
		log.SetOutput(o.prevLogWriter)
	}
	ts := time.Now().Format("15:04:05")
	runtime.EventsEmit(o.appCtx, "log", "INFO", fmt.Sprintf("[%s] Сессия завершена", ts))
	if o.onTray != nil {
		o.onTray(false, 0, 0, 0)
	}
	o.mu.Lock()
	if o.sess == sess {
		o.sess = nil
	}
	o.mu.Unlock()
	runtime.EventsEmit(o.appCtx, "state_changed", "disconnected", "")
}

func (o *Orchestrator) Stop() {
	o.mu.Lock()
	sess := o.sess
	o.mu.Unlock()
	if sess == nil || sess.c == nil {
		return
	}
	sess.c.Stop()
}

func (o *Orchestrator) SendCaptchaResult(token string) {
	o.mu.Lock()
	sess := o.sess
	o.mu.Unlock()
	if sess == nil || sess.c == nil {
		return
	}
	sess.c.SolveCaptcha(token)
}

func (o *Orchestrator) IsRunning() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.sess != nil && o.sess.c != nil
}
