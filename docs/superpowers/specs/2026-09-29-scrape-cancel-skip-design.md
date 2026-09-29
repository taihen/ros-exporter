# Skip remaining collectors when the scrape context is done

## Problem

After scrape-deadline support (#22), `MikrotikCollector.Collect` still runs every enabled collector in sequence even when the scrape context is already cancelled or expired.

Each later collector calls into the client, gets an error immediately (or after a closed socket), and `markErr` records that collector as failed. The result is a cascade of `mikrotik_collector_error{collector=...}=1` values and ERROR logs for collectors that never did meaningful RouterOS work.

`mikrotik_scrape_success` already goes to `0` when `ctx.Err() != nil`. The cascade is wrong signal for `docs/ALERTS.md` per-collector rules, which treat those series as real collector failures.

Connect-time cancel is already handled: failed `ConnectContext` emits connect status and returns before the collector list.

## Decision

When the scrape context is done, stop starting further collectors. Do not mark cancel/deadline as `mikrotik_collector_error`, including for the collector that was running when the socket was closed because the context ended.

Ship this, and not limiter metrics, config reload, or ROS7 BGP, in this change.

## Behavior

After a successful connect, `Collect` runs the same collector sequence as today (system, routerboard, interfaces, health, then optional BGP/PPP/wireless/OSPF/optics).

Before each collector starts:

1. If `ctx.Err() != nil`, do not invoke that collector or any later ones.
2. Collectors that already finished keep their metrics and error state.
3. A collector that was in progress when the context ended may see a library/I/O error after the client closes the TCP socket. `Client.run` must return `ctx.Err()` when `ctx.Err() != nil` after `fn` fails (instead of the raw I/O error), so callers and `markErr` can treat it as cancel/deadline.
4. `markErr` must not record `context.Canceled` or `context.DeadlineExceeded` (including when wrapped with `%w`) into `scrape.errors`. Real API/parse errors still record.

### Collector status series for never-started collectors

Do not pre-seed `supported` with `system` / `interfaces` / `health` at scrape start. Each collector sets `supported[name]=1` (or `0` for unsupported health) when it actually starts, matching routerboard and the optional collectors.

`emitCollectorStatus` then only emits `mikrotik_collector_supported` / `mikrotik_collector_error` for collectors that ran. Cancel-skipped collectors produce no per-collector series for that scrape. Scrape-level gauges still report failure via `mikrotik_scrape_success=0`.

Final scrape status stays:

- `hasError := len(scrape.errors) > 0 || ctx.Err() != nil`
- Connected scrapes that hit the deadline still emit `mikrotik_connected=1` and `mikrotik_scrape_success=0`

Unsupported / soft-fail collectors (for example wireless package missing) keep their existing rules. Only cancel and deadline are excluded from `mikrotik_collector_error`.

## Contract for alerts

- Use `mikrotik_scrape_success == 0` (or `mikrotik_last_scrape_error == 1`) for budget expiry / cancel after connect.
- Use `mikrotik_collector_error` only for collectors that actually failed their RouterOS work.
- Absent `mikrotik_collector_error` for a collector on a timed-out scrape means that collector did not run; do not treat absence as healthy.
- Add clarifying sentences to `docs/ALERTS.md` under the per-collector rule.

## Non-goals

- Limiter gauges or scrape-outcome counters on `/-/metrics`
- Config hot-reload
- ROS7 BGP `/routing/bgp/session`
- Optics N+1 changes
- Health presence-flag redesign
- Changing metric names, auth, or scrape budget math from #22
- Introducing a client interface solely for this change

## Implementation shape

Keep the concrete `*mikrotik.Client`. Extract a small helper that runs an ordered list of steps and returns early when `ctx.Err() != nil`. Change `Client.run` to prefer `ctx.Err()` when the context is done after a failed command. Change `markErr` to ignore cancel and deadline errors. Move core `supported` seeding into each collector start.

## Tests

No live router.

- Helper: cancelled mid-list → later steps not called; already-cancelled → no steps; live → all steps.
- `markErr`: bare and wrapped `Canceled` / `DeadlineExceeded` do not set errors; plain errors do.
- `Client.run`: when `fn` returns a non-context error and `ctx` is already done, the returned error is `ctx.Err()` (and the client is closed as today).
- Existing connect-failure and cancelled-before-connect tests still pass.

## Docs

- `docs/ALERTS.md`: clarify scrape-level timeout/cancel vs per-collector, and that missing collector series on a failed scrape means skipped, not healthy.
- Do not expand README for this behavior.
