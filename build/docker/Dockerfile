# Build stage
FROM golang:1.25.4-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN go build -o bin/codecurfew ./cmd/app

# Final stage
FROM alpine:3.23

# Install ca-certificates, su-exec, and curl for HTTPS requests, user switching, and healthchecks
RUN apk --no-cache add ca-certificates su-exec curl

# Create a non-root user
RUN addgroup -g 1000 app && adduser -D -u 1000 -G app app

# Create logs directory with proper ownership
RUN mkdir -p /var/log/codecurfew && chown app:app /var/log/codecurfew

# Copy the binary from builder stage
COPY --from=builder /app/bin/codecurfew /usr/local/bin/codecurfew

# Copy entrypoint script
COPY build/docker/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Expose the port
EXPOSE 8080

# Set entrypoint
ENTRYPOINT ["/entrypoint.sh"]