package chain

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"
)

var errPanic = fmt.Errorf("panic in execution")

// RecoverInterceptor is an interceptor that recovers from panics in the ChainFunc
func RecoverInterceptor[I, O any](fn HandleFunc[I, O]) HandleFunc[I, O] {
	return func(ctx context.Context, args *State[I, O]) (err error) {
		isPanic := true
		defer func() {
			if isPanic {
				if r := recover(); r != nil {
					err = fmt.Errorf("%w: %v", errPanic, r)
					debug.PrintStack()
				}
			}
		}()
		err = fn(ctx, args)
		isPanic = false
		return
	}
}

// LogInterceptor is an interceptor that prints the input and output of the ChainFunc
func LogInterceptor[I, O any](fn HandleFunc[I, O]) HandleFunc[I, O] {
	return func(ctx context.Context, args *State[I, O]) error {
		now := time.Now()
		slog.DebugContext(ctx, "After execution", slog.Any("input", args.Input()), slog.Any("output", args.Output()))
		err := fn(ctx, args)
		if err != nil {
			slog.DebugContext(ctx, "After execution", slog.Any("input", args.Input()), slog.Any("output", args.Output()),
				"error", err, "elapsed", time.Since(now))
		} else {
			slog.DebugContext(ctx, "After execution", slog.Any("input", args.Input()), slog.Any("output", args.Output()),
				"elapsed", time.Since(now))
		}

		return err
	}
}
