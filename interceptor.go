package chain

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

var errPanic = fmt.Errorf("panic in execution")

// RecoverInterceptor is an interceptor that recovers from panics in the ChainFunc
func RecoverInterceptor[I, O any](fn HandleFunc[I, O]) HandleFunc[I, O] {
	return func(ctx context.Context, args *Args[I, O]) (err error) {
		isPanic := true
		defer func() {
			if isPanic {
				if r := recover(); r != nil {
					err = fmt.Errorf("%w: %v", errPanic, r)
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
	return func(ctx context.Context, args *Args[I, O]) error {
		now := time.Now()
		slog.DebugContext(ctx, "Before execution", slog.Any("input", args.input), slog.Any("output", args.output))
		err := fn(ctx, args)
		if err != nil {
			slog.WarnContext(ctx, "Error during execution", slog.Any("input", args.input), slog.Any("output", args.output),
				"error", err, "elapsed", time.Since(now), "fn", getFunctionName(fn))
		} else {
			slog.DebugContext(ctx, "After execution", slog.Any("input", args.input), slog.Any("output", args.output),
				"elapsed", time.Since(now))
		}

		return err
	}
}
