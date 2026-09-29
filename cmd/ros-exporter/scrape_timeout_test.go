package main

import (
	"testing"
	"time"

	"github.com/taihen/ros-exporter/pkg/mikrotik"
)

func TestEffectiveScrapeTimeout(t *testing.T) {
	configured := 10 * time.Second
	cases := []struct {
		name   string
		cfg    time.Duration
		header string
		want   time.Duration
	}{
		{name: "missing", cfg: configured, header: "", want: configured},
		{name: "blank", cfg: configured, header: "  ", want: configured},
		{name: "invalid", cfg: configured, header: "nope", want: configured},
		{name: "zero", cfg: configured, header: "0", want: configured},
		{name: "negative", cfg: configured, header: "-1", want: configured},
		{name: "nan", cfg: configured, header: "NaN", want: configured},
		{name: "inf", cfg: configured, header: "+Inf", want: configured},
		{name: "larger than configured", cfg: configured, header: "50", want: configured},
		{name: "equal to configured", cfg: configured, header: "10", want: 10*time.Second - scrapeTimeoutMargin},
		{name: "shorter", cfg: configured, header: "4.5", want: 4500*time.Millisecond - scrapeTimeoutMargin},
		{name: "within margin", cfg: configured, header: "0.25", want: 250 * time.Millisecond},
		{name: "underflow", cfg: configured, header: "1e-12", want: time.Millisecond},
		{name: "non-positive configured", cfg: 0, header: "", want: mikrotik.DefaultTimeout},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := effectiveScrapeTimeout(tc.cfg, tc.header)
			if got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}
