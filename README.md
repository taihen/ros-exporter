# MikroTik RouterOS Prometheus Exporter (ros-exporter)

[![Test](https://github.com/taihen/ros-exporter/actions/workflows/test.yml/badge.svg)](https://github.com/taihen/ros-exporter/actions/workflows/test.yml)
[![Release](https://github.com/taihen/ros-exporter/actions/workflows/release.yml/badge.svg)](https://github.com/taihen/ros-exporter/actions/workflows/release.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/taihen/ros-exporter)](https://goreportcard.com/report/github.com/taihen/ros-exporter)

A Prometheus exporter for MikroTik RouterOS devices.

This exporter connects to MikroTik routers using the native API (via the `go-routeros/routeros` library) and exposes metrics for monitoring with Prometheus.

## Features

- System resources (CPU, memory, storage, uptime, board info) — always on
- System health (temperature, board temperature, voltage, current, power, fan)
- Interface stats (traffic, packets, errors, drops, admin state, speed, duplex)
- Optional: BGP, PPP (incl. RX/TX bytes), wireless (legacy + wifiwave2/`/interface/wifi`), OSPF, transceiver optics
- Multi-target `/metrics?target=...` (default port `9483`)
- Single scrape deadline (`-scrape.timeout`), connection concurrency limit
- Status metrics: connected vs scrape success vs per-collector error
- Process health: `/-/healthy`, `/-/ready`, `/-/metrics` (Go/process collectors)

## Requirements

- Go 1.25+
- MikroTik RouterOS v6.48 or later (tested paths for v6.x and v7.x)
- A dedicated read-only API user on each router

## Breaking changes since v1.4.0

| Change | Impact |
|--------|--------|
| Removed `uptime_text` label from PPP and wireless client info metrics | Series IDs change; use `*_uptime_seconds` instead |
| BGP peer info label `instance` renamed to `routing_instance` | Avoids clash with Prometheus `instance` |
| `mikrotik_up` means **API connected**, not full scrape success | Prefer `mikrotik_scrape_success` / `mikrotik_last_scrape_error` for completeness |
| New metrics: `mikrotik_connected`, `mikrotik_scrape_success`, `mikrotik_collector_error`, `mikrotik_collector_supported` | Update dashboards/alerts |

Existing alert `up{job="ros_exporter"} * mikrotik_up` still works for reachability. Add a second alert on `mikrotik_last_scrape_error == 1` for partial failures. See [docs/ALERTS.md](docs/ALERTS.md).

## Getting Started

### Building

```bash
go build -ldflags="-X main.version=dev -X main.commit=$(git rev-parse --short HEAD)" -o ros-exporter ./cmd/ros-exporter
```

### Running

```bash
./ros-exporter [flags]
```

**Flags:**

- `-web.listen-address`: Listen address (default `:9483`)
- `-web.telemetry-path`: Target metrics path (default `/metrics`)
- `-scrape.timeout`: Budget for the **entire** scrape of one target (default `10s`)
- `-scrape.max-concurrent`: Max concurrent RouterOS API connections (default `25`)
- `-version`: Print version and exit

### Endpoints

| Path | Purpose |
|------|---------|
| `/metrics?target=HOST` | Per-target RouterOS metrics |
| `/-/metrics` | Exporter process / Go metrics |
| `/-/healthy` | Liveness |
| `/-/ready` | Readiness |

Optional query params: `user`, `password`, `port`, `collect_bgp`, `collect_ppp`, `collect_wireless`, `collect_ospf`, `collect_optics`.

### MikroTik Configuration

```mikrotik
/user group add name=prometheus policy=read,api
/user add name=prometheus group=prometheus password=YOUR_STRONG_PASSWORD address=EXPORTER_IP_ADDRESS
```

Allow API (`/ip service`) from the exporter host.

### Prometheus Configuration

```yaml
scrape_configs:
  - job_name: 'mikrotik'
    scrape_interval: 1m
    scrape_timeout: 50s
    metrics_path: /metrics
    static_configs:
      - targets: ['192.168.88.1']
    relabel_configs:
      - source_labels: [__address__]
        target_label: __param_target
      - source_labels: [__param_target]
        target_label: instance
      - target_label: __address__
        replacement: localhost:9483
      - target_label: __param_password
        replacement: YOUR_STRONG_PASSWORD
```

Optional params: `collect_bgp`, `collect_ppp`, `collect_wireless`, `collect_ospf`, `collect_optics`.

Set Prometheus `scrape_timeout` **greater than** exporter `-scrape.timeout`.

## Metrics (status)

| Metric | Meaning |
|--------|---------|
| `mikrotik_up` / `mikrotik_connected` | API TCP login succeeded |
| `mikrotik_scrape_success` | No collector errors in this scrape |
| `mikrotik_last_scrape_error` | Inverse signal for partial/full scrape errors |
| `mikrotik_collector_error{collector=...}` | Per-collector failure |
| `mikrotik_collector_supported{collector=...}` | Collector enabled/supported on target |
| `mikrotik_build_info{version,commit}` | Exporter build |
| `mikrotik_scrape_duration_seconds` | Scrape duration |

### Always collected

- System: CPU, memory, uptime, storage, board info
- Interfaces: RX/TX bytes/packets/errors/drops, operational status, admin up, speed, duplex (when reported)
- Health: CPU/board temperature, voltage, current, power, fan

### Optional

- BGP peers (label `routing_instance`)
- PPP sessions + RX/TX bytes
- Wireless interfaces/clients (legacy wireless + wifiwave2), rates, noise floor, CCQ
- OSPF neighbors (`collect_ospf=true`)
- Transceiver temp/TX/RX power (`collect_optics=true`)

## Testing

```bash
go test ./...
go vet ./...
```

Fixtures under `pkg/mikrotik/testdata/` cover ROS6 and ROS7 sample payloads.

## Ansible / release notes

- Release assets include per-binary SHA256 in `checksums.txt` and an SPDX SBOM.
- Pin Ansible downloads to a release tag **and** verify `checksums.txt`.
- Dashboard/alert changes live in ansible-infrastructure; this repo documents the metric contract in [docs/ALERTS.md](docs/ALERTS.md).

## Additional resources

- [Grafana Dashboard](./resources/ros-grafana.json)
- [SystemD Service](./resources/ros-exporter.service)
- [Alert contract](./docs/ALERTS.md)

## License

[MIT License](LICENSE)
