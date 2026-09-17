package mikrotik

import (
	"testing"
	"time"
)

func TestParseMikrotikDuration(t *testing.T) {
	tests := []struct {
		in   string
		want time.Duration
	}{
		{"1w2d3h4m5s", 7*24*time.Hour + 2*24*time.Hour + 3*time.Hour + 4*time.Minute + 5*time.Second},
		{"30s", 30 * time.Second},
		{"1.5s", time.Duration(1.5 * float64(time.Second))},
		{"2h", 2 * time.Hour},
	}
	for _, tc := range tests {
		got, err := parseMikrotikDuration(tc.in)
		if err != nil {
			t.Fatalf("parseMikrotikDuration(%q): %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("parseMikrotikDuration(%q)=%v want %v", tc.in, got, tc.want)
		}
	}
}

func TestParseRateBps(t *testing.T) {
	tests := []struct {
		in   string
		want float64
	}{
		{"54Mbps", 54e6},
		{"1Gbps", 1e9},
		{"150000000", 150000000},
		{"6Mbps-40Mhz/1S/SGI", 6e6},
		{"866.6Mbps", 866.6e6},
	}
	for _, tc := range tests {
		got, err := ParseRateBps(tc.in)
		if err != nil {
			t.Fatalf("ParseRateBps(%q): %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("ParseRateBps(%q)=%v want %v", tc.in, got, tc.want)
		}
	}
}

func TestParseLinkSpeedBps(t *testing.T) {
	tests := []struct {
		in   string
		want uint64
	}{
		{"1Gbps", 1e9},
		{"100Mbps", 100e6},
		{"1G", 1e9},
		{"100M", 100e6},
		{"10M", 10e6},
	}
	for _, tc := range tests {
		got, err := ParseLinkSpeedBps(tc.in)
		if err != nil {
			t.Fatalf("ParseLinkSpeedBps(%q): %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("ParseLinkSpeedBps(%q)=%v want %v", tc.in, got, tc.want)
		}
	}
}

func TestApplyHealthFlatBoardTemperature(t *testing.T) {
	h := &SystemHealth{Supported: true}
	applyHealthFlat(h, map[string]string{
		"temperature":       "45",
		"board-temperature": "38",
		"voltage":           "24.1",
	})
	if h.Temperature != 45 || h.BoardTemperature != 38 || h.Voltage != 24.1 {
		t.Fatalf("unexpected health: %+v", h)
	}
}

func TestApplyHealthValueRowsROS7(t *testing.T) {
	h := &SystemHealth{Supported: true}
	applyHealthValue(h, "cpu-temperature", "52")
	applyHealthValue(h, "board-temperature", "41")
	if h.Temperature != 52 || h.BoardTemperature != 41 {
		t.Fatalf("unexpected health: %+v", h)
	}
}

func TestParseWirelessMonitorRateUnits(t *testing.T) {
	iface := parseWirelessMonitor("wlan1", map[string]string{
		"ssid":            "bridge",
		"frequency":       "5180",
		"signal-strength": "-65",
		"tx-rate":         "54Mbps",
		"rx-rate":         "48Mbps",
		"noise-floor":     "-95",
	})
	if iface.TxRate != 54e6 || iface.RxRate != 48e6 {
		t.Fatalf("rates: tx=%v rx=%v", iface.TxRate, iface.RxRate)
	}
	if !iface.HasNoiseFloor || iface.NoiseFloor != -95 {
		t.Fatalf("noise: %+v", iface)
	}
}
