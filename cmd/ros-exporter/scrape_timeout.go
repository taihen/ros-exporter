package main

import (
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/taihen/ros-exporter/pkg/mikrotik"
)

const prometheusScrapeTimeoutHeader = "X-Prometheus-Scrape-Timeout-Seconds"

// scrapeTimeoutMargin is reserved so the handler can write metrics before
// Prometheus cancels the scrape.
const scrapeTimeoutMargin = 500 * time.Millisecond

// effectiveScrapeTimeout is the budget for one target scrape.
// Missing or invalid header values fall back to configured.
// The header is adjusted before it is compared with configured:
// subtract the margin when the header is longer than the margin,
// otherwise keep the header, or 1ms if it rounds down to zero.
func effectiveScrapeTimeout(configured time.Duration, header string) time.Duration {
	if configured <= 0 {
		configured = mikrotik.DefaultTimeout
	}
	header = strings.TrimSpace(header)
	if header == "" {
		return configured
	}
	seconds, err := strconv.ParseFloat(header, 64)
	if err != nil || seconds <= 0 || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		log.Printf("ignoring invalid %s %q; using %s", prometheusScrapeTimeoutHeader, header, configured)
		return configured
	}
	maxHeader := configured.Seconds() + scrapeTimeoutMargin.Seconds()
	if seconds >= maxHeader {
		return configured
	}
	prom := time.Duration(seconds * float64(time.Second))
	if prom <= 0 {
		return time.Millisecond
	}
	if prom <= scrapeTimeoutMargin {
		return prom
	}
	adjusted := prom - scrapeTimeoutMargin
	if adjusted > configured {
		return configured
	}
	return adjusted
}
