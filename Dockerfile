FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ ./cmd/
COPY pkg/ ./pkg/

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /ros-exporter ./cmd/ros-exporter

FROM gcr.io/distroless/static-debian12 AS final

WORKDIR /bin/

COPY --from=builder /ros-exporter /bin/ros-exporter

LABEL org.opencontainers.image.source="https://github.com/taihen/ros-exporter"
LABEL org.opencontainers.image.description="Prometheus Exporter for MikroTik RouterOS"
LABEL org.opencontainers.image.licenses="MIT"

EXPOSE 9483

ENTRYPOINT ["/bin/ros-exporter"]
CMD ["-web.listen-address=:9483"]
