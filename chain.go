package chain

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
)

// HandleFunc represents a function that operates on a Chain's State
type HandleFunc[I, O any] func(context.Context, *State[I, O]) error

// Interceptor represents a function that wraps a handleFunc
type Interceptor[I, O any] func(HandleFunc[I, O]) HandleFunc[I, O]

// Chain represents a generic operation chain, supporting input type I and output type O
type Chain[I, O any] struct {
	ctx           context.Context
	state         *State[I, O]
	fns           []HandleFunc[I, O]
	interceptors  []Interceptor[I, O]
	timeout       time.Duration
	maxGoroutines int
}

// State holds the input and output data and a mutex for synchronization
type State[I, O any] struct {
	mu         sync.RWMutex
	input      *I
	output     *O
	isParallel atomic.Bool
}

// New creates a new Chain, specifying input and output types
func New[I, O any](input *I, output *O) *Chain[I, O] {
	return &Chain[I, O]{
		ctx: context.Background(),
		state: &State[I, O]{
			input:  input,
			output: output,
		},
	}
}

// Input returns a copy of the input data of the Chain
//
// When the chain is running in parallel, it use a mutex to get the input data
func (s *State[I, O]) Input() I {
	if s.isParallel.Load() {
		s.mu.RLock()
		defer s.mu.RUnlock()
	}
	return *s.input
}

// Output returns a copy of the output data of the Chain
//
// When the chain is running in parallel, it use a mutex to get the output data
func (s *State[I, O]) Output() O {
	if s.isParallel.Load() {
		s.mu.RLock()
		defer s.mu.RUnlock()
	}
	return *s.output
}

// SetOutput sets the output data of the Chain
//
// When the chain is running in parallel, it use a mutex to set the output data
func (s *State[I, O]) SetOutput(fn func(*O)) {
	if s.isParallel.Load() {
		s.mu.Lock()
		defer s.mu.Unlock()
	}
	fn(s.output)
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

// Use adds interceptors to the chain to be executed before each operation
func (c *Chain[I, O]) Use(interceptors ...Interceptor[I, O]) *Chain[I, O] {
	c.interceptors = append(c.interceptors, interceptors...)
	return c
}

// Serial adds operations to be executed sequentially
func (c *Chain[I, O]) Serial(fns ...HandleFunc[I, O]) *Chain[I, O] {
	c.fns = append(c.fns, func(ctx context.Context, state *State[I, O]) error {
		for _, fn := range fns {
			fn := c.buildInterceptors(fn)
			if err := fn(ctx, state); err != nil {
				return err
			}
		}
		return nil
	})
	return c
}

// Parallel adds operations to be executed concurrently
func (c *Chain[I, O]) Parallel(fns ...HandleFunc[I, O]) *Chain[I, O] {
	c.fns = append(c.fns, func(ctx context.Context, state *State[I, O]) error {
		state.isParallel.Store(true)
		defer state.isParallel.Store(false)

		g, ctx := errgroup.WithContext(ctx)
		if c.maxGoroutines > 0 {
			g.SetLimit(c.maxGoroutines)
		}

		for _, fn := range fns {
			fn := c.buildInterceptors(fn)
			g.Go(func() error { return fn(ctx, state) })
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
		err := fn(ctx, c.state)
		if err != nil {
			return c.state.output, err
		}
	}

	return c.state.output, nil
}

// buildInterceptors wraps the given handleFunc with all interceptors in the chain
func (c *Chain[I, O]) buildInterceptors(fn HandleFunc[I, O]) HandleFunc[I, O] {
	handleFunc := func(ctx context.Context, state *State[I, O]) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		return fn(ctx, state)
	}
	for i := len(c.interceptors) - 1; i >= 0; i-- {
		handleFunc = c.interceptors[i](handleFunc)
	}
	return handleFunc
}
