---
title: "Programación en Go"
lang: es
---

# Programación en Go

Go es un lenguaje de programación de código abierto que facilita la construcción de software simple, confiable y eficiente.

## Características Principales

- **Tipado estático** con sintaxis similar a C
- **Recolección de basura** para gestión de memoria
- **Concurrencia integrada** con goroutines y canales
- **Compilación rápida** a código máquina

## Hola Mundo

```go
package main

import "fmt"

func main() {
    fmt.Println("¡Hola, Mundo!")
}
```

## Goroutines

Las goroutines son hilos ligeros gestionados por el runtime de Go:

```go
go funcion()
```

## Canales

Los canales son conductos tipados para comunicación entre goroutines:

```go
ch := make(chan int)
go func() { ch <- 42 }()
valor := <-ch
```

Go fue diseñado para abordar las necesidades del desarrollo de software a gran escala.
