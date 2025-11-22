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

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/nytebubo .

# Create directory for config and database
RUN mkdir -p /root/.nytebubo

# Expose port if webhook server is used
EXPOSE 8080

# Run the binary
ENTRYPOINT ["./nytebubo"]
CMD ["agent"]
