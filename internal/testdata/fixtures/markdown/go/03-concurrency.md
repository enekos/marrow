---
title: "Go Concurrency Patterns"
lang: en
---

# Go Concurrency Patterns

Concurrency is one of Go's strongest features. The language provides first-class support for concurrent programming through goroutines and channels.

## Goroutines

A goroutine is a lightweight thread managed by the Go runtime. Starting one is as simple as:

```go
go functionName()
```

Goroutines are multiplexed onto OS threads automatically, so you can create thousands without significant overhead.

## Channels

Channels are typed conduits for communication between goroutines:

```go
ch := make(chan int)
go func() { ch <- 42 }()
value := <-ch
```

## Select Statement

The `select` statement lets you wait on multiple channel operations:

```go
select {
case v1 := <-ch1:
    fmt.Println("ch1:", v1)
case v2 := <-ch2:
    fmt.Println("ch2:", v2)
case ch3 <- 100:
    fmt.Println("sent to ch3")
}
```

## Worker Pools

A common pattern for limiting concurrent work:

```go
jobs := make(chan int, 100)
results := make(chan int, 100)

for w := 1; w <= 3; w++ {
    go worker(w, jobs, results)
}
```

## Context Package

For cancellation and timeouts:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
```
