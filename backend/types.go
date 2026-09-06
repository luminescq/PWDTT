package backend

// ConnectParams — параметры подключения из frontend.
type ConnectParams struct {
	PeerAddr    string   `json:"peerAddr"`             // адрес:порт сервера
	Password    string   `json:"password"`             // пароль подключения
	Hashes      []string `json:"hashes"`               // хеши VK
	DeviceID    string   `json:"deviceId,omitempty"`   // ID устройства
	Workers     int      `json:"workers,omitempty"`    // воркеры (кратно 9)
	CaptchaMode string   `json:"captchaMode,omitempty"` // auto/rjs/wv
	ObfsMode    string   `json:"obfsMode,omitempty"`   // audio/video
	Fingerprint string   `json:"fingerprint,omitempty"` // chrome/android/ios/safari/firefox
	TurnTCP     bool     `json:"turnTcp,omitempty"`    // использовать TCP транспорт
}

// ProfileData — данные сервера, хранятся в ~/.config/pwdtt/servers/<name>.json.
type ProfileData struct {
	PeerAddr string   `json:"peer"`      // адрес:порт VPS сервера
	Password string   `json:"password"`  // пароль подключения
	Hashes   []string `json:"hashes"`    // хеши VK-звонков
	Listen   string   `json:"listen"`    // локальный адрес (по умолчанию 127.0.0.1:9000)
	TurnHost string   `json:"turn"`      // переопределение IP TURN
	TurnPort string   `json:"port"`      // переопределение порта TURN
	DeviceID string   `json:"device_id"` // уникальный ID устройства
	TurnTCP  bool     `json:"turn_tcp"`  // использовать TCP транспорт
}

// AppSettings — настройки приложения, хранятся в ~/.config/pwdtt/config.json.
type AppSettings struct {
	AutoStart    bool   `json:"autoStart"`    // автозапуск при старте системы
	ObfsMode     string `json:"obfsMode"`     // audio/video
	ObfsAccepted bool   `json:"obfsAccepted"` // пользователь принял предупреждение об обфускации
	TurnTCP      bool   `json:"turnTcp"`      // использовать TCP транспорт
}
