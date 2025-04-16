package metrics

import (
	"log"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/taihen/ros-exporter/pkg/mikrotik"
)

const namespace = "mikrotik"

// prometheus.Collector interface.
type MikrotikCollector struct {
	client *mikrotik.Client

	// Configuration flags
	collectBGP bool
	collectPPP bool

	// Exporter health
	upDesc              *prometheus.Desc
	scrapeDurationDesc  *prometheus.Desc
	lastScrapeErrorDesc *prometheus.Desc

	mutex sync.Mutex

	// System
	cpuLoadDesc     *prometheus.Desc
	memoryUsageDesc *prometheus.Desc
	totalMemoryDesc *prometheus.Desc
	uptimeDesc      *prometheus.Desc
	boardInfoDesc   *prometheus.Desc

	// Interface
	interfaceInfoDesc      *prometheus.Desc
	interfaceRxBytesDesc   *prometheus.Desc
	interfaceTxBytesDesc   *prometheus.Desc
	interfaceRxPacketsDesc *prometheus.Desc
	interfaceTxPacketsDesc *prometheus.Desc
	interfaceRxErrorsDesc  *prometheus.Desc
	interfaceTxErrorsDesc  *prometheus.Desc
	interfaceRxDropsDesc   *prometheus.Desc
	interfaceTxDropsDesc   *prometheus.Desc

	// BGP (Optional)
	bgpPeerInfoDesc          *prometheus.Desc
	bgpPeerStateDesc         *prometheus.Desc
	bgpPeerUptimeDesc        *prometheus.Desc
	bgpPeerPrefixCountDesc   *prometheus.Desc
	bgpPeerUpdatesSentDesc   *prometheus.Desc
	bgpPeerUpdatesRecvDesc   *prometheus.Desc
	bgpPeerWithdrawsSentDesc *prometheus.Desc
	bgpPeerWithdrawsRecvDesc *prometheus.Desc

	// PPP (Optional)
	pppActiveCountDesc *prometheus.Desc
	pppUserInfoDesc    *prometheus.Desc
	pppUserUptimeDesc  *prometheus.Desc
	// pppUserRxBytesDesc *prometheus.Desc // Removed as requested
	// pppUserTxBytesDesc *prometheus.Desc // Removed as requested
}

// NewMikrotikCollector creates a new collector instance.
// collectBGP and collectPPP flags control optional metric groups.
func NewMikrotikCollector(client *mikrotik.Client, collectBGP, collectPPP bool) *MikrotikCollector {
	mc := &MikrotikCollector{
		client:     client,
		collectBGP: collectBGP,
		collectPPP: collectPPP,
		upDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "up"),
			"Was the last scrape of the MikroTik router successful.",
			nil,
			nil,
		),
		scrapeDurationDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "scrape_duration_seconds"),
			"Duration of the last scrape.",
			nil,
			nil,
		),
		lastScrapeErrorDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "last_scrape_error"),
			"Whether the last scrape of metrics resulted in an error (1 for error, 0 for success).",
			nil,
			nil,
		),
		// System Descriptions
		cpuLoadDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "system", "cpu_load_percent"),
			"Current CPU load percentage.",
			nil, nil,
		),
		memoryUsageDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "system", "memory_usage_bytes"),
			"Currently used memory in bytes.",
			nil, nil,
		),
		totalMemoryDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "system", "memory_total_bytes"),
			"Total available memory in bytes.",
			nil, nil,
		),
		uptimeDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "system", "uptime_seconds"),
			"System uptime in seconds.",
			nil, nil,
		),
		boardInfoDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "system", "info"),
			"Non-numeric information about the router board.",
			[]string{"board_name", "model", "serial_number", "firmware_type", "factory_firmware", "current_firmware", "upgrade_firmware"},
			nil,
		),
		// Interface Descriptions
		interfaceInfoDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "interface", "info"),
			"Interface information (admin status, running status).",
			[]string{"name", "type", "comment", "mac_address"},
			nil,
		),
		interfaceRxBytesDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "interface", "receive_bytes_total"),
			"Total number of bytes received.",
			[]string{"name"},
			nil,
		),
		interfaceTxBytesDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "interface", "transmit_bytes_total"),
			"Total number of bytes transmitted.",
			[]string{"name"},
			nil,
		),
		interfaceRxPacketsDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "interface", "receive_packets_total"),
			"Total number of packets received.",
			[]string{"name"},
			nil,
		),
		interfaceTxPacketsDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "interface", "transmit_packets_total"),
			"Total number of packets transmitted.",
			[]string{"name"},
			nil,
		),
		interfaceRxErrorsDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "interface", "receive_errors_total"),
			"Total number of receive errors.",
			[]string{"name"},
			nil,
		),
		interfaceTxErrorsDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "interface", "transmit_errors_total"),
			"Total number of transmit errors.",
			[]string{"name"},
			nil,
		),
		interfaceRxDropsDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "interface", "receive_drops_total"),
			"Total number of received packets dropped.",
			[]string{"name"},
			nil,
		),
		interfaceTxDropsDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "interface", "transmit_drops_total"),
			"Total number of transmitted packets dropped.",
			[]string{"name"},
			nil,
		),
	}

	// Initialize BGP descriptions only if enabled
	if mc.collectBGP {
		mc.bgpPeerInfoDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "bgp_peer", "info"),
			"BGP peer information.",
			[]string{"name", "instance", "remote_address", "remote_as", "local_address", "local_role", "remote_role", "disabled"},
			nil,
		)
		mc.bgpPeerStateDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "bgp_peer", "state"),
			"BGP peer state (1 = Established, 0 = Other).",
			[]string{"name", "state_text"},
			nil,
		)
		mc.bgpPeerUptimeDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "bgp_peer", "uptime_seconds"),
			"BGP peer session uptime in seconds.",
			[]string{"name"},
			nil,
		)
		mc.bgpPeerPrefixCountDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "bgp_peer", "prefix_count"),
			"Number of prefixes received from the BGP peer.",
			[]string{"name"},
			nil,
		)
		mc.bgpPeerUpdatesSentDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "bgp_peer", "updates_sent_total"),
			"Total number of BGP update messages sent.",
			[]string{"name"},
			nil,
		)
		mc.bgpPeerUpdatesRecvDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "bgp_peer", "updates_received_total"),
			"Total number of BGP update messages received.",
			[]string{"name"},
			nil,
		)
		mc.bgpPeerWithdrawsSentDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "bgp_peer", "withdraws_sent_total"),
			"Total number of BGP withdraw messages sent.",
			[]string{"name"},
			nil,
		)
		mc.bgpPeerWithdrawsRecvDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "bgp_peer", "withdraws_received_total"),
			"Total number of BGP withdraw messages received.",
			[]string{"name"},
			nil,
		)
	}

	// Initialize PPP descriptions only if enabled
	if mc.collectPPP {
		mc.pppActiveCountDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "ppp", "active_users_count"),
			"Total number of active PPP users.",
			nil,
			nil,
		)
		mc.pppUserInfoDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "ppp_user", "info"),
			"PPP user session information (1 = active).",
			[]string{"name", "service", "caller_id", "address", "uptime_text"},
			nil,
		)
		mc.pppUserUptimeDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "ppp_user", "uptime_seconds"),
			"PPP user session uptime in seconds.",
			[]string{"name"},
			nil,
		)
		// mc.pppUserRxBytesDesc = prometheus.NewDesc(
		// 	prometheus.BuildFQName(namespace, "ppp_user", "receive_bytes_total"),
		// 	"Total number of bytes received by the PPP user.",
		// 	[]string{"name"},
		// 	nil,
		// )
		// mc.pppUserTxBytesDesc = prometheus.NewDesc(
		// 	prometheus.BuildFQName(namespace, "ppp_user", "transmit_bytes_total"),
		// 	"Total number of bytes transmitted by the PPP user.",
		// 	[]string{"name"},
		// 	nil,
		// )
	}

	return mc
}

// Describe sends the static descriptions of all metrics collected by this collector.
func (c *MikrotikCollector) Describe(ch chan<- *prometheus.Desc) {
	// Required metrics
	ch <- c.upDesc
	ch <- c.scrapeDurationDesc
	ch <- c.lastScrapeErrorDesc
	ch <- c.cpuLoadDesc
	ch <- c.memoryUsageDesc
	ch <- c.totalMemoryDesc
	ch <- c.uptimeDesc
	ch <- c.boardInfoDesc
	ch <- c.interfaceInfoDesc
	ch <- c.interfaceRxBytesDesc
	ch <- c.interfaceTxBytesDesc
	ch <- c.interfaceRxPacketsDesc
	ch <- c.interfaceTxPacketsDesc
	ch <- c.interfaceRxErrorsDesc
	ch <- c.interfaceTxErrorsDesc
	ch <- c.interfaceRxDropsDesc
	ch <- c.interfaceTxDropsDesc

	// Optional BGP metrics
	if c.collectBGP {
		ch <- c.bgpPeerInfoDesc
		ch <- c.bgpPeerStateDesc
		ch <- c.bgpPeerUptimeDesc
		ch <- c.bgpPeerPrefixCountDesc
		ch <- c.bgpPeerUpdatesSentDesc
		ch <- c.bgpPeerUpdatesRecvDesc
		ch <- c.bgpPeerWithdrawsSentDesc
		ch <- c.bgpPeerWithdrawsRecvDesc
	}

	// Optional PPP metrics
	if c.collectPPP {
		ch <- c.pppActiveCountDesc
		ch <- c.pppUserInfoDesc
		ch <- c.pppUserUptimeDesc
		// ch <- c.pppUserRxBytesDesc // Removed as requested
		// ch <- c.pppUserTxBytesDesc // Removed as requested
	}
}

// Collect fetches metrics from the MikroTik router and sends them to the Prometheus channel.
func (c *MikrotikCollector) Collect(ch chan<- prometheus.Metric) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	start := time.Now()
	log.Printf("Starting scrape for router %s (BGP: %t, PPP: %t)", c.client.Address, c.collectBGP, c.collectPPP)

	up := 1.0
	lastScrapeError := 0.0
	var bgpErr error // Declare bgpErr here to make it accessible later

	// Attempt connection first
	if err := c.client.Connect(); err != nil {
		log.Printf("ERROR: Failed to connect to router %s: %v", c.client.Address, err)
		up = 0.0
		lastScrapeError = 1.0
		duration := time.Since(start).Seconds()
		ch <- prometheus.MustNewConstMetric(c.upDesc, prometheus.GaugeValue, up)
		ch <- prometheus.MustNewConstMetric(c.scrapeDurationDesc, prometheus.GaugeValue, duration)
		ch <- prometheus.MustNewConstMetric(c.lastScrapeErrorDesc, prometheus.GaugeValue, lastScrapeError)
		// Note: We don't call client.Close() here as the connection failed.
		return // Stop collection if connection failed
	}
	// Connection successful, proceed with metric collection.
	// The client connection will be closed by the handler in main.go after ServeHTTP finishes.

	// --- Required Metrics ---

	// Fetch System Resources
	systemRes, sysErr := c.client.GetSystemResources()
	if sysErr != nil {
		log.Printf("ERROR: Failed to get system resources from %s: %v", c.client.Address, sysErr)
		// Don't mark 'up' as 0 here, as connection was successful, but record the error.
		lastScrapeError = 1.0
	} else {
		ch <- prometheus.MustNewConstMetric(c.cpuLoadDesc, prometheus.GaugeValue, float64(systemRes.CPULoad))
		ch <- prometheus.MustNewConstMetric(c.memoryUsageDesc, prometheus.GaugeValue, float64(systemRes.TotalMemory-systemRes.FreeMemory))
		ch <- prometheus.MustNewConstMetric(c.totalMemoryDesc, prometheus.GaugeValue, float64(systemRes.TotalMemory))
		ch <- prometheus.MustNewConstMetric(c.uptimeDesc, prometheus.GaugeValue, systemRes.Uptime.Seconds())
	}

	// Fetch Routerboard Info
	routerboard, rbErr := c.client.GetRouterboard()
	if rbErr != nil {
		log.Printf("ERROR: Failed to get routerboard info from %s: %v", c.client.Address, rbErr)
		if sysErr == nil { // Only set scrape error if system resource scrape was ok
			lastScrapeError = 1.0
		}
		if sysErr == nil { // Send info metric with empty labels if fetch failed but systemRes was okay
			ch <- prometheus.MustNewConstMetric(c.boardInfoDesc, prometheus.GaugeValue, 1, "", "", "", "", "", "", "")
		}
	} else if sysErr == nil { // Only send board info if system resource scrape was also successful
		ch <- prometheus.MustNewConstMetric(c.boardInfoDesc, prometheus.GaugeValue, 1,
			routerboard.BoardName,
			routerboard.Model,
			routerboard.SerialNumber,
			routerboard.FirmwareType,
			routerboard.FactoryFirmware,
			routerboard.CurrentFirmware,
			routerboard.UpgradeFirmware,
		)
	}

	// Fetch Interface Stats
	interfaceStats, ifErr := c.client.GetInterfaceStats()
	if ifErr != nil {
		log.Printf("ERROR: Failed to get interface stats from %s: %v", c.client.Address, ifErr)
		if sysErr == nil && rbErr == nil { // Only mark as error if other scrapes were ok
			lastScrapeError = 1.0
		}
	} else {
		for _, iface := range interfaceStats {
			opStatus := 0.0
			if iface.Running {
				opStatus = 1.0
			}
			ch <- prometheus.MustNewConstMetric(c.interfaceInfoDesc, prometheus.GaugeValue, opStatus,
				iface.Name, iface.Type, iface.Comment, iface.MACAddress,
			)

			ch <- prometheus.MustNewConstMetric(c.interfaceRxBytesDesc, prometheus.CounterValue, float64(iface.RxBytes), iface.Name)
			ch <- prometheus.MustNewConstMetric(c.interfaceTxBytesDesc, prometheus.CounterValue, float64(iface.TxBytes), iface.Name)
			ch <- prometheus.MustNewConstMetric(c.interfaceRxPacketsDesc, prometheus.CounterValue, float64(iface.RxPackets), iface.Name)
			ch <- prometheus.MustNewConstMetric(c.interfaceTxPacketsDesc, prometheus.CounterValue, float64(iface.TxPackets), iface.Name)
			ch <- prometheus.MustNewConstMetric(c.interfaceRxErrorsDesc, prometheus.CounterValue, float64(iface.RxErrors), iface.Name)
			ch <- prometheus.MustNewConstMetric(c.interfaceTxErrorsDesc, prometheus.CounterValue, float64(iface.TxErrors), iface.Name)
			ch <- prometheus.MustNewConstMetric(c.interfaceRxDropsDesc, prometheus.CounterValue, float64(iface.RxDrops), iface.Name)
			ch <- prometheus.MustNewConstMetric(c.interfaceTxDropsDesc, prometheus.CounterValue, float64(iface.TxDrops), iface.Name)
		}
	}

	// --- Optional BGP Metrics ---
	if c.collectBGP {
		var bgpStats []mikrotik.BGPPeerStat           // Corrected type name here
		bgpStats, bgpErr = c.client.GetBGPPeerStats() // Assign to the outer bgpErr
		if bgpErr != nil {
			log.Printf("ERROR: Failed to get BGP stats from %s: %v", c.client.Address, bgpErr)
			// Don't mark the whole scrape as failed, but record the error if others were ok
			if sysErr == nil && rbErr == nil && ifErr == nil {
				lastScrapeError = 1.0
			}
		} else {
			for _, peer := range bgpStats {
				disabledLabel := "false"
				if peer.Disabled {
					disabledLabel = "true"
				}
				ch <- prometheus.MustNewConstMetric(c.bgpPeerInfoDesc, prometheus.GaugeValue, 1,
					peer.Name, peer.Instance, peer.RemoteAddress, peer.RemoteAS, peer.LocalAddress, peer.LocalRole, peer.RemoteRole, disabledLabel,
				)

				stateValue := 0.0
				if peer.State == "established" {
					stateValue = 1.0
				}
				ch <- prometheus.MustNewConstMetric(c.bgpPeerStateDesc, prometheus.GaugeValue, stateValue, peer.Name, peer.State)

				ch <- prometheus.MustNewConstMetric(c.bgpPeerUptimeDesc, prometheus.GaugeValue, peer.Uptime.Seconds(), peer.Name)
				ch <- prometheus.MustNewConstMetric(c.bgpPeerPrefixCountDesc, prometheus.GaugeValue, float64(peer.PrefixCount), peer.Name)
				ch <- prometheus.MustNewConstMetric(c.bgpPeerUpdatesSentDesc, prometheus.CounterValue, float64(peer.UpdatesSent), peer.Name)
				ch <- prometheus.MustNewConstMetric(c.bgpPeerUpdatesRecvDesc, prometheus.CounterValue, float64(peer.UpdatesRecv), peer.Name)
				ch <- prometheus.MustNewConstMetric(c.bgpPeerWithdrawsSentDesc, prometheus.CounterValue, float64(peer.WithdrawsSent), peer.Name)
				ch <- prometheus.MustNewConstMetric(c.bgpPeerWithdrawsRecvDesc, prometheus.CounterValue, float64(peer.WithdrawsRecv), peer.Name)
			}
		}
	}

	// --- Optional PPP Metrics ---
	if c.collectPPP {
		pppUsers, pppErr := c.client.GetPPPActiveUsers()
		if pppErr != nil {
			log.Printf("ERROR: Failed to get PPP stats from %s: %v", c.client.Address, pppErr)
			// Don't mark the whole scrape as failed, but record the error if others were ok
			// Check BGP error status only if BGP collection was attempted
			bgpCollectionSuccessful := !c.collectBGP || bgpErr == nil
			if sysErr == nil && rbErr == nil && ifErr == nil && bgpCollectionSuccessful {
				lastScrapeError = 1.0
			}
		} else {
			ch <- prometheus.MustNewConstMetric(c.pppActiveCountDesc, prometheus.GaugeValue, float64(len(pppUsers)))

			for _, user := range pppUsers {
				ch <- prometheus.MustNewConstMetric(c.pppUserInfoDesc, prometheus.GaugeValue, 1,
					user.Name, user.Service, user.CallerID, user.Address, user.UptimeStr,
				)
				ch <- prometheus.MustNewConstMetric(c.pppUserUptimeDesc, prometheus.GaugeValue, user.Uptime.Seconds(), user.Name)
				// ch <- prometheus.MustNewConstMetric(c.pppUserRxBytesDesc, prometheus.CounterValue, float64(user.RxBytes), user.Name) // Removed as requested
				// ch <- prometheus.MustNewConstMetric(c.pppUserTxBytesDesc, prometheus.CounterValue, float64(user.TxBytes), user.Name) // Removed as requested
			}
		}
	}

	// --- Final Health Reporting ---
	duration := time.Since(start).Seconds()
	log.Printf("Scrape finished for router %s in %.2f seconds (up: %.0f, error: %.0f)", c.client.Address, duration, up, lastScrapeError)

	ch <- prometheus.MustNewConstMetric(c.upDesc, prometheus.GaugeValue, up)
	ch <- prometheus.MustNewConstMetric(c.scrapeDurationDesc, prometheus.GaugeValue, duration)
	ch <- prometheus.MustNewConstMetric(c.lastScrapeErrorDesc, prometheus.GaugeValue, lastScrapeError)
}
