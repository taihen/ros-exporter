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
# Station bridge link down (prefer over empty registration-table)
expr: mikrotik_wireless_interface_connected{role="station"} == 0

# Weak signal (AP peers or station interface via join — role is not on the signal metric)
expr: mikrotik_wireless_client_signal_strength_dbm < -80
expr: mikrotik_wireless_interface_signal_strength_dbm < -80 and on(instance, name) mikrotik_wireless_interface_info{role="station"}

# RF environment
expr: mikrotik_wireless_interface_noise_floor_dbm > -70
expr: mikrotik_wireless_interface_signal_to_noise_db < 20
expr: mikrotik_wireless_client_signal_to_noise_db < 20

# Link quality (legacy wireless CCQ; often absent on wifiwave2)
expr: mikrotik_wireless_interface_transmit_ccq_percent < 70
expr: mikrotik_wireless_client_transmit_ccq_percent < 70

# Optional: AP with no peers — can false-positive on idle PtMP sectors
# expr: mikrotik_wireless_interface_active_clients_count == 0 and on(instance, interface) label_replace(mikrotik_wireless_interface_info{role="ap"}, "interface", "$1", "name", "(.*)")
```

Info metric labels: `name`, `ssid`, `frequency`, `mode`, `role`, `bssid` (series ID change). Use `role="ap"|"station"` on `info` / `connected`, or join RF gauges with `info` on `(instance, name)`. Channel width is a gauge only (`channel_width_mhz`), not an info label.

`active_clients_count` is emitted for every discovered wireless interface (including `0` when idle). `signal_to_noise_db` is separate from `noise_floor_dbm` (previously SNR could appear under noise floor). CCQ/SNR and `associated` are omitted for non-station interfaces / when the device does not report them. If `mode` is missing, `role` is `unknown` — use `mikrotik_wireless_interface_connected{role="unknown"} == 0` (not bare `associated == 0`).

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
