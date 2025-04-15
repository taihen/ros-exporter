# Stage 1: Build the exporter
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy Go module files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY cmd/ ./cmd/
COPY pkg/ ./pkg/

# Build the static binary
# CGO_ENABLED=0 produces a static binary suitable for scratch/distroless images
# -ldflags="-w -s" strips debug information and symbols to reduce binary size
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /ros-exporter ./cmd/ros-exporter

# Stage 2: Create the final minimal image
FROM gcr.io/distroless/static-debian11 AS final
# Alternatively, use 'FROM scratch' for an even smaller image,
# but distroless includes basics like CA certificates if needed later.

WORKDIR /bin/

# Copy the static binary from the builder stage
COPY --from=builder /ros-exporter /bin/ros-exporter

# Metadata
LABEL org.opencontainers.image.source="https://github.com/taihen/ros-exporter"
LABEL org.opencontainers.image.description="Prometheus Exporter for MikroTik RouterOS"
LABEL org.opencontainers.image.licenses="MIT"

# Expose the default port
EXPOSE 9483

# Set the entrypoint
ENTRYPOINT ["/bin/ros-exporter"]

# Default command (can be overridden) - includes default flags
CMD ["-web.listen-address=:9483"]
