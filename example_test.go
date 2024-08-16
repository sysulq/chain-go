package chain_test

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/sysulq/chain-go"
)

// Input represents the input data structure
type Input struct {
	Numbers []int
}

// Output represents the output data structure
type Output struct {
	Sum       int
	Product   int
	Processed bool
}

func Example() {
	slog.SetLogLoggerLevel(slog.LevelDebug)

	// Initialize input and output
	input := &Input{Numbers: []int{1, 2, 3, 4, 5}}
	output := &Output{}

	// Create a new chain
	c := chain.New(input, output).
		WithTimeout(5*time.Second).
		Use(chain.RecoverInterceptor, chain.LogInterceptor)

	// Define chain operations
	c.Serial(
		func(ctx context.Context, c *chain.State[Input, Output]) error {
			fmt.Println("Starting serial operations")
			return nil
		},
		calculateSum,
	).Parallel(
		simulateSlowOperation,
		calculateProduct,
	).Serial(
		markProcessed,
	)

	// Execute the chain
	result, err := c.Execute()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Sum: %d, Product: %d, Processed: %v\n", result.Sum, result.Product, result.Processed)

	// Output:
	// Starting serial operations
	// Calculating sum
	// Calculating product
	// Simulating slow operation
	// Marking as processed
	// Sum: 15, Product: 120, Processed: true
}

func calculateSum(ctx context.Context, c *chain.State[Input, Output]) error {
	fmt.Println("Calculating sum")
	sum := 0
	for _, num := range c.Input().Numbers {
		sum += num
	}
	c.SetOutput(func(o *Output) {
		o.Sum = sum
	})
	return nil
}

func calculateProduct(ctx context.Context, c *chain.State[Input, Output]) error {
	fmt.Println("Calculating product")
	product := 1
	for _, num := range c.Input().Numbers {
		product *= num
	}
	c.SetOutput(func(o *Output) {
		o.Product = product
	})
	return nil
}

func simulateSlowOperation(ctx context.Context, c *chain.State[Input, Output]) error {
	select {
	case <-time.After(100 * time.Millisecond):
		fmt.Println("Simulating slow operation")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func markProcessed(ctx context.Context, c *chain.State[Input, Output]) error {
	fmt.Println("Marking as processed")
	c.SetOutput(func(o *Output) {
		o.Processed = true
	})
	return nil
}
