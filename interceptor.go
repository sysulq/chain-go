package chain

import (
	"context"
	"fmt"
	"log/slog"
)

// RecoverInterceptor is an interceptor that recovers from panics in the ChainFunc
func RecoverInterceptor[I, O any](fn ChainFunc[I, O]) ChainFunc[I, O] {
	return func(ctx context.Context, args *Args[I, O]) (err error) {
		isPanic := true
		defer func() {
			if isPanic {
				if r := recover(); r != nil {
					err = fmt.Errorf("panic in execution: %v", r)
				}
			}
		}()
		err = fn(ctx, args)
		isPanic = false
		return
	}
}

// DebugInterceptor is an interceptor that prints the input and output of the ChainFunc
func DebugInterceptor[I, O any](fn ChainFunc[I, O]) ChainFunc[I, O] {
	return func(ctx context.Context, args *Args[I, O]) error {
		slog.InfoContext(ctx, "Before execution", slog.Any("input", args.input), slog.Any("output", args.output))
		err := fn(ctx, args)
		slog.InfoContext(ctx, "After execution", slog.Any("input", args.input), slog.Any("output", args.output), "error", err)
		return err
	}
}
