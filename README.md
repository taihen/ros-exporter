# ros-exporter

Prometheus exporter for MikroTik RouterOS. It scrapes devices over the native API and exposes per-router metrics you can alert and graph on.

Use it when you already run Prometheus and need MikroTik CPU, memory, interfaces, and health (plus optional BGP, PPP, wireless, OSPF, or optics) without relying on SNMP.

Default listen address: `:9483`. Requires RouterOS **v6.48+** and a read-only API user on each device.

[![Test](https://github.com/taihen/ros-exporter/actions/workflows/test.yml/badge.svg)](https://github.com/taihen/ros-exporter/actions/workflows/test.yml)
[![Release](https://github.com/taihen/ros-exporter/actions/workflows/release.yml/badge.svg)](https://github.com/taihen/ros-exporter/actions/workflows/release.yml)

## Quick start

1. Download a release binary from [Releases](https://github.com/taihen/ros-exporter/releases) (verify `checksums.txt` if you pin installs).
2. Create a credentials file. Target keys must match the Prometheus `target` value exactly:

```bash
mkdir -p /etc/ros-exporter
cat >/etc/ros-exporter/targets.json <<'EOF'
{
  "default_user": "prometheus",
  "targets": {
    "192.168.88.1": {
      "password": "YOUR_STRONG_PASSWORD"
    }
  }
}
EOF
chmod 600 /etc/ros-exporter/targets.json
```

3. Run the exporter with that file:

```bash
./ros-exporter -web.listen-address=:9483 -config.file=/etc/ros-exporter/targets.json
```

4. Check liveness: `curl -sS http://localhost:9483/-/healthy`
5. Create the MikroTik API user (below), then point Prometheus at the exporter.

## MikroTik API user

Create a dedicated read-only user and allow API only from the exporter host:

```mikrotik
/user group add name=prometheus policy=read,api
/user add name=prometheus group=prometheus password=YOUR_STRONG_PASSWORD address=EXPORTER_IP_ADDRESS
```

Enable the API service (`/ip service`) for the exporter IP.

## Credentials config

RouterOS passwords live in a local JSON file (`-config.file`), not in Prometheus scrape URLs. The file is also an allowlist: scrapes for targets missing from the map return HTTP 403.

Example ([resources/targets.example.json](./resources/targets.example.json)):

```json
{
  "default_user": "prometheus",
  "targets": {
    "192.168.88.1": {
      "user": "prometheus",
      "password": "YOUR_STRONG_PASSWORD",
      "port": "8728"
    },
    "core-rtr.example.com": {
      "password": "OTHER_PASSWORD"
    }
  }
}
```

- `default_user` is optional (default `prometheus`).
- Per-target `user` is optional (falls back to `default_user`).
- Per-target `password` is required.
- Per-target `port` is optional; a `port` query param overrides it when set.
- Lookup is an exact string match on `target` (no host/port normalization).

Startup requires `-config.file` or `-web.unsafe-query-auth`. Prefer `-config.file`. If both are set, the file wins for auth and unknown targets are still refused; a query `password` is ignored with a log warning.

`-web.unsafe-query-auth` takes credentials from `user` / `password` query params instead. Those values can appear in Prometheus target URLs and logs.

## Prometheus scrape config

Multi-target pattern: Prometheus talks to the exporter; the exporter talks to each router. Passwords stay in `-config.file` on the exporter host.

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
```

Set Prometheus `scrape_timeout` **greater than** the exporter `-scrape.timeout` (default `10s`).

Enable optional collectors with extra `__param_collect_*` labels (see below).

## Endpoints

| Path | Purpose |
|------|---------|
| `/metrics?target=HOST` | Metrics for one RouterOS device |
| `/-/metrics` | Exporter process metrics |
| `/-/healthy` | Liveness |
| `/-/ready` | Readiness |

Query parameters on `/metrics`: `target` (required), optional `port`, and the `collect_*` flags below. With `-web.unsafe-query-auth`, `user` and `password` are also accepted.

## Flags

| Flag | Default | Meaning |
|------|---------|---------|
| `-web.listen-address` | `:9483` | Listen address |
| `-web.telemetry-path` | `/metrics` | Path for target scrapes |
| `-config.file` | | JSON file with per-target credentials (allowlist) |
| `-web.unsafe-query-auth` | `false` | Allow credentials via URL query params |
| `-scrape.timeout` | `10s` | Budget for one full target scrape |
| `-scrape.max-concurrent` | `25` | Max concurrent RouterOS API connections |
| `-version` | | Print version and exit |

## Optional collectors

Always collected: system (CPU, memory, uptime, storage, board), interfaces, and health (temperature, voltage, current, power, fan).

Turn these on per scrape (default **off**):

| Parameter | Collects |
|-----------|----------|
| `collect_bgp=true` | BGP peers (`routing_instance` label) |
| `collect_ppp=true` | PPP sessions and RX/TX bytes |
| `collect_wireless=true` | Wireless AP and station (legacy + wifiwave2) |
| `collect_ospf=true` | OSPF neighbors |
| `collect_optics=true` | Transceiver temperature and TX/RX power |

Example relabel to enable wireless:

```yaml
- target_label: __param_collect_wireless
  replacement: "true"
```

## Is the scrape healthy?

`mikrotik_up` / `mikrotik_connected` only mean the API login worked. Use scrape success for completeness:

| Metric | Meaning |
|--------|---------|
| `mikrotik_up` / `mikrotik_connected` | API login succeeded |
| `mikrotik_scrape_success` | No collector errors this scrape |
| `mikrotik_last_scrape_error` | `1` if any collector failed |
| `mikrotik_collector_error{collector=...}` | Which collector failed |
| `mikrotik_collector_supported{collector=...}` | Collector enabled / available on the device |
| `mikrotik_scrape_duration_seconds` | How long the scrape took |
| `mikrotik_build_info{version,commit}` | Exporter build |

Alert examples and upgrade notes: [docs/ALERTS.md](docs/ALERTS.md).

## More

- [Credentials example](./resources/targets.example.json)
- [Grafana dashboard](./resources/ros-grafana.json)
- [systemd unit](./resources/ros-exporter.service)
- [Alert contract](./docs/ALERTS.md)
- [MIT License](LICENSE)

### Build from source

Needs Go 1.25+:

```bash
go build -ldflags="-X main.version=dev -X main.commit=$(git rev-parse --short HEAD)" -o ros-exporter ./cmd/ros-exporter
```
