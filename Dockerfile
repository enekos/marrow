# Build stage
FROM golang:1.25-alpine AS builder
RUN apk add --no-cache gcc musl-dev sqlite-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -tags sqlite_fts5 -ldflags="-w -s" -o marrow .

# Runtime stage
FROM alpine:latest
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/marrow /usr/local/bin/marrow
EXPOSE 8080
ENTRYPOINT ["marrow"]
CMD ["serve"]
