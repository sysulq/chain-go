package chain

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

// ChainFunc represents a function that operates on a Chain's Args
type ChainFunc[I, O any] func(context.Context, *Args[I, O]) error

// Interceptor represents a function that wraps a ChainFunc
type Interceptor[I, O any] func(ChainFunc[I, O]) ChainFunc[I, O]

// Chain represents a generic operation chain, supporting input type I and output type O
type Chain[I, O any] struct {
	args          *Args[I, O]
	ctx           context.Context
	fns           []ChainFunc[I, O]
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

// Serial adds one or more operations to be executed sequentially
func (c *Chain[I, O]) Serial(fns ...ChainFunc[I, O]) *Chain[I, O] {
	c.fns = append(c.fns, func(ctx context.Context, args *Args[I, O]) error {
		for _, fn := range fns {
			chainFunc := c.buildInterceptors(fn)
			if err := chainFunc(ctx, c.args); err != nil {
				return wrapError(fn, err)
			}
		}
		return nil
	})
	return c
}

// Parallel adds operations to be executed concurrently
func (c *Chain[I, O]) Parallel(fns ...ChainFunc[I, O]) *Chain[I, O] {
	c.fns = append(c.fns, func(ctx context.Context, args *Args[I, O]) error {
		g, ctx := errgroup.WithContext(ctx)

		if c.maxGoroutines > 0 {
			g.SetLimit(c.maxGoroutines)
		}
		for _, fn := range fns {
			fn := fn // https://golang.org/doc/faq#closures_and_goroutines
			g.Go(func() error {
				chainFunc := c.buildInterceptors(fn)
				if err := chainFunc(ctx, c.args); err != nil {
					return wrapError(fn, err)
				}
				return nil
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
			return c.args.output, err
		}
	}

	return c.args.output, nil
}

// buildInterceptors wraps the given ChainFunc with all interceptors in the chain
func (c *Chain[I, O]) buildInterceptors(fn ChainFunc[I, O]) ChainFunc[I, O] {
	chainFunc := func(ctx context.Context, args *Args[I, O]) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		return fn(ctx, args)
	}
	for i := len(c.interceptors) - 1; i >= 0; i-- {
		chainFunc = c.interceptors[i](chainFunc)
	}
	return chainFunc
}

// wrapError returns a new error with function name and original error
func wrapError(fn any, err error) error {
	name := runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
	parts := strings.Split(name, ".")
	name = parts[len(parts)-1]
	return fmt.Errorf("%s: %w", name, err)
}
