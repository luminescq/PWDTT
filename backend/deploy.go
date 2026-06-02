package backend

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/crypto/ssh"
)

type DeployParams struct {
	Host         string `json:"host"`
	Login        string `json:"login"`
	Password     string `json:"password"`
	SSHPort      string `json:"sshPort"`
	MainPassword string `json:"mainPassword"`
	AdminID      string `json:"adminId"`
	BotToken     string `json:"botToken"`
	DtlsPort     int    `json:"dtlsPort"`
	WgPort       int    `json:"wgPort"`
}

func sshConnect(host, login, password, port string) (*ssh.Client, error) {
	if port == "" {
		port = "22"
	}
	cfg := &ssh.ClientConfig{
		User:            login,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         20 * time.Second,
	}
	return ssh.Dial("tcp", net.JoinHostPort(host, port), cfg)
}

func sshUpload(client *ssh.Client, data []byte, remotePath string) error {
	sess, err := client.NewSession()
	if err != nil {
		return err
	}
	defer sess.Close()
	sess.Stdin = bytes.NewReader(data)
	return sess.Run("cat > " + remotePath)
}

func sshExecStream(client *ssh.Client, cmd string, onLine func(string)) (string, error) {
	sess, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()

	pr, pw := io.Pipe()
	sess.Stdout = pw
	sess.Stderr = pw

	var buf strings.Builder
	done := make(chan struct{})
	go func() {
		defer close(done)
		tmp := make([]byte, 4096)
		var line strings.Builder
		for {
			n, readErr := pr.Read(tmp)
			for _, b := range tmp[:n] {
				if b == '\n' {
					s := line.String()
					buf.WriteString(s + "\n")
					onLine(s)
					line.Reset()
				} else {
					line.WriteByte(b)
				}
			}
			if readErr != nil {
				if line.Len() > 0 {
					s := line.String()
					buf.WriteString(s)
					onLine(s)
				}
				break
			}
		}
	}()

	runErr := sess.Run(cmd)
	pw.Close()
	<-done
	return buf.String(), runErr
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

func (a *App) Deploy(p DeployParams) error {
	login := p.Login
	if login == "" {
		login = "root"
	}
	if p.DtlsPort == 0 {
		p.DtlsPort = 56000
	}
	if p.WgPort == 0 {
		p.WgPort = 56001
	}
	if p.SSHPort == "" {
		p.SSHPort = "22"
	}

	emit := func(msg string) { runtime.EventsEmit(a.ctx, "deploy_log", msg) }

	emit("Подключение к " + p.Host + "...")
	client, err := sshConnect(p.Host, login, p.Password, p.SSHPort)
	if err != nil {
		return fmt.Errorf("SSH: %w", err)
	}
	defer client.Close()
	emit("Подключено ✓")

	emit("Загрузка deploy.sh...")
	if err := sshUpload(client, deployScript, "/tmp/deploy.sh"); err != nil {
		return fmt.Errorf("upload deploy.sh: %w", err)
	}
	emit("Загрузка wdtt-server...")
	if err := sshUpload(client, serverBinary, "/tmp/wdtt-server"); err != nil {
		return fmt.Errorf("upload wdtt-server: %w", err)
	}
	emit("Файлы загружены ✓")

	var argParts []string
	if p.MainPassword != "" {
		argParts = append(argParts, "-password "+shellQuote(p.MainPassword))
	}
	if p.AdminID != "" {
		argParts = append(argParts, "-admin "+shellQuote(p.AdminID))
	}
	if p.BotToken != "" {
		argParts = append(argParts, "-bot-token "+shellQuote(p.BotToken))
	}
	wdttArgs := strings.Join(argParts, " ")

	cmd := fmt.Sprintf(
		"chmod +x /tmp/wdtt-server /tmp/deploy.sh && WDTT_ARGS=%s WDTT_DTLS_PORT=%d WDTT_WG_PORT=%d WDTT_SSH_PORT=%s bash /tmp/deploy.sh 2>&1",
		shellQuote(wdttArgs), p.DtlsPort, p.WgPort, p.SSHPort,
	)

	emit("Запуск установки...")
	out, err := sshExecStream(client, cmd, emit)
	if err != nil && !strings.Contains(out, "✅") && !strings.Contains(out, "active") {
		return fmt.Errorf("deploy: %w", err)
	}

	runtime.EventsEmit(a.ctx, "deploy_done", "success")
	return nil
}

func (a *App) Undeploy(p DeployParams) error {
	login := p.Login
	if login == "" {
		login = "root"
	}
	if p.DtlsPort == 0 {
		p.DtlsPort = 56000
	}
	if p.WgPort == 0 {
		p.WgPort = 56001
	}
	if p.SSHPort == "" {
		p.SSHPort = "22"
	}

	emit := func(msg string) { runtime.EventsEmit(a.ctx, "deploy_log", msg) }

	emit("Подключение...")
	client, err := sshConnect(p.Host, login, p.Password, p.SSHPort)
	if err != nil {
		return fmt.Errorf("SSH: %w", err)
	}
	defer client.Close()

	if err := sshUpload(client, deployScript, "/tmp/deploy.sh"); err != nil {
		return fmt.Errorf("upload deploy.sh: %w", err)
	}

	cmd := fmt.Sprintf(
		"WDTT_DTLS_PORT=%d WDTT_WG_PORT=%d WDTT_SSH_PORT=%s bash /tmp/deploy.sh uninstall 2>&1",
		p.DtlsPort, p.WgPort, p.SSHPort,
	)
	out, err := sshExecStream(client, cmd, emit)
	if err != nil && !strings.Contains(out, "removed") && !strings.Contains(out, "удал") {
		return fmt.Errorf("undeploy: %w", err)
	}

	runtime.EventsEmit(a.ctx, "deploy_done", "removed")
	return nil
}
