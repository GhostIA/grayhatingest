# Stage 1: Build the application
FROM golang:1.23-bookworm as builder

# Set the working directory
WORKDIR /app

# Install necessary libraries

RUN apt-get update
RUN apt-get install -y libc6-dev libc6 

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Run tests
RUN go test ./... -v

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o ingestor .

# Stage 2: Create the runtime image
FROM debian:bookworm-slim

# Install dependencies for the runtime
RUN apt-get update && apt-get install -y ca-certificates libc6 && rm -rf /var/lib/apt/lists/*

# Set the working directory
WORKDIR /app

# Copy the built binary from the builder stage
COPY --from=builder /app/ingestor .

# Expose necessary ports (optional, if the app exposes ports)
EXPOSE 8080

# Create a non-root user and group
RUN groupadd -r svc-ingest
RUN useradd -r -g svc-ingest svc-ingest

# Give permission on app dir
RUN chown -R svc-ingest:svc-ingest /app

# Switch to nonroot
USER svc-ingest

# Set the entrypoint
CMD ["./ingestor"]
