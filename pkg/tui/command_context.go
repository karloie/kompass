package tui

import (
	"context"
	"sync"
	"time"
)

var commandExecTimeout = 30 * time.Second

var (
	programCommandContext   = context.Background()
	programCommandContextMu sync.RWMutex
)

func setProgramCommandContext(ctx context.Context) func() {
	programCommandContextMu.Lock()
	prev := programCommandContext
	if ctx == nil {
		ctx = context.Background()
	}
	programCommandContext = ctx
	programCommandContextMu.Unlock()

	return func() {
		programCommandContextMu.Lock()
		programCommandContext = prev
		programCommandContextMu.Unlock()
	}
}

func commandTimeoutContext() (context.Context, context.CancelFunc) {
	programCommandContextMu.RLock()
	base := programCommandContext
	programCommandContextMu.RUnlock()
	if base == nil {
		base = context.Background()
	}
	return context.WithTimeout(base, commandExecTimeout)
}
