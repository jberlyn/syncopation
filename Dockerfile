# Build stage
FROM golang:1.25.0-alpine AS builder

# Install tzdata and ca-certificates for the final image
RUN apk add --no-cache tzdata ca-certificates

WORKDIR /app

# Download Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the statically linked application with stripped debugging symbols and paths
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o syncopation .

# Create data directory so we can copy it to the scratch container
RUN mkdir -p /app/data

# Run stage
FROM scratch

WORKDIR /app

# Copy timezones and CA certificates from builder
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the binary and necessary directories from the builder stage
COPY --from=builder /app/syncopation .
COPY --from=builder /app/db ./db
COPY --from=builder /app/data ./data

# Environment variables
ENV PORT=22300
ENV DB_PATH=/app/data/syncopation.sqlite

EXPOSE 22300

CMD ["./syncopation"]
