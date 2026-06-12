//go:build darwin

package backend

import "log"

// SetAutoStart — заглушка управления автозапуском для macOS
func (a *App) SetAutoStart(enable bool) error {
	log.Println("[Stub] Установка автозапуска в macOS пока не поддерживается:", enable)
	return nil
}

// GetAutoStart — заглушка получения статуса автозапуска для macOS
func (a *App) GetAutoStart() (bool, error) {
	return false, nil
}
