package backend

func (a *App) SetTrayEnabled(v bool) {
	a.trayEnabled.Store(v)
	setTrayVisible(v)
}
