<p align="center">
  <img src="assets/icons/icon.png" width="96" />
</p>

<h1 align="center">PWDTT</h1>

<p align="center">
  Десктопный VPN-клиент, который туннелирует трафик через TURN-серверы VK,<br>
  маскируя соединение под зашифрованный медиатрафик звонка.<br>
  <sub>Форк <a href="https://github.com/amurcanov/proxy-turn-vk-android">proxy-turn-vk-android</a> — версия для ПК</sub>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.26-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go">
  <img src="https://img.shields.io/badge/Wails-v2-red?style=for-the-badge&logo=wails&logoColor=white" alt="Wails">
  <img src="https://img.shields.io/badge/Linux-amd64-FCC624?style=for-the-badge&logo=linux&logoColor=black" alt="Linux">
  <img src="https://img.shields.io/badge/Windows-amd64-0078D4?style=for-the-badge&logo=windows&logoColor=white" alt="Windows">
</p>

---

## Как это работает

Приложение поднимает локальный WireGuard-интерфейс и передаёт его трафик через TURN/DTLS серверы VK, оборачивая пакеты в RTP с шифрованием ChaCha20-Poly1305. С точки зрения провайдера — это обычный зашифрованный VK-звонок.

```
Приложение → WireGuard → ChaCha20/RTP → VK TURN/DTLS → wdtt-server (VPS) → интернет
```

Для работы нужен свой VPS — деплой встроен прямо в приложение.

---

## Запуск

### Linux

Установите зависимости:
```bash
# Ubuntu/Debian
sudo apt install wireguard-tools libayatana-appindicator3-1

# Arch
sudo pacman -S wireguard-tools libayatana-appindicator
```

Разрешите команды `ip` и `wg` без пароля — добавьте в `/etc/sudoers` через `visudo`:
```
your_user ALL=(ALL) NOPASSWD: /usr/bin/ip, /usr/bin/wg
```

Скачайте бинарник из [Releases](https://github.com/luminescq/PWDTT/releases) и запустите:
```bash
chmod +x pwdtt-linux-amd64
./pwdtt-linux-amd64
```

### Windows

Скачайте `pwdtt-windows-amd64.exe` из [Releases](https://github.com/luminescq/PWDTT/releases) и запустите. Драйвер WireGuard (wintun) встроен.

---

## Быстрый старт

1. **Деплой сервера** — вкладка Deploy → введите данные VPS → «Установить»
2. **Добавьте сервер** — кнопка `+` → вставьте `wdtt://`-ссылку или введите вручную
3. **VK-хеши** — Настройки → VK Хеши → вставьте 1–4 хеша из `vk.com/call/join/<hash>`
4. **Подключение** — кнопка питания

---

## Формат ссылки

Приложение принимает ссылки вида:

```
wdtt://<IP>:<DTLS_PORT>:<WG_PORT>:<PROXY_PORT>:<PASSWORD>[:<HASH1>,<HASH2>,...][#название]
```

- Поля 1–5 обязательны
- Хеши — опциональны, через запятую, до 4 штук
- `#название` — опциональный псевдоним сервера

Пример:
```
wdtt://1.2.3.4:56000:56001:0:mypassword:AbCdEfGh,XyZ12345#Мой сервер
```

Вставить ссылку можно через кнопку `+` или просто **Ctrl+V** в любом месте окна.

> Если ссылка содержит хеши, приложение предложит заменить уже сохранённые.

---

## Сборка из исходников

**Зависимости:** Go 1.22+, Node.js 18+, [Wails v2](https://wails.io)

```bash
# Linux
sudo apt install libayatana-appindicator3-dev pkg-config gcc
go install github.com/wailsapp/wails/v2/cmd/wails@latest

git clone https://github.com/luminescq/PWDTT
cd PWDTT
wails build -platform linux/amd64 -o pwdtt-linux-amd64
# → build/bin/pwdtt-linux-amd64
```

```bash
# Windows (кросс-компиляция с Linux)
wails build -platform windows/amd64
# → build/bin/pwdtt.exe
```

---

> [!IMPORTANT]
> Приложение является техническим инструментом для защищённого туннелирования собственного трафика через ваш сервер. Автор не призывает использовать PWDTT для противоправных целей или нарушения правил сторонних сервисов.

---

## Лицензия

Этот проект распространяется под лицензией GNU General Public License v3.0.
