package util

import (
	"fmt"
	"sync"
)

type Logger struct {
	mu sync.Mutex
	IsVerbose bool
	IsDebug bool
}

func (l *Logger) Verbose(format string, a ...any) {
	if !l.IsVerbose {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Printf(format + "\n", a...)
}

func (l *Logger) Debug(format string, a ...any) {
	if !l.IsDebug {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Printf("[DEBUG] " + format + "\n", a...)
}

func (l *Logger) Always(format string, a ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Printf(format + "\n", a...)
}
