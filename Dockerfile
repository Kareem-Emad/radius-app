# Multi-stage build for RADIUS server
FROM golang:1.24-alpine AS builder

# Set working directory
WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy all source code
COPY . ./

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o radius-server ./cmd/api

# Final stage - minimal runtime image
FROM alpine:latest

# Install ca-certificates for HTTPS requests if needed
RUN apk --no-cache add ca-certificates

# Create non-root user for security
RUN addgroup -g 1001 -S radius && \
    adduser -u 1001 -S radius -G radius

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/radius-server .

# Change ownership to radius user
RUN chown radius:radius radius-server

# Switch to non-root user
USER radius

# Expose ports 1812 (Authentication) and 1813 (Accounting)
EXPOSE 1812/udp 1813/udp

# Default environment variables for Redis connection
ENV REDIS_HOST=redis
ENV REDIS_PORT=6379

# Health check to ensure the server is running
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD nc -zu localhost 1812 && nc -zu localhost 1813 || exit 1

# Run the RADIUS server
CMD ["./radius-server"]