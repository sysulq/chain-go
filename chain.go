package chain

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

// HandleFunc represents a function that operates on a Chain's Args
type HandleFunc[I, O any] func(context.Context, *Args[I, O]) error

// Interceptor represents a function that wraps a handleFunc
type Interceptor[I, O any] func(HandleFunc[I, O]) HandleFunc[I, O]

// Chain represents a generic operation chain, supporting input type I and output type O
type Chain[I, O any] struct {
	ctx           context.Context
	args          *Args[I, O]
	fns           []HandleFunc[I, O]
	interceptors  []Interceptor[I, O]
	timeout       time.Duration
	maxGoroutines int
}

// Args holds the input and output data and a mutex for synchronization
type Args[I, O any] struct {
	input  *I
	output *O
	mu     sync.Mutex
}

// New creates a new Chain, specifying input and output types
func New[I, O any](input *I, output *O) *Chain[I, O] {
	return &Chain[I, O]{
		ctx: context.Background(),
		args: &Args[I, O]{
			input:  input,
			output: output,
			mu:     sync.Mutex{},
		},
	}
}

// Input returns a pointer to the input data of the Chain
func (c *Args[I, O]) Input() *I {
	return c.input
}

// Output returns a pointer to the output data of the Chain
func (c *Args[I, O]) Output() *O {
	return c.output
}

// WithLock executes the given function with the Chain's mutex locked
func (c *Args[I, O]) WithLock(fn func()) {
	c.mu.Lock()
	fn()
	c.mu.Unlock()
}

// WithContext sets a custom context for the Chain
func (c *Chain[I, O]) WithContext(ctx context.Context) *Chain[I, O] {
	c.ctx = ctx
	return c
}

// WithTimeout sets a timeout duration for the entire chain execution
func (c *Chain[I, O]) WithTimeout(d time.Duration) *Chain[I, O] {
	c.timeout = d
	return c
}

// WithMaxGoroutines sets the maximum number of goroutines for parallel execution
func (c *Chain[I, O]) WithMaxGoroutines(max int) *Chain[I, O] {
	c.maxGoroutines = max
	return c
}

// Use adds an interceptor to the chain
func (c *Chain[I, O]) Use(interceptor Interceptor[I, O]) *Chain[I, O] {
	c.interceptors = append(c.interceptors, interceptor)
	return c
}

// Serial adds operations to be executed sequentially
func (c *Chain[I, O]) Serial(fns ...HandleFunc[I, O]) *Chain[I, O] {
	c.fns = append(c.fns, func(ctx context.Context, args *Args[I, O]) error {
		for _, fn := range fns {
			handleFunc := c.buildInterceptors(fn)
			if err := handleFunc(ctx, c.args); err != nil {
				return err
			}
		}
		return nil
	})
	return c
}

// Parallel adds operations to be executed concurrently
func (c *Chain[I, O]) Parallel(fns ...HandleFunc[I, O]) *Chain[I, O] {
	c.fns = append(c.fns, func(ctx context.Context, args *Args[I, O]) error {
		g, ctx := errgroup.WithContext(ctx)

		if c.maxGoroutines > 0 {
			g.SetLimit(c.maxGoroutines)
		}
		for _, fn := range fns {
			fn := fn // https://golang.org/doc/faq#closures_and_goroutines
			g.Go(func() error {
				handleFunc := c.buildInterceptors(fn)
				return handleFunc(ctx, c.args)
			})
		}
		return g.Wait()
	})
	return c
}

// Execute runs all operations in the chain
func (c *Chain[I, O]) Execute() (*O, error) {
	ctx := c.ctx
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	for _, fn := range c.fns {
		// Execute the chain
		err := fn(ctx, c.args)
		if err != nil {
			return c.args.output, errors.Unwrap(err)
		}
	}

	return c.args.output, nil
}

// buildInterceptors wraps the given handleFunc with all interceptors in the chain
func (c *Chain[I, O]) buildInterceptors(fn HandleFunc[I, O]) HandleFunc[I, O] {
	handleFunc := func(ctx context.Context, args *Args[I, O]) error {
		if ctx.Err() != nil {
			return wrapError(fn, ctx.Err())
		}

		if err := fn(ctx, args); err != nil {
			return wrapError(fn, err)
		}

		return nil
	}
	for i := len(c.interceptors) - 1; i >= 0; i-- {
		handleFunc = c.interceptors[i](handleFunc)
	}
	return handleFunc
}

// wrapError returns a new error with function name and original error
func wrapError(fn any, err error) error {
	return fmt.Errorf("%s: %w", getFunctionName(fn), err)
}

// getFunctionName returns the name of the given function
func getFunctionName(fn any) string {
	name := runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
	parts := strings.Split(name, ".")
	return parts[len(parts)-1]
}
