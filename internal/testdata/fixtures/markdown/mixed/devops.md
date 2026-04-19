---
title: "DevOps Best Practices"
lang: en
---

# DevOps Best Practices

DevOps is a set of practices that combines software development and IT operations.

## CI/CD

Continuous Integration and Continuous Deployment are essential:

```yaml
name: CI
on: [push]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
      - run: go test ./...
```

## Infrastructure as Code

Manage infrastructure with code using Terraform or Pulumi:

```hcl
resource "aws_instance" "web" {
  ami           = "ami-0c55b159cbfafe1f0"
  instance_type = "t2.micro"
}
```

## Monitoring

- Prometheus for metrics
- Grafana for visualization
- Jaeger for distributed tracing

## Containerization

Docker and Kubernetes are the standard:

```dockerfile
FROM golang:1.21-alpine
WORKDIR /app
COPY . .
RUN go build -o server .
CMD ["./server"]
```
