package mikrotik

import "testing"

func TestSystemResourceHDDSpaceIsBytes(t *testing.T) {
	// Values match a RouterOS 6.48+ /system/resource/print reply: HDD fields are bytes.
	res := systemResourceFromMap(map[string]string{
		"uptime":          "1d",
		"free-memory":     "105840640",
		"total-memory":    "134217728",
		"cpu-load":        "3",
		"free-hdd-space":  "115208192",
		"total-hdd-space": "134217728",
		"board-name":      "RB2011",
		"model":           "RB2011UAS-2HnD",
		"serial-number":   "ABC",
	})

	if res.TotalHDDSpace != 134217728 {
		t.Fatalf("total hdd = %d, want 134217728 bytes", res.TotalHDDSpace)
	}
	if res.FreeHDDSpace != 115208192 {
		t.Fatalf("free hdd = %d, want 115208192 bytes", res.FreeHDDSpace)
	}
	if res.TotalMemory != 134217728 || res.FreeMemory != 105840640 {
		t.Fatalf("memory = free %d total %d", res.FreeMemory, res.TotalMemory)
	}
}
