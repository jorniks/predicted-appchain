# Stage 1: Build
FROM golang:1.25-alpine AS builder
WORKDIR /app

# Install build tools
RUN apk add --no-cache git build-base

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build from the cmd directory
RUN go build -o appchain ./cmd

# Stage 2: Runtime
FROM alpine:latest
WORKDIR /app

# Create data directory for MDBX
RUN mkdir -p /data

# Copy the built binary
COPY --from=builder /app/appchain .

# Ensure it’s executable
RUN chmod +x ./appchain

# Expose port
EXPOSE 8080

# Run app
CMD ["./appchain", "-rpc-port", ":8080", "-db-path", "/data/appchain.mdbx", "-local-db-path", "/data/local.mdbx"]
# End of Dockerfile