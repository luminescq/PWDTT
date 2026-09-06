package core

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/cbeuw/connutil"
	"github.com/pion/dtls/v3"
	"github.com/pion/dtls/v3/pkg/crypto/selfsign"
	"github.com/pion/turn/v5"
)

const (
	workerSendBuf      = 128
	sessionReadTimeout = 30 * time.Minute
	readBufSize        = 1600
	socketBufSize      = 625 * 1024
	keepaliveByte      = 0xFF
	keepaliveInterval  = 15 * time.Second
)

var handshakeSem = make(chan struct{}, 3)

type connectedUDPConn struct{ *net.UDPConn }

func (c *connectedUDPConn) WriteTo(p []byte, _ net.Addr) (int, error) { return c.Write(p) }

func dialTURNConn(turnAddr string, tcp bool) (net.PacketConn, error) {
	if !tcp {
		resolved, err := net.ResolveUDPAddr("udp", turnAddr)
		if err != nil {
			return nil, fmt.Errorf("резолв TURN: %w", err)
		}
		c, err := net.DialUDP("udp", nil, resolved)
		if err != nil {
			return nil, fmt.Errorf("подключение TURN UDP: %w", err)
		}
		_ = c.SetReadBuffer(socketBufSize)
		_ = c.SetWriteBuffer(socketBufSize)
		return &connectedUDPConn{c}, nil
	}

	d := net.Dialer{Timeout: 10 * time.Second}
	c, err := d.Dial("tcp", turnAddr)
	if err != nil {
		return nil, fmt.Errorf("подключение TURN TCP: %w", err)
	}
	return turn.NewSTUNConn(c), nil
}

// SessionErrorType — тип ошибки сессии
type SessionErrorType string

const (
	// ADDRESS_DEAD — этот конкретный TURN-адрес не работает (квота, unreachable, timeout)
	SessionErrorAddressDead SessionErrorType = "ADDRESS_DEAD"
	// WRAP_TIMEOUT — DTLS-рукопожатие не прошло, нужно сменить обфускацию
	SessionErrorWrapTimeout SessionErrorType = "WRAP_TIMEOUT"
	// AUTH — TURN отклонил креды (401/протухшие); нужно обновить креды и повторить
	SessionErrorAuth SessionErrorType = "AUTH"
	// FATAL — невосстановимая ошибка (FATAL_AUTH, хеш мёртв и т.д.)
	SessionErrorFatal SessionErrorType = "FATAL"
)

// SessionError — структурированная ошибка сессии
type SessionError struct {
	Type    SessionErrorType
	Address string // TURN-адрес, на котором произошла ошибка (если применимо)
	Err     error
}

func (e *SessionError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v (addr=%s)", e.Type, e.Err, e.Address)
	}
	return fmt.Sprintf("%s (addr=%s)", e.Type, e.Address)
}

func (e *SessionError) Unwrap() error { return e.Err }

// RunSession устанавливает TURN-сессию с указанным адресом.
// В отличие от предыдущей версии, адрес передаётся явно, а не выбирается по индексу.
func RunSession(
	ctx context.Context,
	tp *TurnParams,
	peer *net.UDPAddr,
	d *Dispatcher,
	localPort string,
	getConfig bool,
	configCh chan<- string,
	sessionID int,
	turnAddr string, // конкретный TURN-адрес
	turnUser string, // username для TURN
	turnPass string, // password для TURN
	obfsMode string, // режим обфускации (пер-воркерский, может меняться на WRAP_TIMEOUT)
	cacheStreamID int, // для handleAuthError
	deviceID, password string,
	stats *Stats,
) (bool, *SessionError) {
	configDelivered := false

	if turnAddr == "" {
		return false, &SessionError{
			Type: SessionErrorFatal,
			Err:  fmt.Errorf("пустой TURN-адрес"),
		}
	}

	urlhost, urlport, err := net.SplitHostPort(turnAddr)
	if err != nil {
		return false, &SessionError{
			Type:    SessionErrorAddressDead,
			Address: turnAddr,
			Err:     fmt.Errorf("разбор TURN-адреса: %w", err),
		}
	}
	if tp.Host != "" {
		urlhost = tp.Host
	}
	if tp.Port != "" {
		urlport = tp.Port
	}
	turnAddrResolved := net.JoinHostPort(urlhost, urlport)

	turnConn, err := dialTURNConn(turnAddrResolved, tp.TCPTransport)
	if err != nil {
		return false, &SessionError{
			Type:    SessionErrorAddressDead,
			Address: turnAddr,
			Err:     err,
		}
	}
	defer turnConn.Close()

	if tp.TCPTransport {
		log.Printf("[СЕССИЯ #%d] TURN TCP (%s)", sessionID, turnAddr)
	} else {
		log.Printf("[СЕССИЯ #%d] TURN UDP (%s)", sessionID, turnAddr)
	}

	var addrFamily turn.RequestedAddressFamily
	if peer.IP.To4() != nil {
		addrFamily = turn.RequestedAddressFamilyIPv4
	} else {
		addrFamily = turn.RequestedAddressFamilyIPv6
	}

	tc, err := turn.NewClient(&turn.ClientConfig{
		STUNServerAddr:         turnAddrResolved,
		TURNServerAddr:         turnAddrResolved,
		Conn:                   turnConn,
		Username:               turnUser,
		Password:               turnPass,
		RequestedAddressFamily: addrFamily,
		LoggerFactory:          &NullLoggerFactory{},
	})
	if err != nil {
		return false, &SessionError{
			Type:    SessionErrorAddressDead,
			Address: turnAddr,
			Err:     fmt.Errorf("TURN клиент: %w", err),
		}
	}
	defer tc.Close()

	if err = tc.Listen(); err != nil {
		return false, &SessionError{
			Type:    SessionErrorAddressDead,
			Address: turnAddr,
			Err:     fmt.Errorf("TURN Listen: %w", err),
		}
	}

	relay, err := tc.Allocate()
	if err != nil {
		errStr := err.Error()
		errLower := strings.ToLower(errStr)

		// Проверяем на ошибки авторизации (кеш кредов)
		if isAuthError(err) {
			handleAuthError(cacheStreamID)
			// Креды протухли — это НЕ фатальная ошибка: воркер обновит
			// креды через GetCreds (кеш уже инвалидирован выше) и повторит
			return false, &SessionError{
				Type:    SessionErrorAuth,
				Address: turnAddr,
				Err:     fmt.Errorf("TURN Allocate: %w", err),
			}
		}

		// Квота, unreachable, timeout — всё это "адрес мёртв"
		if strings.Contains(errLower, "quota") ||
			strings.Contains(errLower, "486") ||
			strings.Contains(errLower, "unreachable") ||
			strings.Contains(errLower, "timeout") ||
			strings.Contains(errLower, "connection refused") ||
			strings.Contains(errLower, "no route to host") {
			return false, &SessionError{
				Type:    SessionErrorAddressDead,
				Address: turnAddr,
				Err:     fmt.Errorf("TURN Allocate: %w", err),
			}
		}

		// Остальные ошибки — фатальные
		return false, &SessionError{
			Type:    SessionErrorFatal,
			Address: turnAddr,
			Err:     fmt.Errorf("TURN Allocate: %w", err),
		}
	}
	defer relay.Close()

	getStreamCache(cacheStreamID).errorCount.Store(0)

	log.Printf("[СЕССИЯ #%d] Relay: %s", sessionID, relay.LocalAddr())

	pipeA, pipeB := connutil.AsyncPacketPipe()

	sessCtx, sessCancel := context.WithCancel(ctx)
	// ВАЖНО: defer sessCancel() регистрируется НИЖЕ, рядом с defer stopRelay().
	// Defers работают LIFO, и sessCancel обязан выполниться первым — иначе
	// stopRelay снимет AfterFunc до его срабатывания, дедлайны не выставятся,
	// и горутины релея навсегда зависнут в ReadFrom (утечка на каждом
	// error-пути сессии).

	var sessionWg sync.WaitGroup
	sessionWg.Add(1)
	go func() {
		defer sessionWg.Done()
		t := time.NewTicker(10 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-sessCtx.Done():
				return
			case <-t.C:
				tc.SendBindingRequest()
			}
		}
	}()

	var relayWg sync.WaitGroup
	relayWg.Add(2)

	useWrap := len(tp.WrapKey) == wrapKeyLen

	var obfsCfg *ObfsConfig
	var obfsWriteState *ObfsState
	if useWrap {
		obfsCfg = NewObfsConfig(obfsMode)
		obfsWriteState = NewObfsState()
	}

	stopRelay := context.AfterFunc(sessCtx, func() {
		_ = relay.SetDeadline(time.Now())
		_ = pipeA.SetDeadline(time.Now())
	})
	defer stopRelay()
	defer sessCancel()

	go func() {
		defer relayWg.Done()
		defer sessCancel()

		readBufLen := readBufSize + 80
		buf := make([]byte, readBufLen)
		plain := make([]byte, readBufSize)
		for {
			n, _, readErr := relay.ReadFrom(buf)
			if readErr != nil {
				return
			}
			payload := buf[:n]
			if useWrap {
				if !obfsIsRTPPacket(payload) {
					log.Printf("[СЕССИЯ #%d] OBFS unwrap: unexpected packet (n=%d)", sessionID, n)
					continue
				}
				m, wrapErr := obfsUnwrapPacket(tp.WrapKey, payload, plain)
				if wrapErr != nil {
					log.Printf("[СЕССИЯ #%d] OBFS unwrap: %v (n=%d)", sessionID, wrapErr, n)
					continue
				}
				payload = plain[:m]
			}
			if _, writeErr := pipeA.WriteTo(payload, peer); writeErr != nil {
				return
			}
		}
	}()

	go func() {
		defer relayWg.Done()
		defer sessCancel()
		b := make([]byte, readBufSize)
		for {
			n, _, readErr := pipeA.ReadFrom(b)
			if readErr != nil {
				return
			}
			out := b[:n]
			if useWrap {
				if obfsCfg != nil && obfsWriteState != nil {
					wrapped, wrapErr := obfsWrapPacket(tp.WrapKey, out, obfsCfg, obfsWriteState)
					if wrapErr != nil {
						log.Printf("[СЕССИЯ #%d] OBFS wrap: %v", sessionID, wrapErr)
						return
					}
					out = wrapped
				}
			}
			if _, writeErr := relay.WriteTo(out, peer); writeErr != nil {
				return
			}
		}
	}()

	cert, err := selfsign.GenerateSelfSigned()
	if err != nil {
		return false, &SessionError{
			Type:    SessionErrorFatal,
			Address: turnAddr,
			Err:     fmt.Errorf("генерация сертификата: %w", err),
		}
	}

	select {
	case handshakeSem <- struct{}{}:
	case <-sessCtx.Done():
		return false, &SessionError{
			Type:    SessionErrorFatal,
			Address: turnAddr,
			Err:     sessCtx.Err(),
		}
	}

	dtlsCfg := &dtls.Config{
		Certificates:          []tls.Certificate{cert},
		InsecureSkipVerify:    true,
		ExtendedMasterSecret:  dtls.RequireExtendedMasterSecret,
		CipherSuites:          []dtls.CipherSuiteID{dtls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256},
		ConnectionIDGenerator: dtls.OnlySendCIDGenerator(),
	}

	dtlsConn, err := dtls.Client(pipeB, peer, dtlsCfg)
	if err != nil {
		<-handshakeSem
		return false, &SessionError{
			Type:    SessionErrorAddressDead,
			Address: turnAddr,
			Err:     fmt.Errorf("DTLS клиент: %w", err),
		}
	}
	defer dtlsConn.Close()

	hctx, hcancel := context.WithTimeout(sessCtx, 20*time.Second)
	log.Printf("[ВОРКЕР #%d] [DTLS] Рукопожатие (Handshake)...", sessionID)
	err = dtlsConn.HandshakeContext(hctx)
	hcancel()
	<-handshakeSem

	if err != nil {
		errStr := strings.ToLower(err.Error())
		if useWrap && (strings.Contains(errStr, "deadline") || strings.Contains(errStr, "timeout")) {
			return false, &SessionError{
				Type:    SessionErrorWrapTimeout,
				Address: turnAddr,
				Err:     fmt.Errorf("DTLS timeout, пароль/WRAP не подтверждён"),
			}
		}
		return false, &SessionError{
			Type:    SessionErrorAddressDead,
			Address: turnAddr,
			Err:     fmt.Errorf("DTLS хендшейк: %w", err),
		}
	}
	log.Printf("[ВОРКЕР #%d] [DTLS] Соединение установлено ✓", sessionID)

	stats.ActiveConnections.Add(1)
	defer stats.ActiveConnections.Add(-1)

	if getConfig && configCh != nil {
		conf, confErr := RequestConfig(dtlsConn, localPort, deviceID, password)
		if confErr != nil {
			errStr := confErr.Error()
			if strings.Contains(errStr, "FATAL_AUTH") {
				return false, &SessionError{
					Type:    SessionErrorFatal,
					Address: turnAddr,
					Err:     confErr,
				}
			}
			log.Printf("[ВОРКЕР #%d] Ошибка конфига: %v", sessionID, confErr)
		} else if conf != "" {
			select {
			case configCh <- conf:
				configDelivered = true
				log.Printf("[ВОРКЕР #%d] Конфиг получен", sessionID)
			default:
				configDelivered = true
				log.Printf("[ВОРКЕР #%d] Конфиг уже был доставлен другим воркером", sessionID)
			}
		} else {
			log.Printf("[ВОРКЕР #%d] Сервер ещё не выдал WireGuard-конфиг, повторим позже", sessionID)
		}
	} else {
		if authErr := SendAuth(dtlsConn, deviceID, password); authErr != nil {
			log.Printf("[ВОРКЕР #%d] Ошибка авторизации: %v", sessionID, authErr)
		}
	}

	emitReady()

	slot := &WorkerSlot{
		ID:     sessionID,
		SendCh: make(chan []byte, workerSendBuf),
	}
	d.Register(slot)
	defer d.Unregister(slot)

	var proxyWg sync.WaitGroup
	proxyWg.Add(3)

	stopDTLS := context.AfterFunc(sessCtx, func() {
		_ = dtlsConn.SetDeadline(time.Now())
	})
	defer stopDTLS()

	go func() {
		defer proxyWg.Done()
		t := time.NewTicker(keepaliveInterval)
		defer t.Stop()
		ping := []byte{keepaliveByte}
		for {
			select {
			case <-sessCtx.Done():
				return
			case <-t.C:
				_ = dtlsConn.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if _, err := dtlsConn.Write(ping); err != nil {
					return
				}
			}
		}
	}()

	go func() {
		defer proxyWg.Done()
		defer sessCancel()
		for {
			select {
			case <-sessCtx.Done():
				return
			case pkt, ok := <-slot.SendCh:
				if !ok {
					return
				}
				_ = dtlsConn.SetWriteDeadline(time.Now().Add(sessionReadTimeout))
				_, writeErr := dtlsConn.Write(pkt)
				putPktBuf(pkt)
				if writeErr != nil {
					log.Printf("[ВОРКЕР #%d] Ошибка Writer: %v", sessionID, writeErr)
					return
				}
			}
		}
	}()

	go func() {
		defer proxyWg.Done()
		defer sessCancel()
		for {
			pkt := getPktBuf(2048)
			_ = dtlsConn.SetReadDeadline(time.Now().Add(sessionReadTimeout))
			n, readErr := dtlsConn.Read(pkt)
			if readErr != nil {
				putPktBuf(pkt)
				if sessCtx.Err() != nil {
					return
				}
				if ne, ok := readErr.(net.Error); ok && ne.Timeout() {
					continue
				}
				log.Printf("[ВОРКЕР #%d] Ошибка Reader: %v", sessionID, readErr)
				return
			}

			if n == 1 && pkt[0] == keepaliveByte {
				putPktBuf(pkt)
				continue
			}

			pkt = pkt[:n]
			select {
			case d.ReturnCh <- pkt:
			case <-sessCtx.Done():
				putPktBuf(pkt)
				return
			}
		}
	}()

	proxyWg.Wait()
	sessCancel()
	relayWg.Wait()
	sessionWg.Wait()
	_ = pipeA.Close()
	_ = pipeB.Close()
	log.Printf("[СЕССИЯ #%d] Завершена", sessionID)
	return configDelivered, nil
}
