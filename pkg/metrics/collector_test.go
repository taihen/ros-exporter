package metrics

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/taihen/ros-exporter/pkg/mikrotik"
)

func TestCollectorDescsNoUptimeText(t *testing.T) {
	client := mikrotik.NewClient("127.0.0.1", "u", "p", mikrotik.DefaultTimeout)
	c := NewMikrotikCollectorWithOptions(client, CollectorOptions{
		CollectBGP:      true,
		CollectPPP:      true,
		CollectWireless: true,
		CollectOSPF:     true,
		CollectOptics:   true,
		Version:         "test",
		Commit:          "abc",
	})

	ch := make(chan *prometheus.Desc, 256)
	go func() {
		c.Describe(ch)
		close(ch)
	}()

	for desc := range ch {
		s := desc.String()
		if strings.Contains(s, "uptime_text") {
			t.Fatalf("found uptime_text label in desc: %s", s)
		}
		if strings.Contains(s, "variableLabels: [instance]") && strings.Contains(s, "bgp_peer") {
			t.Fatalf("bgp still using instance label: %s", s)
		}
	}
}

func TestCollectorStatusMetricsOnConnectFailure(t *testing.T) {
	client := mikrotik.NewClient("127.0.0.1:1", "u", "p", mikrotik.DefaultTimeout)
	c := NewMikrotikCollector(client, false, false, false)

	ch := make(chan prometheus.Metric, 64)
	go func() {
		c.Collect(ch)
		close(ch)
	}()

	got := map[string]float64{}
	for m := range ch {
		var dtoMetric dto.Metric
		if err := m.Write(&dtoMetric); err != nil {
			t.Fatal(err)
		}
		desc := m.Desc().String()
		if strings.Contains(desc, "fqName: \"mikrotik_up\"") || strings.Contains(desc, `fqName: "mikrotik_up"`) {
			got["up"] = dtoMetric.GetGauge().GetValue()
		}
		if strings.Contains(desc, "mikrotik_connected") {
			got["connected"] = dtoMetric.GetGauge().GetValue()
		}
		if strings.Contains(desc, "mikrotik_scrape_success") {
			got["scrape_success"] = dtoMetric.GetGauge().GetValue()
		}
		if strings.Contains(desc, "mikrotik_last_scrape_error") {
			got["last_scrape_error"] = dtoMetric.GetGauge().GetValue()
		}
	}

	if got["up"] != 0 || got["connected"] != 0 {
		t.Fatalf("expected connect failure metrics, got %#v", got)
	}
	if got["scrape_success"] != 0 || got["last_scrape_error"] != 1 {
		t.Fatalf("expected scrape failure, got %#v", got)
	}
}

func TestCollectorCancelledContextReturnsFast(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var accepted []net.Conn
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			accepted = append(accepted, conn)
			mu.Unlock()
		}
	}()
	defer func() {
		_ = ln.Close()
		mu.Lock()
		defer mu.Unlock()
		for _, conn := range accepted {
			_ = conn.Close()
		}
	}()

	// Already cancelled. A collector that ignores ScrapeContext dials the
	// listener and blocks in login; one that honors it returns immediately.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := mikrotik.NewClient(ln.Addr().String(), "u", "p", 30*time.Second)
	c := NewMikrotikCollectorWithOptions(client, CollectorOptions{ScrapeContext: ctx})

	ch := make(chan prometheus.Metric, 64)
	done := make(chan struct{})
	go func() {
		c.Collect(ch)
		close(ch)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("collect did not return within 1s")
	}
	got := map[string]float64{}
	for m := range ch {
		var dtoMetric dto.Metric
		if err := m.Write(&dtoMetric); err != nil {
			t.Fatal(err)
		}
		desc := m.Desc().String()
		if strings.Contains(desc, `fqName: "mikrotik_up"`) {
			got["up"] = dtoMetric.GetGauge().GetValue()
		}
		if strings.Contains(desc, "mikrotik_scrape_success") {
			got["scrape_success"] = dtoMetric.GetGauge().GetValue()
		}
	}
	if got["up"] != 0 || got["scrape_success"] != 0 {
		t.Fatalf("metrics=%#v", got)
	}
}

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

func TestEmitCollectorStatusOmitsNeverStarted(t *testing.T) {
	client := mikrotik.NewClient("127.0.0.1", "u", "p", mikrotik.DefaultTimeout)
	c := NewMikrotikCollector(client, false, false, false)
	ch := make(chan prometheus.Metric, 16)
	s := &scrapeState{
		ch:        ch,
		addr:      "t",
		errors:    map[string]bool{},
		supported: map[string]float64{"system": 1},
	}
	s.emitCollectorStatus(c)
	close(ch)
	seen := map[string]bool{}
	for m := range ch {
		var dtoMetric dto.Metric
		if err := m.Write(&dtoMetric); err != nil {
			t.Fatal(err)
		}
		for _, lp := range dtoMetric.GetLabel() {
			if lp.GetName() == "collector" {
				seen[lp.GetValue()] = true
			}
		}
	}
	if !seen["system"] {
		t.Fatal("expected system collector status")
	}
	if seen["interfaces"] || seen["health"] {
		t.Fatalf("never-started collectors should be omitted, got %#v", seen)
	}
}
