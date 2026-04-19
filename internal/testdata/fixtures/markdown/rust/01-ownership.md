---
title: "Understanding Ownership in Rust"
lang: en
---

# Understanding Ownership in Rust

Ownership is Rust's most unique feature and enables memory safety guarantees without a garbage collector.

## The Ownership Rules

1. Each value in Rust has a variable that's its *owner*.
2. There can only be one owner at a time.
3. When the owner goes out of scope, the value is dropped.

## Variable Scope

```rust
{                      // s is not valid here
    let s = "hello";   // s is valid from this point forward
    // do stuff with s
}                      // this scope is now over, and s is no longer valid
```

## Move Semantics

```rust
let s1 = String::from("hello");
let s2 = s1;  // s1 is moved into s2
// println!("{}", s1); // ERROR! s1 is no longer valid
```

## Borrowing

References allow you to borrow without taking ownership:

```rust
fn calculate_length(s: &String) -> usize {
    s.len()
}

let s1 = String::from("hello");
let len = calculate_length(&s1);
// s1 is still valid here
```

## Mutable References

```rust
fn change(some_string: &mut String) {
    some_string.push_str(", world");
}
```

## Memory Safety

The borrow checker enforces these rules at compile time, preventing:
- Dangling pointers
- Double frees
- Data races
