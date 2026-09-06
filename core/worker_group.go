package core

import (
	"context"
	"log"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	workersPerGroup  = 9
	defaultCycleSecs = 36000

	// maxAttemptsPerAddress — сколько раз пробуем один адрес, прежде чем забанить
	maxAttemptsPerAddress = 3

	// sleepWhenAllBanned — сколько спим, если все адреса забанены
	sleepWhenAllBanned = 60 * time.Second

	// sleepAfterAddressDead — спим после того как адрес умер (перед переходом к следующему)
	sleepAfterAddressDead = 500 * time.Millisecond

	// workerStartDelay — задержка между запуском воркеров
	workerStartDelay = 200 * time.Millisecond

	// workerBatchDelay — задержка между первой и второй волной воркеров (5 секунд)
	workerBatchDelay = 5 * time.Second

	// firstBatchSize — сколько воркеров запускаем в первой волне
	firstBatchSize = 5
)

func WorkerGroup(
	ctx context.Context,
	groupID int,
	hashIndex int,
	tp *TurnParams,
	peer *net.UDPAddr,
	d *Dispatcher,
	localPort string,
	getConfig bool,
	configCh chan<- string,
	turnIPsCh chan<- []string,
	workerIDs []int,
	pauseFlag *int32,
	deviceID, password string,
	stats *Stats,
	waitCreds <-chan struct{},
	signalCreds chan<- struct{},
	waitSpawn <-chan struct{},
	signalSpawn chan<- struct{},
) {

	// signalSpawnOnce гарантирует ровно один close(signalSpawn) на любом пути выхода
	spawnSignaled := false
	signalSpawnOnce := func() {
		if signalSpawn != nil && !spawnSignaled {
			spawnSignaled = true
			close(signalSpawn)
		}
	}

	if waitCreds != nil {
		select {
		case <-waitCreds:
		case <-ctx.Done():
			signalSpawnOnce()
			return
		}
	}

	// Сигнал следующей группе не должен зависеть от успешности GetCreds ниже:
	// иначе один transient-фейл VK API навсегда блокирует все последующие группы
	if signalCreds != nil {
		go func() {
			time.Sleep(2 * time.Second)
			close(signalCreds)
		}()
	}

	var configSent int32
	if !getConfig {
		configSent = 1
	}

	for atomic.LoadInt32(pauseFlag) != 0 {
		if ctx.Err() != nil {
			signalSpawnOnce()
			return
		}
		time.Sleep(1 * time.Second)
	}

	if waitSpawn != nil {
		select {
		case <-waitSpawn:
		case <-ctx.Done():
			signalSpawnOnce()
			return
		}
	}

	hash := tp.Hashes[hashIndex%len(tp.Hashes)]
	shortHash := hash
	if len(shortHash) > 8 {
		shortHash = shortHash[:8]
	}
	log.Printf("[ГРУППА #%d] Запрос кредов (хеш: %s...)", groupID, shortHash)

	credStreamID := groupID * 100

	var credsMu sync.RWMutex
	var curUser, curPass string
	var curURLs []string
	var credGen uint64

	fetchCreds := func() error {
		u, p, urls, err := GetCreds(ctx, hash, credStreamID)
		if err != nil {
			return err
		}
		credsMu.Lock()
		curUser, curPass, curURLs = u, p, urls
		credsMu.Unlock()
		return nil
	}

	// Первичное получение кредов — с ретраями, а не один фейл навсегда
	for attempt := 1; ; attempt++ {
		if err := fetchCreds(); err == nil {
			break
		} else {
			log.Printf("[ГРУППА #%d] Ошибка кредов (попытка %d): %v", groupID, attempt, err)
		}
		select {
		case <-time.After(5 * time.Second):
		case <-ctx.Done():
			signalSpawnOnce()
			return
		}
	}

	credsMu.RLock()
	log.Printf("[ГРУППА #%d] Креды OK, TURN: %v, %d воркеров", groupID, curURLs, len(workerIDs))
	credsMu.RUnlock()

	// Отправляем TURN IP для exclude routes (только первая группа)
	if groupID == 1 && turnIPsCh != nil {
		credsMu.RLock()
		urls := cloneStringSlice(curURLs)
		credsMu.RUnlock()
		if len(urls) > 0 {
			var turnIPs []string
			for _, url := range urls {
				host, _, err := net.SplitHostPort(url)
				if err != nil {
					host = url
				}
				if tp.Host != "" {
					host = tp.Host
				}
				resolved, err := net.LookupIP(host)
				if err != nil {
					turnIPs = append(turnIPs, host)
				} else {
					for _, ip := range resolved {
						turnIPs = append(turnIPs, ip.String())
					}
				}
			}
			// Дедупликация
			seen := make(map[string]bool)
			var unique []string
			for _, ip := range turnIPs {
				if !seen[ip] {
					seen[ip] = true
					unique = append(unique, ip)
				}
			}
			select {
			case turnIPsCh <- unique:
			default:
			}
		}
	}

	var configRequestInFlight int32
	var wg sync.WaitGroup

	// Запускаем воркеры с поэтапной задержкой
	for idx, wid := range workerIDs {
		wg.Add(1)

		// Определяем задержку перед запуском этого воркера
		var startDelay time.Duration
		if idx < firstBatchSize {
			// Первая волна: 5 воркеров с задержкой 200ms
			startDelay = time.Duration(idx) * workerStartDelay
		} else {
			// Вторая волна: задержка = (первая волна) + 5 секунд + (индекс во второй волне) * 200ms
			secondWaveIdx := idx - firstBatchSize
			startDelay = time.Duration(firstBatchSize)*workerStartDelay + workerBatchDelay + time.Duration(secondWaveIdx)*workerStartDelay
		}

		go func(wid int, delay time.Duration) {
			// Ждём свою задержку перед стартом
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				wg.Done()
				return
			}

			defer wg.Done()

			shouldGetConfig := getConfig
			// Счётчик неудач на текущий адрес
			addressAttempts := 0
			// Храним текущий ObfsMode для этого воркера (может меняться при WRAP_TIMEOUT)
			workerObfsMode := tp.ObfsMode

			for {
				if ctx.Err() != nil {
					return
				}

				// Проверяем паузу
				for atomic.LoadInt32(pauseFlag) != 0 {
					if ctx.Err() != nil {
						return
					}
					time.Sleep(1 * time.Second)
				}

				// Берём свежие креды (могут быть обновлены после AUTH-ошибки)
				credsMu.RLock()
				urls := cloneStringSlice(curURLs)
				wUser, wPass := curUser, curPass
				gen := credGen
				credsMu.RUnlock()

				if len(urls) == 0 {
					log.Printf("[ВОРКЕР #%d] Нет TURN-адресов в кредах", wid)
					select {
					case <-time.After(5 * time.Second):
					case <-ctx.Done():
					}
					continue
				}

				// Фильтруем забаненные адреса
				var available []string
				for _, u := range urls {
					if !GlobalBlacklist.IsBanned(u) {
						available = append(available, u)
					}
				}

				// Если все адреса забанены — спим и пробуем заново
				if len(available) == 0 {
					log.Printf("[ВОРКЕР #%d] Все TURN-адреса забанены (всего %d), спим %v",
						wid, len(urls), sleepWhenAllBanned)
					select {
					case <-time.After(sleepWhenAllBanned):
					case <-ctx.Done():
					}
					continue
				}

				// Берём первый доступный адрес
				turnAddr := available[0]

				getConf := false
				if shouldGetConfig && atomic.LoadInt32(&configSent) == 0 {
					getConf = atomic.CompareAndSwapInt32(&configRequestInFlight, 0, 1)
				}
				var cc chan<- string
				if getConf {
					cc = configCh
				}

					// Передаём в RunSession конкретный адрес и креды
					configDelivered, sessErr := RunSession(
						ctx, tp, peer, d, localPort,
						getConf, cc, wid,
						turnAddr,     // конкретный TURN-адрес
						wUser,        // username из кредов
						wPass,        // password из кредов
						workerObfsMode,
						credStreamID, // для handleAuthError
						deviceID, password, stats,
					)

				if getConf {
					if configDelivered {
						atomic.StoreInt32(&configSent, 1)
					} else {
						atomic.StoreInt32(&configRequestInFlight, 0)
					}
				}

				// Обработка ошибок
				if sessErr != nil {
					if ctx.Err() != nil {
						return
					}

					log.Printf("[ВОРКЕР #%d] Ошибка сессии: %v (адрес=%s, тип=%s)",
						wid, sessErr.Err, sessErr.Address, sessErr.Type)

					switch sessErr.Type {

					case SessionErrorAddressDead:
						// Адрес мёртв — баним и пробуем следующий
						if sessErr.Address != "" {
							GlobalBlacklist.Ban(sessErr.Address)
							log.Printf("[ВОРКЕР #%d] TURN-адрес %s забанен на 5 минут", wid, sessErr.Address)
						}
						// Небольшая пауза перед переходом к следующему адресу
						select {
						case <-time.After(sleepAfterAddressDead):
						case <-ctx.Done():
						}
						// Продолжаем цикл — возьмём следующий доступный адрес
						continue

					case SessionErrorWrapTimeout:
						// DTLS-таймаут — пробуем сменить обфускацию.
						// Режим пер-воркерский: глобальная перезапись tp.ObfsMode
						// была гонкой и меняла режим у всех живых сессий
						log.Printf("[ВОРКЕР #%d] WRAP_TIMEOUT на адресе %s, пробуем сменить обфускацию",
							wid, sessErr.Address)

						// Меняем режим обфускации
						if workerObfsMode == "audio" {
							workerObfsMode = "video"
						} else {
							workerObfsMode = "audio"
						}
						log.Printf("[ВОРКЕР #%d] Режим обфускации изменён на %s", wid, workerObfsMode)

						// Увеличиваем счётчик попыток на этом адресе
						addressAttempts++

						// Если слишком много попыток на одном адресе — баним его
						if addressAttempts >= maxAttemptsPerAddress {
							if sessErr.Address != "" {
								GlobalBlacklist.Ban(sessErr.Address)
								log.Printf("[ВОРКЕР #%d] TURN-адрес %s забанен после %d попыток",
									wid, sessErr.Address, addressAttempts)
							}
							addressAttempts = 0
							// Продолжаем цикл — возьмём следующий адрес
							continue
						}

						// Пробуем тот же адрес с новой обфускацией
						continue

					case SessionErrorAuth:
						// Креды протухли (401) — обновляем и продолжаем.
						// Обновляет только один воркер: остальные увидят новый credGen
						log.Printf("[ВОРКЕР #%d] Ошибка авторизации TURN, обновляю креды", wid)
						credsMu.Lock()
						if credGen == gen {
							if err := fetchCreds(); err == nil {
								credGen++
								log.Printf("[ВОРКЕР #%d] Креды обновлены", wid)
							} else {
								log.Printf("[ВОРКЕР #%d] Не удалось обновить креды: %v", wid, err)
							}
						}
						credsMu.Unlock()
						select {
						case <-time.After(2 * time.Second):
						case <-ctx.Done():
							return
						}
						continue

					case SessionErrorFatal:
						// Фатальная ошибка — выходим
						log.Printf("[ВОРКЕР #%d] Фатальная ошибка: %v", wid, sessErr.Err)
						return

					default:
						// Неизвестный тип ошибки — спим и пробуем заново
						log.Printf("[ВОРКЕР #%d] Неизвестная ошибка: %v", wid, sessErr)
						select {
						case <-time.After(5 * time.Second):
						case <-ctx.Done():
						}
						continue
					}
				}

				// Успех — сбрасываем счётчики
				addressAttempts = 0

				// После успешной сессии — небольшая пауза перед следующим циклом
				select {
				case <-time.After(100 * time.Millisecond):
				case <-ctx.Done():
					return
				}
			}
		}(wid, startDelay)
	}

	signalSpawnOnce()

	wg.Wait()
	log.Printf("[ГРУППА #%d] Все воркеры группы завершились.", groupID)
}

func ParseHashes(raw string) []string {
	var result []string
	seen := make(map[string]struct{})
	for _, h := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	}) {
		h = normalizeVKJoinHash(h)
		if h != "" {
			if _, exists := seen[h]; exists {
				continue
			}
			seen[h] = struct{}{}
			result = append(result, h)
		}
	}
	return result
}

func normalizeVKJoinHash(input string) string {
	s := strings.Trim(strings.TrimSpace(input), "<>\"'")
	if s == "" {
		return ""
	}

	lower := strings.ToLower(s)
	if idx := strings.Index(lower, "/call/join/"); idx >= 0 {
		s = s[idx+len("/call/join/"):]
	} else if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return ""
	}

	if idx := strings.IndexAny(s, "?#/"); idx != -1 {
		s = s[:idx]
	}
	return strings.Trim(strings.TrimSpace(s), "/")
}

type TurnParams struct {
	Host         string
	Port         string
	Hashes       []string
	WrapKey      []byte
	ObfsMode     string
	TCPTransport bool
}