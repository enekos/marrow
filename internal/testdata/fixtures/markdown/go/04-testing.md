---
title: "Go Testing Best Practices"
lang: en
---

# Go Testing Best Practices

Go has a built-in testing framework in the `testing` package. Writing good tests is essential for maintaining reliable software.

## Table-Driven Tests

The idiomatic Go pattern:

```go
func TestAdd(t *testing.T) {
    tests := []struct {
        a, b, want int
    }{
        {1, 2, 3},
        {0, 0, 0},
        {-1, 1, 0},
    }
    for _, tt := range tests {
        got := Add(tt.a, tt.b)
        if got != tt.want {
            t.Errorf("Add(%d,%d) = %d, want %d", tt.a, tt.b, got, tt.want)
        }
    }
}
```

## Benchmarks

```go
func BenchmarkFibonacci(b *testing.B) {
    for i := 0; i < b.N; i++ {
        Fibonacci(20)
    }
}
```

## Subtests

Use `t.Run()` for organized test output:

```go
func TestAuth(t *testing.T) {
    t.Run("valid token", func(t *testing.T) {
        // test code
    })
    t.Run("expired token", func(t *testing.T) {
        // test code
    })
}
```

## Mocking

Use interfaces for testability:

```go
type Storage interface {
    Get(id string) (*Item, error)
    Set(id string, item *Item) error
}
```
