---
title: "Async Programming in Rust"
lang: en
---

# Async Programming in Rust

Rust's async programming model allows you to run multiple tasks concurrently on a single thread.

## Futures

A `Future` represents an asynchronous computation:

```rust
async fn hello_world() {
    println!("hello, world!");
}
```

## Await

The `.await` keyword pauses execution until the future completes:

```rust
async fn learn_and_sing() {
    let song = learn_song().await;
    sing_song(song).await;
}
```

## Tokio Runtime

Tokio is the most popular async runtime:

```rust
#[tokio::main]
async fn main() {
    let result = fetch_data().await;
    println!("{}", result);
}
```

## Select

Wait for multiple futures:

```rust
tokio::select! {
    result = task1 => println!("task1 done: {}", result),
    result = task2 => println!("task2 done: {}", result),
}
```

## Streams

Async iterators for handling sequences:

```rust
use tokio_stream::StreamExt;

let mut stream = tokio::net::TcpListener::bind("127.0.0.1:8080").await?;
while let Ok((socket, _)) = stream.accept().await {
    tokio::spawn(handle_connection(socket));
}
```
