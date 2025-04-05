# Start with a golang base image for building
FROM golang:1.24.1-alpine AS builder

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum files first to leverage Docker cache
COPY go.mod go.sum* ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application as a static binary
# CGO_ENABLED=0 prevents linking against C libraries (like glibc/musl)
# GOOS=linux ensures it's built for the Linux kernel (Alpine's kernel)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o biu_email .

# --- Final Stage ---
# Use a minimal alpine image
FROM alpine:3.19

# Install ca-certificates for HTTPS support (if needed by the app)
RUN apk --no-cache add ca-certificates

# Create a non-root user and group
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Set the working directory
WORKDIR /app

# Copy the binary and config file from the builder stage
COPY --from=builder --chown=appuser:appgroup /app/biu_email .
COPY --from=builder --chown=appuser:appgroup /app/config.yaml .

# Create directories and set permissions
RUN mkdir -p /app/{messages,temp-files,logs,storage} && \
    chown -R appuser:appgroup /app && \
    chmod -R 775 /app/storage && \
    chmod 664 /app/biu_email.db || true

# Switch to the non-root user
USER appuser

# Expose the port the application listens on (defined in config.yaml)
EXPOSE 3003

# Define the command to run the application with config file
CMD ["./biu_email", "-config", "/app/config.yaml"]
