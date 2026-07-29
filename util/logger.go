package util

import (
	"fmt"
	"io"
	"sync"
)

type Logger struct {
	mu sync.Mutex
	Output io.Writer
	IsVerbose bool
	IsDebug bool
}

func (l *Logger) Verbose(format string, a ...any) {
	if !l.IsVerbose {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintf(l.Output, format + "\n", a...)
}

func (l *Logger) Debug(format string, a ...any) {
	if !l.IsDebug {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintf(l.Output, "[DEBUG] " + format + "\n", a...)
}

func (l *Logger) Always(format string, a ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintf(l.Output, format + "\n", a...)
}
