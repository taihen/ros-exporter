FROM golang:1.25-alpine AS builder

ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ ./cmd/
COPY pkg/ ./pkg/

RUN CGO_ENABLED=0 GOOS=linux go build \
  -ldflags="-w -s -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
  -o /ros-exporter ./cmd/ros-exporter

FROM ghcr.io/taihen/base-image:v2025.10.09 As final

WORKDIR /bin/

COPY --from=builder /ros-exporter /bin/ros-exporter

LABEL org.opencontainers.image.source="https://github.com/taihen/ros-exporter"
LABEL org.opencontainers.image.description="Prometheus Exporter for MikroTik RouterOS"
LABEL org.opencontainers.image.licenses="MIT"

EXPOSE 9483

ENTRYPOINT ["/bin/ros-exporter"]
CMD ["-web.listen-address=:9483"]
