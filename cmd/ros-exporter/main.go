package main

import (
	"context"
	"flag"
	"log"
	"net" // Import net package
	"net/http"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/taihen/ros-exporter/pkg/metrics"
	"github.com/taihen/ros-exporter/pkg/mikrotik"
)

const defaultUsername = "prometheus"
const defaultAPIPort = "8728" // Define default port locally for logging if needed

// Define command-line flags (keeping existing ones)
var (
	listenAddressFlag = flag.String("web.listen-address", ":9483", "Address to listen on for web interface and telemetry.")
	metricsPathFlag   = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics.")
	scrapeTimeout     = flag.Duration("scrape.timeout", 10*time.Second, "Timeout for scraping a target.")
)

func main() {
	flag.Parse() // Parse the flags

	log.Println("Starting MikroTik Prometheus Exporter")
	log.Printf("Listen Address: %s", *listenAddressFlag)
	log.Printf("Metrics Path: %s", *metricsPathFlag)
	log.Printf("Scrape Timeout: %s", *scrapeTimeout)
	log.Printf("Default Username (if not provided via param): %s", defaultUsername)
	log.Printf("Default API Port (if not provided via param): %s", defaultAPIPort) // Use local const for log

	// Setup signal handling for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Setup HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc(*metricsPathFlag, handleMetricsRequest)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<html>
			<head><title>MikroTik Exporter</title></head>
			<body>
			<h1>MikroTik Exporter</h1>
			<p><a href='` + *metricsPathFlag + `'>Metrics</a></p>
			</body>
			</html>`))
	})

	server := &http.Server{
		Addr:    *listenAddressFlag,
		Handler: mux,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Listening on %s", *listenAddressFlag)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server ListenAndServe: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	log.Println("Shutting down server...")

	// Shutdown server with a timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("HTTP server Shutdown: %v", err)
	}

	log.Println("Server gracefully stopped")
}

// handleMetricsRequest handles incoming scrapes from Prometheus.
func handleMetricsRequest(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	target := query.Get("target")
	user := query.Get("user")
	password := query.Get("password")
	port := query.Get("port") // Optional port parameter
	collectBGPParam := query.Get("collect_bgp")
	collectPPPParam := query.Get("collect_ppp")
	collectWirelessParam := query.Get("collect_wireless") // Added wireless param

	// --- Parameter Handling ---
	if target == "" {
		http.Error(w, "'target' parameter is missing", http.StatusBadRequest)
		return
	}

	effectiveUser := user
	if effectiveUser == "" {
		effectiveUser = defaultUsername
		log.Printf("Scrape for target %s: 'user' parameter missing, using default '%s'", target, defaultUsername)
	}

	// Construct address: Use target directly if port param is missing (client will add default).
	// If port param is present, join it with the target.
	address := target
	if port != "" {
		address = net.JoinHostPort(target, port)
		log.Printf("Scrape for target %s: Using specified port %s -> %s", target, port, address)
	} else {
		log.Printf("Scrape for target %s: No port specified, client will use default.", target)
	}

	collectBGP, _ := strconv.ParseBool(collectBGPParam)
	collectPPP, _ := strconv.ParseBool(collectPPPParam)
	collectWireless, _ := strconv.ParseBool(collectWirelessParam) // Added wireless parsing

	log.Printf("Processing scrape request for address: %s, user: %s, collect_bgp: %t, collect_ppp: %t, collect_wireless: %t",
		address, effectiveUser, collectBGP, collectPPP, collectWireless) // Added wireless to log

	// --- Client & Collector Setup ---
	// Pass the potentially port-modified address to NewClient
	client := mikrotik.NewClient(address, effectiveUser, password, *scrapeTimeout)
	registry := prometheus.NewRegistry()
	collector := metrics.NewMikrotikCollector(client, collectBGP, collectPPP, collectWireless) // Added collectWireless arg
	registry.MustRegister(collector)

	// --- Serve Metrics ---
	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)

	log.Printf("Finished scrape request for address: %s", address)
	client.Close() // Close the connection established for this request
}
