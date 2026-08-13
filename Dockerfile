# Build stage
FROM golang:1.25.0-alpine AS builder

WORKDIR /app

# Install build dependencies for CGO (sqlite3 requires it)
RUN apk add --no-cache gcc musl-dev

# Download Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -o syncopation .

# Run stage
FROM alpine:3.21

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache tzdata ca-certificates

# Copy the binary and necessary directories from the builder stage
COPY --from=builder /app/syncopation .
COPY --from=builder /app/db ./db

# Ensure data directory exists for sqlite db
RUN mkdir -p /app/data

# Environment variables
ENV PORT=8080
ENV DB_PATH=/app/data/syncopation.sqlite

EXPOSE 8080

CMD ["./syncopation"]
