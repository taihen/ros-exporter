# Alert and dashboard contract (for ansible-infrastructure)

This file is the exporter-side contract for Grafana/Prometheus alerts. Implement rules in ansible-infrastructure; do not rely only on `mikrotik_up`.

## Availability (existing)

```yaml
# Device reachable via exporter connection
expr: up{job="ros_exporter"} * mikrotik_up == 0
```

Equivalent with the new alias:

```yaml
expr: up{job="ros_exporter"} * mikrotik_connected == 0
```

## Partial scrape failure (required after upgrade)

```yaml
expr: mikrotik_last_scrape_error == 1
# or
expr: mikrotik_scrape_success == 0 and mikrotik_connected == 1
```

Per collector:

```yaml
expr: mikrotik_collector_error{collector=~"system|interfaces|health|wireless"} == 1
```

## Health / capacity

```yaml
expr: mikrotik_system_cpu_load_percent > 90
expr: mikrotik_system_memory_usage_bytes / mikrotik_system_memory_total_bytes > 0.9
expr: mikrotik_system_storage_free_bytes / mikrotik_system_storage_total_bytes < 0.1
expr: mikrotik_health_temperature_celsius > 75
expr: mikrotik_health_board_temperature_celsius > 75
```

## Interfaces

```yaml
expr: rate(mikrotik_interface_receive_errors_total[5m]) > 0
expr: mikrotik_interface_admin_up == 1 and mikrotik_interface_info == 0
```

## Wireless (when collect_wireless=true)

```yaml
expr: mikrotik_wireless_client_signal_strength_dbm < -80
expr: mikrotik_wireless_interface_noise_floor_dbm > -70
```

## BGP (when collect_bgp=true)

```yaml
expr: mikrotik_bgp_peer_state == 0
```

Use label `routing_instance` (not `instance`) on BGP info metrics.

## Dashboard checklist

1. Pin Overview dashboard to a release commit/checksum; do not pull a mutable GitHub tag blindly.
2. Replace hardcoded Grafana datasource UID (`cehfr9pxtob9cc`) with a variable or `${datasource}`.
3. Remove `uptime_text` from any panel label matchers.
4. Prefer `mikrotik_scrape_success` panels next to `mikrotik_up`.
