# Chain: A Generic Operation Chain for Go

[![Go](https://github.com/sysulq/chain-go/actions/workflows/go.yml/badge.svg)](https://github.com/sysulq/chain-go/actions/workflows/go.yml)

## Introduction

Chain is a powerful and flexible Go package that provides a generic operation chain for executing a series of tasks in a structured and controlled manner. It supports both serial and parallel execution, error handling, interceptor, and context management, making it ideal for complex workflows and data processing pipelines.

## Features

- **Generic Implementation**: Works with any input and output types.
- **Serial and Parallel Execution**: Supports both sequential and concurrent task execution.
- **Context Management**: Integrates with Go's context package for cancellation and timeout control.
- **Error Handling**: Gracefully handles errors and stops execution on first encountered error.
- **Thread-Safe**: Provides mutex-based synchronization for shared resource access.
- **Timeout Control**: Allows setting a timeout for the entire chain execution.
- **Fluent Interface**: Offers a chainable API for easy and readable setup.
- **Interceptor Support**: Enables custom logic before and after each operation.

## Installation

To install Chain, use `go get`:

```bash
go get github.com/sysulq/chain-go
```

## Usage

### Basic Example

Here's a simple example demonstrating the basic usage of Chain:

```go
package chain_test

import (
	"context"
	"fmt"
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
	// Initialize input and output
	input := &Input{Numbers: []int{1, 2, 3, 4, 5}}
	output := &Output{}

	// Create a new chain
	c := chain.New(input, output).
		WithTimeout(5 * time.Second).
		Use(chain.RecoverInterceptor)

	// Define chain operations
	c.Serial(
		func(ctx context.Context, c *chain.Args[Input, Output]) error {
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

func calculateSum(ctx context.Context, c *chain.Args[Input, Output]) error {
	fmt.Println("Calculating sum")
	sum := 0
	for _, num := range c.Input().Numbers {
		sum += num
	}
	c.Output().Sum = sum
	return nil
}

func calculateProduct(ctx context.Context, c *chain.Args[Input, Output]) error {
	fmt.Println("Calculating product")
	product := 1
	for _, num := range c.Input().Numbers {
		product *= num
	}
	c.WithLock(func() {
		c.Output().Product = product
	})
	return nil
}

func simulateSlowOperation(ctx context.Context, c *chain.Args[Input, Output]) error {
	select {
	case <-time.After(100 * time.Millisecond):
		fmt.Println("Simulating slow operation")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func markProcessed(ctx context.Context, c *chain.Args[Input, Output]) error {
	fmt.Println("Marking as processed")
	c.Output().Processed = true
	return nil
}
```

### Advanced Features

1. **Using Context**:
   ```go
   ctx := context.WithValue(context.Background(), "key", "value")
   c := chain.New(input, output).WithContext(ctx)
   ```

2. **Error Handling**:
   ```go
   c.Serial(func(ctx context.Context, args *chain.Args[Input, Output]) error {
       return fmt.Errorf("an error occurred")
   })
   ```

3. **Panic Recovery**:
   ```go
   c.Serial(func(ctx context.Context, args *chain.Args[Input, Output]) error {
       panic("unexpected error")
   })
   // This panic will be recovered and converted to an error
   ```

4. **Thread-Safe Operations**:
   ```go
   c.Parallel(func(ctx context.Context, args *chain.Args[Input, Output]) error {
       args.WithLock(func() {
           // Thread-safe operation
       })
       return nil
   })
   ```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
