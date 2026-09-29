# Scrape cancel skip Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stop starting further collectors once the scrape context is done, and stop recording cancel/deadline as `mikrotik_collector_error` (including mid-flight socket close).

**Architecture:** Extract `runCollectorSteps(ctx, steps)` for skip-on-cancel. Prefer `ctx.Err()` in `Client.run` when the context is done after a failed command. Teach `markErr` to ignore cancel/deadline (including wrapped). Seed `supported` only when a collector starts so never-started collectors emit no healthy `collector_error=0`. Clarify `docs/ALERTS.md`.

**Tech Stack:** Go, Prometheus client_golang, existing `pkg/metrics` and `pkg/mikrotik` tests.

**Spec:** `docs/superpowers/specs/2026-09-29-scrape-cancel-skip-design.md`

---

## File map

| File | Role |
|------|------|
| `pkg/metrics/collector.go` | `runCollectorSteps`; wire `Collect`; `markErr` ignore cancel; seed `supported` at collector start |
| `pkg/metrics/collector_test.go` | Helper + `markErr` tests |
| `pkg/mikrotik/client.go` | Prefer `ctx.Err()` when context done after `fn` fails |
| `pkg/mikrotik/client_test.go` | Test remapped cancel after failed command |
| `docs/ALERTS.md` | Scrape-level vs per-collector clarification |

---

### Task 1: Failing tests for skip helper and markErr

**Files:**
- Modify: `pkg/metrics/collector_test.go`

- [ ] **Step 1: Write failing tests**

Append to `pkg/metrics/collector_test.go` (add `"errors"` and `"fmt"` to imports):

```go
func TestRunCollectorStepsSkipsAfterCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var called []string
	steps := []func(){
		func() { called = append(called, "a") },
		func() {
			called = append(called, "b")
			cancel()
		},
		func() { called = append(called, "c") },
	}
	runCollectorSteps(ctx, steps)
	if got := strings.Join(called, ","); got != "a,b" {
		t.Fatalf("called=%q want a,b", got)
	}
}

func TestRunCollectorStepsRunsAllWhenLive(t *testing.T) {
	var called []string
	steps := []func(){
		func() { called = append(called, "a") },
		func() { called = append(called, "b") },
		func() { called = append(called, "c") },
	}
	runCollectorSteps(context.Background(), steps)
	if got := strings.Join(called, ","); got != "a,b,c" {
		t.Fatalf("called=%q want a,b,c", got)
	}
}

func TestRunCollectorStepsSkipsAllWhenAlreadyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var called []string
	runCollectorSteps(ctx, []func(){
		func() { called = append(called, "a") },
	})
	if len(called) != 0 {
		t.Fatalf("called=%v want none", called)
	}
}

func TestMarkErrIgnoresCancelAndDeadline(t *testing.T) {
	s := &scrapeState{errors: map[string]bool{}, addr: "t"}
	s.markErr("bgp", context.Canceled)
	s.markErr("ppp", context.DeadlineExceeded)
	s.markErr("ospf", fmt.Errorf("wrap: %w", context.Canceled))
	s.markErr("optics", fmt.Errorf("wrap: %w", context.DeadlineExceeded))
	if len(s.errors) != 0 {
		t.Fatalf("errors=%v want empty", s.errors)
	}
	s.markErr("system", errors.New("boom"))
	if !s.errors["system"] {
		t.Fatal("expected system error recorded")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/metrics/ -run 'TestRunCollectorSteps|TestMarkErrIgnoresCancel' -count=1`

Expected: FAIL — `runCollectorSteps` undefined and/or cancel still recorded.

---

### Task 2: Implement helper and markErr; seed supported on start

**Files:**
- Modify: `pkg/metrics/collector.go`

- [ ] **Step 1: Add `runCollectorSteps`**

```go
// runCollectorSteps runs steps in order and stops when ctx is done so
// later collectors are not started after cancel or deadline.
func runCollectorSteps(ctx context.Context, steps []func()) {
	for _, step := range steps {
		if ctx.Err() != nil {
			return
		}
		step()
	}
}
```

- [ ] **Step 2: Update `markErr`**

```go
func (s *scrapeState) markErr(name string, err error) {
	if err == nil {
		return
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return
	}
	log.Printf("ERROR: collector %s on %s: %v", name, s.addr, err)
	s.errors[name] = true
}
```

Add `"errors"` to imports in `collector.go`.

- [ ] **Step 3: Stop pre-seeding core supported; set on collector start**

In `Collect`, change scrapeState init so `supported` starts empty:

```go
	scrape := &scrapeState{
		ch:        ch,
		addr:      c.client.Address,
		errors:    map[string]bool{},
		supported: map[string]float64{},
	}
```

At the start of each of these functions, before the API call:

- `collectSystem`: `s.supported["system"] = 1`
- `collectInterfaces`: `s.supported["interfaces"] = 1`
- `collectHealth`: `s.supported["health"] = 1` (existing `supported["health"]=0` when unsupported stays)

`collectRouterboard` and optional collectors already set `supported` when they start.

- [ ] **Step 4: Re-run the new metrics tests**

Run: `go test ./pkg/metrics/ -run 'TestRunCollectorSteps|TestMarkErrIgnoresCancel' -count=1`

Expected: PASS

---

### Task 3: Prefer ctx.Err() in Client.run after failed command

**Files:**
- Modify: `pkg/mikrotik/client.go`
- Modify: `pkg/mikrotik/client_test.go`

- [ ] **Step 1: Write failing test for preferContextErr**

Append to `pkg/mikrotik/client_test.go` (add `"errors"` if missing):

```go
func TestPreferContextErr(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got := preferContextErr(ctx, errors.New("connection reset"))
	if !errors.Is(got, context.Canceled) {
		t.Fatalf("got=%v", got)
	}
	live := preferContextErr(context.Background(), errors.New("boom"))
	if live.Error() != "boom" {
		t.Fatalf("got=%v", live)
	}
	if preferContextErr(context.Background(), nil) != nil {
		t.Fatal("nil should stay nil")
	}
}
```

- [ ] **Step 2: Implement preferContextErr and use it in run**

In `pkg/mikrotik/client.go`:

```go
func preferContextErr(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return err
}
```

Change the failure branch in `run`:

```go
	reply, err := fn(cli)
	if err != nil {
		log.Printf("Error running %s on %s: %v", kind, c.Address, err)
		if ctx.Err() != nil {
			c.Close()
		}
		return nil, preferContextErr(ctx, err)
	}
```

- [ ] **Step 3: Run mikrotik tests**

Run: `go test ./pkg/mikrotik/ -run 'TestPreferContextErr|TestBeginScrape|TestConnectContext|TestRun' -count=1`

Expected: PASS

---

### Task 4: Wire Collect through runCollectorSteps

**Files:**
- Modify: `pkg/metrics/collector.go` (Collect body after successful connect)

- [ ] **Step 1: Replace sequential collector calls**

```go
	runCollectorSteps(ctx, []func(){
		func() { c.collectSystem(scrape) },
		func() { c.collectRouterboard(scrape) },
		func() { c.collectInterfaces(scrape) },
		func() { c.collectHealth(scrape) },
		func() {
			if c.collectBGP {
				c.collectBGPPeers(scrape)
			}
		},
		func() {
			if c.collectPPP {
				c.collectPPPUsers(scrape)
			}
		},
		func() {
			if c.collectWireless {
				c.collectWirelessMetrics(scrape)
			}
		},
		func() {
			if c.collectOSPF {
				c.collectOSPFNeighbors(scrape)
			}
		},
		func() {
			if c.collectOptics {
				c.collectOpticsMetrics(scrape)
			}
		},
	})
```

Leave `hasError := len(scrape.errors) > 0 || ctx.Err() != nil` unchanged.

- [ ] **Step 2: Run full tests**

Run: `go test ./pkg/metrics/ ./pkg/mikrotik/ -count=1` then `go test ./... -count=1`

Expected: PASS

---

### Task 5: Alert contract note

**Files:**
- Modify: `docs/ALERTS.md`

- [ ] **Step 1: Clarify per-collector vs scrape timeout**

Under the "Partial scrape failure" section, after the per-collector `expr` block, add:

```markdown
Timeout or cancel after connect is scrape-level (`mikrotik_scrape_success == 0`). Later collectors are skipped and may omit `mikrotik_collector_error` series for that scrape; absence means skipped, not healthy. Do not expect every collector to show `mikrotik_collector_error` on budget expiry.
```

- [ ] **Step 2: Final verification**

Run: `go test ./... -count=1`

Expected: PASS

---

## Spec coverage check

| Spec requirement | Task |
|------------------|------|
| Skip collectors when ctx done | Task 2 helper + Task 4 wire |
| No collector_error for cancel (incl. mid-flight I/O) | Task 2 markErr + Task 3 preferContextErr |
| Never-started collectors do not emit healthy error=0 | Task 2 supported seeding |
| scrape_success=0 via ctx.Err() | Unchanged Collect tail |
| Real errors still marked | Task 1 markErr test |
| ALERTS.md | Task 5 |

## Placeholder scan

No TBD. Preferred `preferContextErr` path is concrete. Names match across tasks.
