package termemu

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/strantalis/workset/pkg/unifiedlog"
)

var (
	traceOnce sync.Once
	traceLog  *unifiedlog.Logger
	traceMu   sync.Mutex
)

func traceEnabled() bool {
	if traceLog != nil {
		return true
	}
	traceOnce.Do(func() {
		if !envTruthy(os.Getenv("WORKSET_TERMEMU_TRACE")) {
			return
		}
		logger, err := unifiedlog.Open("termemu", "")
		if err != nil {
			return
		}
		traceLog = logger
	})
	return traceLog != nil
}

func tracef(ctx context.Context, format string, args ...any) {
	if !traceEnabled() {
		return
	}
	if ctx == nil {
		return
	}
	message := fmt.Sprintf(format, args...)
	traceMu.Lock()
	traceLog.Write(ctx, unifiedlog.Entry{
		Category:  "terminal.trace",
		Direction: "none",
		Action:    "event",
		Detail:    message,
	})
	traceMu.Unlock()
}

func EnableTrace(logger *unifiedlog.Logger) {
	traceMu.Lock()
	traceLog = logger
	traceMu.Unlock()
}

func envTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
