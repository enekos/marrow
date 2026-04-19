---
title: "Go Modules Guide"
lang: en
---

# Go Modules Guide

Go modules are the dependency management solution built into Go. They were introduced in Go 1.11 and became the default in Go 1.16.

## Creating a Module

To create a new module, run:

```bash
go mod init example.com/myproject
```

This creates a `go.mod` file that tracks your module's dependencies.

## Adding Dependencies

When you import a package not in the standard library, Go automatically adds it:

```bash
go get github.com/gin-gonic/gin
```

## Version Management

Go modules use semantic versioning. You can specify minimum versions:

```go
require (
    github.com/gin-gonic/gin v1.9.1
    github.com/stretchr/testify v1.8.4
)
```

## Vendoring

For reproducible builds, you can vendor dependencies:

```bash
go mod vendor
```

This copies all dependencies into a `vendor/` directory.
