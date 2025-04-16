# MikroTik RouterOS Prometheus Exporter (ros-exporter)

> [!WARNING]
> This exporter is under development, do not use until first version has bee released.

A Prometheus exporter for MikroTik RouterOS devices.

This exporter connects to MikroTik routers using the native API (via the `go-routeros/routeros` library) and exposes metrics for monitoring with Prometheus.

## Features

- Collects metrics for:
  - System Resources (CPU, Memory, Uptime, Board Info) - **Always Enabled**
  - Interface Statistics (Traffic, Packets, Errors, Drops) - **Always Enabled**
    - PPP and PPPoE interfaces are automatically excluded from interface statistics
  - BGP Peer Status (State, Prefixes, Updates, Uptime) - **Optional**
  - Active PPP Users (Count, User Info, Uptime) - **Optional**
- Exposes metrics via HTTP on `/metrics` (default port 9483)
- Target router and optional metric collection are specified via scrape configuration in Prometheus (using URL parameters)
- Exporter health metrics (`mikrotik_up`, `mikrotik_scrape_duration_seconds`, `mikrotik_last_scrape_error`) - **Always Enabled**
- Configurable listen address, metrics path, and scrape timeout via command-line flags
- Graceful shutdown handling

## Requirements

- Go (version 1.18+ recommended)
- MikroTik RouterOS v6.48 or later (tested with both v6.x and v7.x API features)
- A dedicated read-only user on the MikroTik router for the exporter

## RouterOS Version Compatibility

- **RouterOS 7.x**:
  - Uses the newer API path (`/routing/bgp/peer/print`) for BGP data collection
  - BGP peer uptime is available in the `uptime` field
  - Standard field names are used for BGP metrics and interface statistics

- **RouterOS 6.x**:
  - Uses the older API path (`/ip/bgp/peer/print`) for BGP data collection
  - In RouterOS 6.48, BGP peer uptime might be in the `established-for` field instead of `uptime`
  - Field names for BGP metrics and interface statistics might be different in RouterOS 6.48
  - The exporter tries multiple possible field names for each metric to find the correct one
  - Multiple fallback methods for interface statistics collection:
    - First tries `/interface/monitor-traffic` with all interfaces
    - If that fails, tries `/interface/print stats`
    - If that fails, tries `/interface/ethernet/print stats`
    - If that fails, tries to monitor each interface individually
  - Debug logging is available to identify which fields and methods are being used

The exporter automatically detects the RouterOS version and uses the appropriate API paths and field names for data collection. It also handles differences in field names and formats between RouterOS versions, making it compatible with both RouterOS 6.x and 7.x.

## Getting Started

### Building

```bash
go build -o ros-exporter ./cmd/ros-exporter
```

### Running

```bash
./ros-exporter [flags]
```

**Flags:**

- `-web.listen-address`: Address to listen on (default: `:9483`).
- `-web.telemetry-path`: Path for metrics endpoint (default: `/metrics`).
- `-scrape.timeout`: Timeout for scraping a target router (default: `10s`).

### MikroTik Configuration

Create a read-only user group and user on your MikroTik router:

```mikrotik
/user group add name=prometheus policy=read,api
/user add name=prometheus group=prometheus password=YOUR_STRONG_PASSWORD address=EXPORTER_IP_ADDRESS
```

> [!IMPORTANT]
> Replace `YOUR_STRONG_PASSWORD` with a secure password and EXPORTER_IP_ADDRESS
> with the IP address of the exporter itself.

### Prometheus Configuration

Add the following job to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'mikrotik'
    scrape_interval: 1m
    scrape_timeout: 50s # Should be slightly less than scrape_interval and greater than exporter's scrape.timeout
    metrics_path: /metrics # Or your custom path if using -web.telemetry-path
    static_configs:
      - targets: ['192.168.88.1'] # Replace with your router IPs/hostnames

    relabel_configs:
      - source_labels: [__address__]
        target_label: __param_target
      - source_labels: [__param_target]
        target_label: instance
      - target_label: __address__
        replacement: localhost:9483 # Address of the ros-exporter itself

      # Pass credentials via parameters. User defaults to 'prometheus' if omitted.
      # Consider using Prometheus agent secrets management for production.
      # - target_label: __param_user
      #   replacement: your_custom_user # Uncomment and set if not using default 'prometheus'
      - target_label: __param_password
        replacement: YOUR_STRONG_PASSWORD # Replace with the password set on the router

      # Optionally specify the API port (defaults to 8728 if omitted)
      # - target_label: __param_port
      #   replacement: 8729 # Example: Use non-standard port

      # Optionally enable BGP and/or PPP metrics per target
      # Add these blocks if you want to enable them for this job. Default is false.
      # - target_label: __param_collect_bgp
      #   replacement: true
      # - target_label: __param_collect_ppp
      #   replacement: true
```

**Explanation:**

1. `targets`: List your MikroTik router IPs/hostnames here.
2. `relabel_configs`:
    - The first two rules take the router address from `targets` and set it as the `target` URL parameter (`__param_target`) for the exporter and also as the `instance` label in Prometheus.
    - The third rule rewrites the scrape address (`__address__`) to point to where your `ros-exporter` is running (e.g., `localhost:9483`).
    - The `__param_user` rule adds the username. If omitted, the exporter defaults to `prometheus`.
    - The `__param_password` rule adds the password. **Warning:** Storing passwords directly in `prometheus.yml` is insecure. Use appropriate secret management in production.
    - The optional `__param_port` rule allows specifying a non-default API port (default is 8728).
    - The optional `__param_collect_bgp` and `__param_collect_ppp` rules enable collection of BGP and PPP metrics respectively (default is `false`).

Alternatively, using the `params` block (Prometheus v2.28+):

```yaml
scrape_configs:
  - job_name: 'mikrotik-params'
    scrape_interval: 1m
    scrape_timeout: 50s
    metrics_path: /metrics # Or your custom path
    params:
      # user: ['your_custom_user'] # Optional: Defaults to 'prometheus' if omitted
      password: ['YOUR_STRONG_PASSWORD'] # Required
      # port: ['8729'] # Optional: Defaults to 8728 if omitted
      collect_bgp: ['true'] # Optional: Enable BGP collection
      collect_ppp: ['true'] # Optional: Enable PPP collection
    static_configs:
      - targets: ['192.168.88.1', '10.0.0.1'] # Router IPs/hostnames

    relabel_configs:
      - source_labels: [__address__]
        target_label: __param_target # Set target parameter from address
      - source_labels: [__param_target]
        target_label: instance # Set instance label from target parameter
      - target_label: __address__
        replacement: localhost:9483 # Address of the ros-exporter
```

## Metrics Exposed

> [!NOTE]
> WIP

List the key metrics exposed:

- `mikrotik_up`
- `mikrotik_scrape_duration_seconds`
- `mikrotik_last_scrape_error`
- System metrics (e.g., `mikrotik_system_cpu_load_percent`, `mikrotik_system_memory_usage_bytes`)
- Interface metrics (e.g., `mikrotik_interface_receive_bytes_total`)
- BGP metrics (e.g., `mikrotik_bgp_peer_state`)
- PPP metrics (e.g., `mikrotik_ppp_active_users_count`)

## Additional resources

- [Grafana Dashboard](./resources/ros-grafana.json)
- [SystemD Service](./resources/ros-exporter.service)

## TODO

#### Daemon experience

- Implement configuration via YAML/env vars
- Add better connection handling/pooling

#### Project / developer experience

- Add unit/integration tests
- Add developer example how to implement metrics collection

#### Collectors

- power feed current and voltage
- fans speed
- cpu and ambient temperature
- transceivers signal and temperature
- ospf
- wireless interfaces client count, tx and rx rate, ccq, noice floor and frequency

## License

[MIT License](LICENSE)
