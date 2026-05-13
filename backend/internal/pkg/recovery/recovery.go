package recovery

import (
	"log/slog"
	"runtime/debug"
)

func GoSafe(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("goroutine panic recovered",
					"error", r,
					"stack", string(debug.Stack()),
				)
			}
		}()
		fn()
	}()
}