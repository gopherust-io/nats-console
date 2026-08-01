package safe

import (
	"runtime/debug"

	"github.com/gopherust-io/tel"
)

// Log records a recovered panic with stack.
func Log(component string, rec any) {
	tel.Error().
		Str("component", component).
		Any("panic", rec).
		Bytes("stack", debug.Stack()).
		Msg("panic recovered")
}

// Recover is intended for defer safe.Recover(component).
func Recover(component string) {
	if rec := recover(); rec != nil {
		Log(component, rec)
	}
}

// Run executes fn and recovers panics so the caller can continue.
func Run(component string, fn func()) {
	defer Recover(component)
	fn()
}
