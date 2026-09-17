package mikrotik

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func loadFixture(t *testing.T, name string) map[string]json.RawMessage {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(b, &root); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestFixtureROS6SystemAndWireless(t *testing.T) {
	root := loadFixture(t, "ros6_sample.json")

	var res map[string]string
	if err := json.Unmarshal(root["system_resource"], &res); err != nil {
		t.Fatal(err)
	}
	uptime, err := parseMikrotikDuration(res["uptime"])
	if err != nil || uptime <= 0 {
		t.Fatalf("uptime: %v %v", uptime, err)
	}

	var health map[string]string
	if err := json.Unmarshal(root["health_flat"], &health); err != nil {
		t.Fatal(err)
	}
	h := &SystemHealth{Supported: true}
	applyHealthFlat(h, health)
	if h.BoardTemperature != 40 {
		t.Fatalf("board temp=%v", h.BoardTemperature)
	}

	var mon map[string]string
	if err := json.Unmarshal(root["wireless_legacy_monitor"], &mon); err != nil {
		t.Fatal(err)
	}
	iface := parseWirelessMonitor("wlan1", mon)
	if iface.TxRate < 70e6 {
		t.Fatalf("expected Mbps parse, got %v", iface.TxRate)
	}
}

func TestFixtureROS7HealthWifiBGPPPP(t *testing.T) {
	root := loadFixture(t, "ros7_sample.json")

	var rows []map[string]string
	if err := json.Unmarshal(root["health_rows"], &rows); err != nil {
		t.Fatal(err)
	}
	h := &SystemHealth{Supported: true}
	for _, row := range rows {
		applyHealthValue(h, row["name"], row["value"])
	}
	if h.Temperature != 55 || h.BoardTemperature != 42 || h.FanSpeed != 3200 {
		t.Fatalf("health=%+v", h)
	}

	var wifi map[string]string
	if err := json.Unmarshal(root["wifi_monitor"], &wifi); err != nil {
		t.Fatal(err)
	}
	iface := parseWirelessMonitor("wifi1", wifi)
	if iface.TxRate != 866.6e6 {
		t.Fatalf("wifi rate=%v", iface.TxRate)
	}

	var bgp map[string]string
	if err := json.Unmarshal(root["bgp_peer"], &bgp); err != nil {
		t.Fatal(err)
	}
	if bgp["instance"] == "" {
		t.Fatal("missing routing instance")
	}

	var ppp map[string]string
	if err := json.Unmarshal(root["ppp_user"], &ppp); err != nil {
		t.Fatal(err)
	}
	if ppp["bytes-in"] != "111" || ppp["bytes-out"] != "222" {
		t.Fatalf("ppp bytes missing: %v", ppp)
	}
}
