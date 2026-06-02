package core

import (
	"fmt"
	"sync/atomic"
	"time"
)

type Stats struct {
	ActiveConnections int32
	TotalBytesUp      int64
	TotalBytesDown    int64
}

func NewStats() *Stats {
	return &Stats{}
}

func (s *Stats) RunLoop(shutdown <-chan struct{}, emitter func(level, msg string)) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-shutdown:
			return
		case <-ticker.C:
			active := atomic.LoadInt32(&s.ActiveConnections)
			up := atomic.LoadInt64(&s.TotalBytesUp)
			down := atomic.LoadInt64(&s.TotalBytesDown)
			totalMB := float64(up+down) / (1024.0 * 1024.0)
			emitter("INFO", fmt.Sprintf("[СТАТИСТИКА] Активных: %d | Трафик: %.2f МБ", active, totalMB))
		}
	}
}
