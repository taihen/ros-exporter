package mikrotik

import (
	"fmt"
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
	iface := parseWirelessMonitor("wlan1", "", false, map[string]string{
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

func TestWirelessRole(t *testing.T) {
	tests := []struct {
		mode string
		want string
	}{
		{"ap-bridge", "ap"},
		{"ap", "ap"},
		{"bridge", "ap"},
		{"wds-slave", "ap"},
		{"station", "station"},
		{"station-bridge", "station"},
		{"station-pseudobridge-clone", "station"},
		{"", "unknown"},
		{"nstream", "unknown"},
	}
	for _, tc := range tests {
		if got := wirelessRole(tc.mode); got != tc.want {
			t.Fatalf("wirelessRole(%q)=%q want %q", tc.mode, got, tc.want)
		}
	}
}

func TestParseChannel(t *testing.T) {
	tests := []struct {
		in          string
		freq, width int
		label       string
	}{
		{"5825/20/ac(10dBm)", 5825, 20, "20"},
		{"5180/40-Ce/ac", 5180, 40, "40"},
		{"5180/20-Ceee/ac", 5180, 80, "80"},
		{"5500/80/ac", 5500, 80, "80"},
		{"5260/ax/Ceee", 5260, 80, "80"},
		{"5600/ac/eCee", 5600, 80, "80"},
		{"2462/ax/eC", 2462, 40, "40"},
		{"2447/ax", 2447, 20, "20"},
		{"2412", 2412, 0, ""},
		{"", 0, 0, ""},
	}
	for _, tc := range tests {
		freq, width, label := parseChannel(tc.in)
		if freq != tc.freq || width != tc.width || label != tc.label {
			t.Fatalf("parseChannel(%q)=(%d,%d,%q) want (%d,%d,%q)",
				tc.in, freq, width, label, tc.freq, tc.width, tc.label)
		}
	}
}

func TestParseWirelessMonitorStation(t *testing.T) {
	iface := parseWirelessMonitor("wlan1", "station-bridge", true, map[string]string{
		"ssid":            "backhaul",
		"channel":         "5825/20/ac(10dBm)",
		"signal-strength": "-58@MCS7",
		"noise-floor":     "-95",
		"signal-to-noise": "37",
		"status":          "connected-to-ess",
		"bssid":           "AA:BB:CC:DD:EE:01",
		"overall-tx-ccq":  "94",
		"rx-ccq":          "92",
		"tx-rate":         "150Mbps",
		"rx-rate":         "135Mbps",
	})
	if iface.Role != "station" || !iface.Connected || !iface.Running {
		t.Fatalf("role/connected: %+v", iface)
	}
	if iface.Frequency != 5825 || iface.ChannelWidthMHz != 20 || iface.ChannelWidth != "20" {
		t.Fatalf("channel: %+v", iface)
	}
	if !iface.HasSNR || iface.SNR != 37 || !iface.HasNoiseFloor || iface.NoiseFloor != -95 {
		t.Fatalf("snr/noise: %+v", iface)
	}
	if !iface.HasTxCCQ || iface.TxCCQ != 94 || !iface.HasRxCCQ || iface.RxCCQ != 92 {
		t.Fatalf("ccq: %+v", iface)
	}
	if iface.BSSID != "AA:BB:CC:DD:EE:01" {
		t.Fatalf("bssid: %q", iface.BSSID)
	}
}

func TestParseWirelessMonitorAP(t *testing.T) {
	iface := parseWirelessMonitor("wlan1", "ap-bridge", true, map[string]string{
		"ssid":           "backhaul",
		"channel":        "5825/40-Ce/ac(20dBm)",
		"noise-floor":    "-90",
		"status":         "running-ap",
		"overall-tx-ccq": "88",
	})
	if iface.Role != "ap" || !iface.Connected {
		t.Fatalf("ap connected should follow running: %+v", iface)
	}
	if iface.ChannelWidthMHz != 40 {
		t.Fatalf("width=%d", iface.ChannelWidthMHz)
	}
}

func TestParseWirelessClientSNRNotNoise(t *testing.T) {
	// Mimic fetchWirelessClientsPath field mapping without API.
	noise, hasNoise := parseNoiseFloor("-94")
	snr, hasSNR := parseNoiseFloor("32")
	if !hasNoise || noise != -94 || !hasSNR || snr != 32 {
		t.Fatalf("noise=%d/%v snr=%d/%v", noise, hasNoise, snr, hasSNR)
	}
	// Empty noise must not fall back to SNR.
	noise2, hasNoise2 := parseNoiseFloor("")
	if hasNoise2 || noise2 != 0 {
		t.Fatalf("empty noise should be absent: %d %v", noise2, hasNoise2)
	}
}

func TestWirelessConnected(t *testing.T) {
	tests := []struct {
		role, status, bssid string
		running, want       bool
	}{
		{"station", "searching-for-network", "", true, false},
		{"station", "scanning", "AA:BB:CC:DD:EE:01", true, false},
		{"station", "searching-for-network", "AA:BB:CC:DD:EE:01", true, false},
		{"station", "authorized", "", false, true},
		{"station", "connected-to-ess", "AA:BB:CC:DD:EE:01", true, true},
		{"station", "", "", true, true},
		{"station", "", "", false, false},
		{"ap", "", "", true, true},
		{"ap", "running-ap", "", false, false},
		{"unknown", "connected", "", false, true},
		{"unknown", "", "", true, true},
		{"unknown", "searching-for-network", "", false, false},
	}
	for _, tc := range tests {
		if got := wirelessConnected(tc.role, tc.running, tc.status, tc.bssid); got != tc.want {
			t.Fatalf("wirelessConnected(%q, %v, %q, %q)=%v want %v",
				tc.role, tc.running, tc.status, tc.bssid, got, tc.want)
		}
	}
}

func TestMergeWirelessSources(t *testing.T) {
	ok := func(items []string) func() ([]string, error) {
		return func() ([]string, error) { return items, nil }
	}
	unsupported := func() ([]string, error) {
		return nil, fmt.Errorf("no such command or directory (wireless)")
	}
	hard := func() ([]string, error) {
		return nil, fmt.Errorf("connection timed out")
	}

	t.Run("ok+unsupported", func(t *testing.T) {
		got, err := mergeWirelessSources("skip", ok([]string{"a"}), unsupported)
		if err != nil || len(got) != 1 || got[0] != "a" {
			t.Fatalf("got=%v err=%v", got, err)
		}
	})
	t.Run("unsupported+ok", func(t *testing.T) {
		got, err := mergeWirelessSources("skip", unsupported, ok([]string{"b"}))
		if err != nil || len(got) != 1 || got[0] != "b" {
			t.Fatalf("got=%v err=%v", got, err)
		}
	})
	t.Run("ok+hard keeps first", func(t *testing.T) {
		got, err := mergeWirelessSources("skip", ok([]string{"a", "b"}), hard)
		if err != nil || len(got) != 2 {
			t.Fatalf("got=%v err=%v", got, err)
		}
	})
	t.Run("hard first fails", func(t *testing.T) {
		got, err := mergeWirelessSources("skip", hard, ok([]string{"a"}))
		if err == nil || got != nil {
			t.Fatalf("got=%v err=%v", got, err)
		}
	})
	t.Run("both unsupported", func(t *testing.T) {
		got, err := mergeWirelessSources("skip", unsupported, unsupported)
		if err != nil || got != nil {
			t.Fatalf("got=%v err=%v", got, err)
		}
	})
	t.Run("merge both ok", func(t *testing.T) {
		got, err := mergeWirelessSources("skip", ok([]string{"a"}), ok([]string{"b"}))
		if err != nil || len(got) != 2 || got[0] != "a" || got[1] != "b" {
			t.Fatalf("got=%v err=%v", got, err)
		}
	})
}
