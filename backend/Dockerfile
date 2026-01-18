# Go Backend Dockerfile
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=1 GOOS=linux go build -o server ./cmd/server/main.go

# Production image
FROM alpine:3.19

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates sqlite-libs

# Copy binary from builder
COPY --from=builder /app/server .

# Create data directory
RUN mkdir -p /app/data

# Environment variables
ENV PORT=8080
ENV DB_PATH=/app/data/tcg.db

# Expose port
EXPOSE 8080

# Run the server
CMD ["./server"]
