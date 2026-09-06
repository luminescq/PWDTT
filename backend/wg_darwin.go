package backend

// macOS: создание utun требует root, поэтому туннель поднимается в два процесса:
//
//  1. Приложение (без прав) слушает unix-сокет и через osascript
//     ("with administrator privileges" — системный диалог пароля) запускает
//     этот же бинарник в режиме `--wg-helper` от root.
//  2. Helper (root) создаёт utun, назначает адрес/MTU, прописывает маршруты
//     и передаёт fd интерфейса приложению через SCM_RIGHTS. Дальше helper
//     висит на сокете: команда "down" или обрыв соединения (краш приложения)
//     → снимает exclude-маршруты и выходит.
//  3. Приложение оборачивает полученный fd в tun.Device и крутит userspace
//     wireguard-go без прав. Ключи helper'у не передаются.
//
// Туннельные маршруты (-interface utunN) умирают вместе с utun, когда
// приложение закрывает fd — явно их снимать не нужно.

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
)

// wgHelperMsg — JSON-строка протокола helper→app (одна на строку).
// Сообщение с непустым Name несёт fd интерфейса в OOB (SCM_RIGHTS).
type wgHelperMsg struct {
	Log   string `json:"log,omitempty"`
	Error string `json:"error,omitempty"`
	Name  string `json:"name,omitempty"`
}

// ═══════════════════════════════════════════════════
// APP SIDE
// ═══════════════════════════════════════════════════

func (w *WG) applyDarwin(confText string, turnIPs []string, logf wgLogFunc) error {
	w.teardownDarwin()

	addr, mtuStr, allowedIPs, wgConf := parseWGConfig(confText)
	if addr == "" {
		return fmt.Errorf("Address not found in wg config")
	}
	mtu := 1300
	if mtuStr != "" {
		fmt.Sscanf(mtuStr, "%d", &mtu)
	}

	// Exclude-маршруты — через физический gateway, мимо туннеля
	var excludes []string
	for _, ip := range turnIPs {
		excludes = append(excludes, ip+"/32")
	}
	excludes = append(excludes, vkExcludeCIDRs...)
	for _, dns := range localDNSServers() {
		excludes = append(excludes, dns+"/32")
	}

	// Туннельные маршруты: полный дефолт заменяем на split-default,
	// чтобы не трогать физический default route
	var tunnels []string
	for _, cidr := range allowedIPs {
		if cidr == "0.0.0.0/0" {
			tunnels = append(tunnels, "0.0.0.0/1", "128.0.0.0/1")
		} else {
			tunnels = append(tunnels, cidr)
		}
	}

	// Сокет, на который выйдет helper
	sockDir, err := os.MkdirTemp("", "pwdtt-wg-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(sockDir)
	sock := filepath.Join(sockDir, "h.sock")
	ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: sock, Net: "unix"})
	if err != nil {
		return err
	}

	exe, err := os.Executable()
	if err != nil {
		ln.Close()
		return err
	}

	shellCmd := fmt.Sprintf("%s --wg-helper -sock %s -addr %s -mtu %d -exclude %s -tunnel %s >/dev/null 2>&1 &",
		shellQuote(exe), shellQuote(sock), shellQuote(addr), mtu,
		shellQuote(strings.Join(excludes, ",")), shellQuote(strings.Join(tunnels, ",")))
	osa := fmt.Sprintf("do shell script %q with administrator privileges", shellCmd)

	logf("Запрос прав администратора для настройки туннеля…")
	if out, err := exec.Command("osascript", "-e", osa).CombinedOutput(); err != nil {
		ln.Close()
		return fmt.Errorf("права администратора не получены: %w — %s", err, strings.TrimSpace(string(out)))
	}

	ln.SetDeadline(time.Now().Add(30 * time.Second))
	hconn, err := ln.AcceptUnix()
	ln.Close()
	if err != nil {
		return fmt.Errorf("helper не вышел на связь: %w", err)
	}

	tunFile, utunName, err := recvTunFD(hconn, logf)
	if err != nil {
		hconn.Close()
		return err
	}
	hconn.SetReadDeadline(time.Time{})
	logf(fmt.Sprintf("Интерфейс %s получен от helper", utunName))

	tunDev, err := tun.CreateTUNFromFile(tunFile, 0) // mtu уже выставлен helper'ом
	if err != nil {
		hconn.Close()
		return fmt.Errorf("wrap TUN: %w", err)
	}

	logger := &device.Logger{
		Verbosef: func(format string, args ...interface{}) {},
		Errorf:   func(format string, args ...interface{}) { logf(fmt.Sprintf(format, args...)) },
	}
	dev := device.NewDevice(tunDev, conn.NewDefaultBind(), logger)

	logf("Применение WireGuard конфига (uapi)")
	if err := dev.IpcSetOperation(strings.NewReader(uapiConf(wgConf))); err != nil {
		dev.Close()
		hconn.Close()
		return fmt.Errorf("IpcSet: %w", err)
	}
	if err := dev.Up(); err != nil {
		dev.Close()
		hconn.Close()
		return fmt.Errorf("device up: %w", err)
	}

	// Захватываем stateMu только на секцию присвоения состояния: osascript
	// выше может висеть минутами (диалог пароля) — нельзя блокировать Teardown
	w.stateMu.Lock()
	w.activeTun = tunDev
	w.activeDevice = dev
	w.helperConn = hconn
	w.activeExcludeRoutes = excludes
	w.activeRoutesMu.Lock()
	w.activeRoutes = tunnels
	w.activeRoutesMu.Unlock()
	w.stateMu.Unlock()

	logf(fmt.Sprintf("Туннель %s поднят, маршруты: %v", utunName, tunnels))
	return nil
}

func (w *WG) teardownDarwin() {
	w.stateMu.Lock()
	defer w.stateMu.Unlock()

	// Останавливаем движок и закрываем fd — utun и его маршруты исчезают сами
	if w.activeDevice != nil {
		w.activeDevice.Close()
		w.activeDevice = nil
	}
	if w.activeTun != nil {
		_ = w.activeTun.Close()
		w.activeTun = nil
	}

	// Просим helper снять exclude-маршруты (без пароля) и ждём его выхода
	if w.helperConn != nil {
		_, _ = w.helperConn.Write([]byte("down\n"))
		_ = w.helperConn.SetReadDeadline(time.Now().Add(5 * time.Second))
		_, _ = io.Copy(io.Discard, w.helperConn)
		w.helperConn.Close()
		w.helperConn = nil
	}

	w.activeRoutesMu.Lock()
	w.activeRoutes = nil
	w.activeRoutesMu.Unlock()
	w.activeExcludeRoutes = nil
}

// recvTunFD читает сообщения helper'а до получения fd интерфейса.
func recvTunFD(hconn *net.UnixConn, logf wgLogFunc) (*os.File, string, error) {
	buf := make([]byte, 4096)
	oob := make([]byte, 128)
	var acc []byte
	pendingFD := -1
	hconn.SetReadDeadline(time.Now().Add(60 * time.Second))

	for {
		for {
			i := bytes.IndexByte(acc, '\n')
			if i < 0 {
				break
			}
			line := acc[:i]
			acc = acc[i+1:]
			var msg wgHelperMsg
			if json.Unmarshal(line, &msg) != nil {
				continue
			}
			if msg.Log != "" {
				logf(msg.Log)
			}
			if msg.Error != "" {
				return nil, "", fmt.Errorf("helper: %s", msg.Error)
			}
			if msg.Name != "" {
				if pendingFD < 0 {
					return nil, "", fmt.Errorf("fd интерфейса %s не получен", msg.Name)
				}
				return os.NewFile(uintptr(pendingFD), msg.Name), msg.Name, nil
			}
		}

		n, oobn, _, _, err := hconn.ReadMsgUnix(buf, oob)
		if err != nil {
			return nil, "", fmt.Errorf("чтение от helper: %w", err)
		}
		if oobn > 0 {
			if scms, err := syscall.ParseSocketControlMessage(oob[:oobn]); err == nil {
				for _, scm := range scms {
					if fds, err := syscall.ParseUnixRights(&scm); err == nil && len(fds) > 0 {
						pendingFD = fds[0]
						syscall.CloseOnExec(fds[0])
					}
				}
			}
		}
		acc = append(acc, buf[:n]...)
	}
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func defaultGatewayDarwin() string {
	out, err := exec.Command("route", "-n", "get", "default").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "gateway:" {
			return fields[1]
		}
	}
	return ""
}

// cleanupStaleExcludeRoutesDarwin — уборка exclude-маршрутов, переживших
// краш приложения вместе с helper'ом: обычные (не -interface) маршруты сами
// не исчезают и после смены сети ведут на мёртвый шлюз, глуша VK/Яндекс
// даже при выключенном туннеле. Права администратора запрашиваются только
// если протухшие маршруты действительно найдены.
func cleanupStaleExcludeRoutesDarwin(logf wgLogFunc) {
	curGW := defaultGatewayDarwin()
	if curGW == "" {
		return
	}

	// Протухший = специфичный маршрут из vkExcludeCIDRs, чей шлюз ≠ текущему
	staleGWs := map[string]bool{}
	for _, cidr := range vkExcludeCIDRs {
		out, err := exec.Command("route", "-n", "get", strings.SplitN(cidr, "/", 2)[0]).Output()
		if err != nil {
			continue
		}
		var dst, gw string
		for _, line := range strings.Split(string(out), "\n") {
			f := strings.Fields(line)
			if len(f) != 2 {
				continue
			}
			switch f[0] {
			case "destination:":
				dst = f[1]
			case "gateway:":
				gw = f[1]
			}
		}
		if dst != "" && dst != "default" && gw != "" && gw != curGW && net.ParseIP(gw) != nil {
			staleGWs[gw] = true
		}
	}
	if len(staleGWs) == 0 {
		return
	}

	// Сносим все маршруты через мёртвые шлюзы — включая /32 на TURN-серверы
	// и DNS, которые заранее не перечислить
	out, err := exec.Command("netstat", "-rn", "-f", "inet").Output()
	if err != nil {
		return
	}
	var dels []string
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 && staleGWs[f[1]] {
			dels = append(dels, "route -q -n delete -net "+shellQuote(f[0]))
		}
	}
	if len(dels) == 0 {
		return
	}

	logf(fmt.Sprintf("Найдены маршруты прошлого запуска через мёртвый шлюз (%d шт), удаляю…", len(dels)))
	osa := fmt.Sprintf("do shell script %q with administrator privileges", strings.Join(dels, "; "))
	if out, err := exec.Command("osascript", "-e", osa).CombinedOutput(); err != nil {
		logf(fmt.Sprintf("уборка маршрутов не удалась: %v — %s", err, strings.TrimSpace(string(out))))
		return
	}
	logf("Протухшие маршруты удалены")
}

func runCmdDarwin(name string, args ...string) error {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v: %w — %s", name, args, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ═══════════════════════════════════════════════════
// HELPER SIDE (запускается от root: pwdtt --wg-helper …)
// ═══════════════════════════════════════════════════

// RunWGHelperDarwin — точка входа режима --wg-helper (вызывается из main_darwin.go).
func RunWGHelperDarwin(args []string) {
	fs := flag.NewFlagSet("wg-helper", flag.ExitOnError)
	sockPath := fs.String("sock", "", "путь unix-сокета приложения")
	addr := fs.String("addr", "", "адрес интерфейса (CIDR)")
	mtu := fs.Int("mtu", 1300, "MTU")
	excludes := fs.String("exclude", "", "exclude CIDR через запятую (мимо туннеля)")
	tunnels := fs.String("tunnel", "", "туннельные CIDR через запятую")
	fs.Parse(args)

	raddr, err := net.ResolveUnixAddr("unix", *sockPath)
	if err != nil {
		os.Exit(1)
	}
	hconn, err := net.DialUnix("unix", nil, raddr)
	if err != nil {
		os.Exit(1)
	}
	defer hconn.Close()

	send := func(msg wgHelperMsg, fd int) {
		data, _ := json.Marshal(msg)
		data = append(data, '\n')
		if fd >= 0 {
			hconn.WriteMsgUnix(data, syscall.UnixRights(fd), nil)
		} else {
			hconn.Write(data)
		}
	}
	fail := func(format string, a ...any) {
		send(wgHelperMsg{Error: fmt.Sprintf(format, a...)}, -1)
		os.Exit(1)
	}

	if os.Geteuid() != 0 {
		fail("helper запущен не от root (uid=%d)", os.Getuid())
	}
	if *addr == "" {
		fail("не задан -addr")
	}

	dev, err := tun.CreateTUN("utun", *mtu)
	if err != nil {
		fail("create utun: %v", err)
	}
	name, err := dev.Name()
	if err != nil {
		fail("utun name: %v", err)
	}
	send(wgHelperMsg{Log: fmt.Sprintf("Создан интерфейс %s (mtu=%d)", name, *mtu)}, -1)

	host := strings.SplitN(*addr, "/", 2)[0]
	if err := runCmdDarwin("ifconfig", name, "inet", *addr, host, "alias"); err != nil {
		fail("ifconfig: %v", err)
	}
	_ = runCmdDarwin("ifconfig", name, "up")
	send(wgHelperMsg{Log: fmt.Sprintf("IP установлен: %s", *addr)}, -1)

	gw := defaultGatewayDarwin()
	send(wgHelperMsg{Log: fmt.Sprintf("Default gateway: %s", gw)}, -1)

	// Exclude-маршруты через физический gateway — ДО туннельных,
	// чтобы трафик к turn/VK не успел уйти в туннель
	var added []string
	if gw != "" {
		for _, cidr := range splitCSV(*excludes) {
			// Снимаем возможный остаток прошлого запуска (краш без уборки) —
			// иначе add упрётся в старый маршрут с мёртвым шлюзом
			_ = runCmdDarwin("route", "-q", "-n", "delete", "-net", cidr)
			if err := runCmdDarwin("route", "-q", "-n", "add", "-net", cidr, gw); err != nil {
				send(wgHelperMsg{Log: fmt.Sprintf("exclude route %s: %v", cidr, err)}, -1)
			} else {
				added = append(added, cidr)
			}
		}
		send(wgHelperMsg{Log: fmt.Sprintf("Exclude-маршрутов добавлено: %d", len(added))}, -1)
	}

	for _, cidr := range splitCSV(*tunnels) {
		if err := runCmdDarwin("route", "-q", "-n", "add", "-net", cidr, "-interface", name); err != nil {
			send(wgHelperMsg{Log: fmt.Sprintf("tunnel route %s: %v", cidr, err)}, -1)
		}
	}

	// Передаём fd интерфейса приложению
	send(wgHelperMsg{Name: name}, int(dev.File().Fd()))

	// Ждём "down" или обрыв соединения (краш приложения) → уборка
	buf := make([]byte, 64)
	for {
		n, err := hconn.Read(buf)
		if err != nil || strings.Contains(string(buf[:n]), "down") {
			break
		}
	}
	for _, cidr := range added {
		_ = runCmdDarwin("route", "-q", "-n", "delete", "-net", cidr)
	}
	runtime.KeepAlive(dev)
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
