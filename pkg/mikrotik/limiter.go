package mikrotik

import (
	"context"
	"sync"
)

const DefaultMaxConcurrent = 25

var (
	limiterMu sync.Mutex
	connSem   chan struct{}
)

func init() {
	SetMaxConcurrent(DefaultMaxConcurrent)
}

// SetMaxConcurrent limits how many RouterOS API connections may be open at once.
func SetMaxConcurrent(n int) {
	if n <= 0 {
		n = DefaultMaxConcurrent
	}
	limiterMu.Lock()
	defer limiterMu.Unlock()
	connSem = make(chan struct{}, n)
}

func acquireConn(ctx context.Context) error {
	limiterMu.Lock()
	sem := connSem
	limiterMu.Unlock()
	select {
	case sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func releaseConn() {
	limiterMu.Lock()
	sem := connSem
	limiterMu.Unlock()
	select {
	case <-sem:
	default:
	}
}
