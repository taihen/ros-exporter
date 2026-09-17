package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/taihen/ros-exporter/pkg/metrics"
	"github.com/taihen/ros-exporter/pkg/mikrotik"
)

const defaultUsername = "prometheus"
const defaultAPIPort = "8728"

// Set via -ldflags at build/release time.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var (
	listenAddressFlag = flag.String("web.listen-address", ":9483", "Address to listen on for web interface and telemetry.")
	metricsPathFlag   = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics.")
	scrapeTimeout     = flag.Duration("scrape.timeout", mikrotik.DefaultTimeout, "Timeout for scraping a target (entire scrape budget).")
	maxConcurrent     = flag.Int("scrape.max-concurrent", mikrotik.DefaultMaxConcurrent, "Maximum concurrent RouterOS API connections.")
	showVersion       = flag.Bool("version", false, "Print version and exit.")
)

func main() {
	flag.Parse()

	if *showVersion {
		fmt.Printf("ros-exporter %s (commit=%s date=%s)\n", version, commit, date)
		os.Exit(0)
	}

	mikrotik.SetMaxConcurrent(*maxConcurrent)

	log.Printf("Starting MikroTik Prometheus Exporter %s", version)
	log.Printf("Listen Address: %s", *listenAddressFlag)
	log.Printf("Metrics Path: %s", *metricsPathFlag)
	log.Printf("Scrape Timeout: %s", *scrapeTimeout)
	log.Printf("Max Concurrent Connections: %d", *maxConcurrent)
	log.Printf("Default Username (if not provided via param): %s", defaultUsername)
	log.Printf("Default API Port (if not provided via param): %s", defaultAPIPort)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Exporter process metrics on a dedicated registry for /-/metrics or root health.
	processRegistry := prometheus.NewRegistry()
	processRegistry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "mikrotik_exporter_build_info",
			Help: "Build info of the exporter process.",
			ConstLabels: prometheus.Labels{
				"version": version,
				"commit":  commit,
			},
		}, func() float64 { return 1 }),
	)

	mux := http.NewServeMux()
	mux.HandleFunc(*metricsPathFlag, handleMetricsRequest)
	mux.Handle("/-/metrics", promhttp.HandlerFor(processRegistry, promhttp.HandlerOpts{}))
	writeOK := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK\n"))
	}
	mux.HandleFunc("/-/healthy", writeOK)
	mux.HandleFunc("/-/ready", writeOK)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<html>
			<head><title>MikroTik Exporter</title></head>
			<body>
			<h1>MikroTik Exporter ` + version + `</h1>
			<p><a href="` + *metricsPathFlag + `">Target metrics</a> (requires ?target=)</p>
			<p><a href="/-/metrics">Process metrics</a></p>
			<p><a href="/-/healthy">Health</a> | <a href="/-/ready">Ready</a></p>
			</body>
			</html>`))
	})

	server := &http.Server{
		Addr:              *listenAddressFlag,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("Listening on %s", *listenAddressFlag)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server ListenAndServe: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("HTTP server Shutdown: %v", err)
	}

	log.Println("Server gracefully stopped")
}

func handleMetricsRequest(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	target := query.Get("target")
	user := query.Get("user")
	password := query.Get("password")
	port := query.Get("port")

	if target == "" {
		http.Error(w, "'target' parameter is missing", http.StatusBadRequest)
		return
	}

	effectiveUser := user
	if effectiveUser == "" {
		effectiveUser = defaultUsername
		log.Printf("Scrape for target %s: 'user' parameter missing, using default '%s'", target, defaultUsername)
	}

	address := target
	if port != "" {
		address = net.JoinHostPort(target, port)
		log.Printf("Scrape for target %s: Using specified port %s -> %s", target, port, address)
	}

	collectBGP, _ := strconv.ParseBool(query.Get("collect_bgp"))
	collectPPP, _ := strconv.ParseBool(query.Get("collect_ppp"))
	collectWireless, _ := strconv.ParseBool(query.Get("collect_wireless"))
	collectOSPF, _ := strconv.ParseBool(query.Get("collect_ospf"))
	collectOptics, _ := strconv.ParseBool(query.Get("collect_optics"))

	log.Printf("Processing scrape request for address: %s, user: %s, bgp=%t ppp=%t wireless=%t ospf=%t optics=%t",
		address, effectiveUser, collectBGP, collectPPP, collectWireless, collectOSPF, collectOptics)

	client := mikrotik.NewClient(address, effectiveUser, password, *scrapeTimeout)
	defer client.Close()

	registry := prometheus.NewRegistry()
	collector := metrics.NewMikrotikCollectorWithOptions(client, metrics.CollectorOptions{
		CollectBGP:      collectBGP,
		CollectPPP:      collectPPP,
		CollectWireless: collectWireless,
		CollectOSPF:     collectOSPF,
		CollectOptics:   collectOptics,
		Version:         version,
		Commit:          commit,
	})
	registry.MustRegister(collector)

	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)

	log.Printf("Finished scrape request for address: %s", address)
}
