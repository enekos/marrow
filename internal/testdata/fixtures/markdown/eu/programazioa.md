---
title: "Programazioa Go-n"
lang: eu
---

# Programazioa Go-n

Go programa lengoaia irekia da, software sinple, fidagarri eta eraginkorra sortzen laguntzen duena.

## Ezaugarri Nagusiak

- **Mota estatikoa** C-ren sintaxi antzekoa
- **Zabor bilketa** memoria kudeaketarako
- **Euskarri berezia** goroutine eta kanalen bidez
- **Konpilazio azkarra** makina koderako

## Kaixo Mundua

```go
package main

import "fmt"

func main() {
    fmt.Println("Kaixo, Mundua!")
}
```

## Goroutines

Goroutine-ak Go runtime-ak kudeatzen dituen hari arinak dira.

## Kanalak

Kanalak goroutine-en arteko komunikaziorako dira:

```go
ch := make(chan int)
go func() { ch <- 42 }()
balioa := <-ch
```

Go Google-n garatu zen software garapen handirako.
