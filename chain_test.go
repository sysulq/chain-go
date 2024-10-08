package chain_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sysulq/chain-go"
)

type TestInput struct {
	Value int
}

type TestOutput struct {
	Result string
}

type contextKey string

func TestNew(t *testing.T) {
	input := &TestInput{Value: 5}
	output := &TestOutput{}
	c := chain.New(input, output)

	if _, err := c.Execute(); err != nil {
		t.Error("New should create a non-nil Chain")
	}
}

func TestWithContext(t *testing.T) {
	input := &TestInput{Value: 5}
	output := &TestOutput{}
	ctx := context.WithValue(context.Background(), contextKey("key"), "value")
	_ = chain.New(input, output).
		WithContext(ctx).
		Serial(func(ctx context.Context, chain *chain.State[TestInput, TestOutput]) error {
			if ctx.Value("key") != "value" {
				t.Error("WithContext did not set the context correctly")
			}
			return nil
		})
}

func TestSerial(t *testing.T) {
	input := &TestInput{Value: 5}
	output := &TestOutput{}
	c := chain.New(input, output)

	c.Serial(func(_ context.Context, chain *chain.State[TestInput, TestOutput]) error {
		value := chain.Input().Value
		fmt.Println(chain.Output())
		chain.SetOutput(func(to *TestOutput) {
			to.Result = strconv.Itoa(value * 2)
		})
		return nil
	})

	result, err := c.Execute()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result.Result != "10" {
		t.Errorf("Expected result 10, got %s", result.Result)
	}
}

func TestParallel(t *testing.T) {
	slog.SetLogLoggerLevel(slog.LevelDebug)

	input := &TestInput{Value: 5}
	output := &TestOutput{}
	c := chain.New(input, output).WithMaxGoroutines(10).Use(chain.LogInterceptor)
	c.Parallel(func(ctx context.Context, s *chain.State[TestInput, TestOutput]) error {
		fmt.Println(s.Input(), s.Output())
		return nil
	})

	c.Parallel(
		func(_ context.Context, chain *chain.State[TestInput, TestOutput]) error {
			fmt.Println(chain.Input(), chain.Output())
			chain.SetOutput(func(o *TestOutput) {
				o.Result += "A"
			})
			return nil
		},
		func(_ context.Context, chain *chain.State[TestInput, TestOutput]) error {
			fmt.Println(chain.Input(), chain.Output())
			chain.SetOutput(func(o *TestOutput) {
				o.Result += "B"
			})
			return nil
		},
		func(_ context.Context, chain *chain.State[TestInput, TestOutput]) error {
			fmt.Println(chain.Input(), chain.Output())
			chain.SetOutput(func(o *TestOutput) {
				o.Result += "C"
			})
			return nil
		},
	).Parallel(
		func(_ context.Context, chain *chain.State[TestInput, TestOutput]) error {
			fmt.Println(chain.Input(), chain.Output())
			chain.SetOutput(func(o *TestOutput) {
				o.Result += "A"
			})
			return nil
		},
		func(_ context.Context, chain *chain.State[TestInput, TestOutput]) error {
			fmt.Println(chain.Input(), chain.Output())
			chain.SetOutput(func(o *TestOutput) {
				o.Result += "B"
			})
			return nil
		},
		func(_ context.Context, chain *chain.State[TestInput, TestOutput]) error {
			fmt.Println(chain.Input(), chain.Output())
			chain.SetOutput(func(o *TestOutput) {
				o.Result += "C"
			})
			return nil
		},
	)

	result, err := c.Execute()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(result.Result) != 6 {
		t.Errorf("Expected result length 3, got %v", result.Result)
	}
}

func TestErrorHandling(t *testing.T) {
	input := &TestInput{Value: 5}
	output := &TestOutput{}
	c := chain.New(input, output).Use(chain.LogInterceptor)

	testError := errors.New("test error")

	c.Serial(func(_ context.Context, chain *chain.State[TestInput, TestOutput]) error {
		return testError
	})

	_, err := c.Execute()
	if err == nil || !errors.Is(err, testError) {
		t.Errorf("Expected 'test error', got %v", err)
	}
}

func TestContextCancellation(t *testing.T) {
	input := &TestInput{Value: 5}
	output := &TestOutput{}
	ctx, cancel := context.WithCancel(context.Background())
	c := chain.New(input, output).WithContext(ctx)

	cancel() // Cancel the context

	c.Serial(func(_ context.Context, chain *chain.State[TestInput, TestOutput]) error {
		t.Error("Function should not be called after context cancellation")
		return nil
	})

	_, err := c.Execute()
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}
}

func TestParallelErrorHandling(t *testing.T) {
	input := &TestInput{Value: 5}
	output := &TestOutput{}
	c := chain.New(input, output)

	testError := errors.New("test error")

	c.Parallel(
		func(_ context.Context, chain *chain.State[TestInput, TestOutput]) error {
			return testError
		},
		func(_ context.Context, chain *chain.State[TestInput, TestOutput]) error {
			time.Sleep(100 * time.Millisecond) // Simulate some work
			return nil
		},
	)

	_, err := c.Execute()
	if err == nil || !errors.Is(err, testError) {
		t.Errorf("Expected 'parallel error', got %v", err)
	}
}

func TestTimeout(t *testing.T) {
	input := &TestInput{Value: 5}
	output := &TestOutput{}
	c := chain.New(input, output)

	c.Serial(func(ctx context.Context, chain *chain.State[TestInput, TestOutput]) error {
		time.Sleep(100 * time.Millisecond)
		return ctx.Err()
	})

	c.WithTimeout(50 * time.Millisecond)

	_, err := c.Execute()
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected context.DeadlineExceeded error, got %v", err)
	}
}

func TestRecover(t *testing.T) {
	input := &TestInput{Value: 5}
	output := &TestOutput{}
	c := chain.New(input, output).Use(chain.RecoverInterceptor)

	c.Serial(func(_ context.Context, chain *chain.State[TestInput, TestOutput]) error {
		panic("panic")
	})

	_, err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "panic in execution") {
		t.Errorf("Expected 'panic in execution\n', got %v", err)
	}

	chain.New(input, output).Use(chain.RecoverInterceptor).
		Parallel(func(ctx context.Context, c *chain.State[TestInput, TestOutput]) error {
			panic("panic")
		})

	_, err = c.Execute()
	if err == nil || !strings.Contains(err.Error(), "panic in execution") {
		t.Errorf("Expected 'panic in execution\n', got %v", err)
	}
}

func TestWrapError(t *testing.T) {
	input := &TestInput{Value: 5}
	output := &TestOutput{}
	c := chain.New(input, output).Use(chain.LogInterceptor)

	c.Serial(testWrapError)

	_, err := c.Execute()
	if err == nil || !errors.Is(err, errTest) {
		t.Errorf("Expected 'test error', got %v", err)
	}
}

var errTest = errors.New("test error")

func testWrapError(ctx context.Context, c *chain.State[TestInput, TestOutput]) error {
	return errTest
}

func TestContextInInterceptor(t *testing.T) {
	slog.SetLogLoggerLevel(slog.LevelDebug)

	input := &TestInput{Value: 5}
	output := &TestOutput{}
	c := chain.New(input, output).Use(chain.LogInterceptor)

	c.Use(func(cf chain.HandleFunc[TestInput, TestOutput]) chain.HandleFunc[TestInput, TestOutput] {
		return func(ctx context.Context, args *chain.State[TestInput, TestOutput]) error {
			ctx = context.WithValue(ctx, contextKey("key"), "value")
			return cf(ctx, args)
		}
	}).Serial(
		func(ctx context.Context, chain *chain.State[TestInput, TestOutput]) error {
			if ctx.Value(contextKey("key")) != "value" {
				t.Error("WithContext did not set the context correctly")
			}
			return nil
		})

	_, err := c.Execute()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}
