# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev sqlite-dev

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o nytebubo .

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates sqlite-libs

# Copy the binary to /usr/local/bin
COPY --from=builder /app/nytebubo /usr/local/bin/nytebubo

# Set working directory for config and data
WORKDIR /data

# Expose port if webhook server is used
EXPOSE 8080

# Run the binary
ENTRYPOINT ["nytebubo"]
CMD ["agent"]
