# syntax=docker/dockerfile:1
# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.* ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o biu_email

# Final stage
FROM alpine:latest

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/biu_email /app/
# Copy config example
COPY config.yaml.example /app/config.yaml

# Create necessary directories
RUN mkdir -p /app/messages /app/temp-files /app/storage /app/uploads \
    && chmod -R 750 /app \
    && adduser -D -H -s /sbin/nologin biu \
    && chown -R biu:biu /app

USER biu

EXPOSE 3003

ENTRYPOINT ["/app/biu_email"]


