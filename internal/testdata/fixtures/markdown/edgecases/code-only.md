---
title: "Code Heavy Document"
lang: en
---

```go
package main

import (
    "fmt"
    "sync"
)

func main() {
    var wg sync.WaitGroup
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(n int) {
            defer wg.Done()
            fmt.Printf("Worker %d done\n", n)
        }(i)
    }
    wg.Wait()
}
```

```rust
fn main() {
    let v = vec![1, 2, 3];
    let handle = std::thread::spawn(move || {
        println!("{:?}", v);
    });
    handle.join().unwrap();
}
```

```python
import asyncio

async def main():
    await asyncio.sleep(1)
    print("done")

asyncio.run(main())
```
