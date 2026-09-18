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
	iface := parseWirelessMonitor("wlan1", "", false, mon)
	if iface.TxRate < 70e6 {
		t.Fatalf("expected Mbps parse, got %v", iface.TxRate)
	}

	var stationMon map[string]string
	if err := json.Unmarshal(root["wireless_legacy_station_monitor"], &stationMon); err != nil {
		t.Fatal(err)
	}
	station := parseWirelessMonitor("wlan1", "station-bridge", true, stationMon)
	if station.Role != "station" || !station.Connected || station.Frequency != 5825 || station.ChannelWidthMHz != 20 {
		t.Fatalf("station fixture: %+v", station)
	}
	if !station.HasSNR || station.SNR != 37 || station.BSSID == "" {
		t.Fatalf("station RF: %+v", station)
	}

	var apMon map[string]string
	if err := json.Unmarshal(root["wireless_legacy_ap_monitor"], &apMon); err != nil {
		t.Fatal(err)
	}
	ap := parseWirelessMonitor("wlan1", "ap-bridge", true, apMon)
	if ap.Role != "ap" || !ap.Connected || ap.ChannelWidthMHz != 40 {
		t.Fatalf("ap fixture: %+v", ap)
	}

	var reg map[string]string
	if err := json.Unmarshal(root["wireless_legacy_registration"], &reg); err != nil {
		t.Fatal(err)
	}
	noise, hasNoise := parseNoiseFloor(reg["noise-floor"])
	snr, hasSNR := parseNoiseFloor(reg["signal-to-noise"])
	rxCCQ, hasRxCCQ := parseOptionalInt(reg["rx-ccq"])
	if !hasNoise || noise != -94 || !hasSNR || snr != 32 || !hasRxCCQ || rxCCQ != 85 {
		t.Fatalf("reg parse noise=%v snr=%v rxccq=%v", noise, snr, rxCCQ)
	}
	sig := parseSignalDBM(firstNonEmpty(reg, "signal-strength", "signal"))
	if sig != -62 {
		t.Fatalf("reg signal=%d", sig)
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
	iface := parseWirelessMonitor("wifi1", "", false, wifi)
	if iface.TxRate != 866.6e6 {
		t.Fatalf("wifi rate=%v", iface.TxRate)
	}

	var stationMon map[string]string
	if err := json.Unmarshal(root["wifi_station_monitor"], &stationMon); err != nil {
		t.Fatal(err)
	}
	station := parseWirelessMonitor("wifi1", "station-bridge", true, stationMon)
	if station.Role != "station" || !station.Connected || station.ChannelWidthMHz != 80 {
		t.Fatalf("wifi station: %+v", station)
	}
	if station.BSSID != "11:22:33:44:55:66" || station.SignalStrength != -61 {
		t.Fatalf("wifi station RF: %+v", station)
	}
	if station.TxRate != 866600000 {
		t.Fatalf("wifi integer rate=%v", station.TxRate)
	}

	var scanningMon map[string]string
	if err := json.Unmarshal(root["wifi_station_scanning"], &scanningMon); err != nil {
		t.Fatal(err)
	}
	scanning := parseWirelessMonitor("wifi1", "station", true, scanningMon)
	if scanning.Connected || scanning.ChannelWidthMHz != 20 {
		t.Fatalf("scanning should be down with 20MHz: %+v", scanning)
	}

	var apMon map[string]string
	if err := json.Unmarshal(root["wifi_ap_monitor"], &apMon); err != nil {
		t.Fatal(err)
	}
	ap := parseWirelessMonitor("wifi1", "ap", true, apMon)
	if ap.Role != "ap" || !ap.Connected || ap.ChannelWidthMHz != 80 {
		t.Fatalf("wifi ap: %+v", ap)
	}

	var reg map[string]string
	if err := json.Unmarshal(root["wifi_registration"], &reg); err != nil {
		t.Fatal(err)
	}
	sig := parseSignalDBM(firstNonEmpty(reg, "signal-strength", "signal"))
	txRate, err := ParseRateBps(reg["tx-rate"])
	if err != nil || sig != -64 || txRate != 433300000 {
		t.Fatalf("wifi reg sig=%d rate=%v err=%v", sig, txRate, err)
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
