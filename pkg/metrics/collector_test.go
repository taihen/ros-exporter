package metrics

import (
	"strings"
	"testing"

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
