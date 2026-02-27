# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git make

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN make build

# Runtime stage
FROM alpine:3.19

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /app/bin/responses2chat /app/responses2chat
COPY --from=builder /app/configs/config.yaml /app/configs/config.yaml

# Expose port
EXPOSE 8080

# Run
ENTRYPOINT ["/app/responses2chat"]
CMD ["-c", "/app/configs/config.yaml"]
