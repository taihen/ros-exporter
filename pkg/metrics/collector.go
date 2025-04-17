package metrics

import (
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/taihen/ros-exporter/pkg/mikrotik"
)

const namespace = "mikrotik"

// MikrotikCollector implements the prometheus.Collector interface.
type MikrotikCollector struct {
	client *mikrotik.Client

	collectBGP      bool
	collectPPP      bool
	collectWireless bool

	upDesc              *prometheus.Desc
	scrapeDurationDesc  *prometheus.Desc
	lastScrapeErrorDesc *prometheus.Desc

	mutex sync.Mutex

	cpuLoadDesc     *prometheus.Desc
	memoryUsageDesc *prometheus.Desc
	totalMemoryDesc *prometheus.Desc
	uptimeDesc      *prometheus.Desc
	boardInfoDesc   *prometheus.Desc

	interfaceInfoDesc      *prometheus.Desc
	interfaceRxBytesDesc   *prometheus.Desc
	interfaceTxBytesDesc   *prometheus.Desc
	interfaceRxPacketsDesc *prometheus.Desc
	interfaceTxPacketsDesc *prometheus.Desc
	interfaceRxErrorsDesc  *prometheus.Desc
	interfaceTxErrorsDesc  *prometheus.Desc
	interfaceRxDropsDesc   *prometheus.Desc
	interfaceTxDropsDesc   *prometheus.Desc

	storageTotalBytesDesc *prometheus.Desc
	storageFreeBytesDesc  *prometheus.Desc
	storageUsedBytesDesc  *prometheus.Desc

	temperatureDesc      *prometheus.Desc
	boardTemperatureDesc *prometheus.Desc
	voltageDesc          *prometheus.Desc
	currentDesc          *prometheus.Desc
	powerConsumedDesc    *prometheus.Desc
	fanSpeedDesc         *prometheus.Desc

	bgpPeerInfoDesc          *prometheus.Desc
	bgpPeerStateDesc         *prometheus.Desc
	bgpPeerUptimeDesc        *prometheus.Desc
	bgpPeerPrefixCountDesc   *prometheus.Desc
	bgpPeerUpdatesSentDesc   *prometheus.Desc
	bgpPeerUpdatesRecvDesc   *prometheus.Desc
	bgpPeerWithdrawsSentDesc *prometheus.Desc
	bgpPeerWithdrawsRecvDesc *prometheus.Desc

	pppActiveCountDesc *prometheus.Desc
	pppUserInfoDesc    *prometheus.Desc
	pppUserUptimeDesc  *prometheus.Desc

	wirelessInterfaceInfoDesc           *prometheus.Desc
	wirelessInterfaceSignalStrengthDesc *prometheus.Desc
	wirelessInterfaceTxRateDesc         *prometheus.Desc
	wirelessInterfaceRxRateDesc         *prometheus.Desc
	wirelessClientInfoDesc              *prometheus.Desc
	wirelessClientSignalStrengthDesc    *prometheus.Desc
	wirelessClientTxCCQDesc             *prometheus.Desc
	wirelessActiveClientsDesc           *prometheus.Desc
}

// NewMikrotikCollector initializes a new collector instance.
func NewMikrotikCollector(client *mikrotik.Client, collectBGP, collectPPP, collectWireless bool) *MikrotikCollector {
	mc := &MikrotikCollector{
		client:          client,
		collectBGP:      collectBGP,
		collectPPP:      collectPPP,
		collectWireless: collectWireless,
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
		storageTotalBytesDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "system", "storage_total_bytes"),
			"Total system storage (HDD) size in bytes.",
			nil, nil,
		),
		storageFreeBytesDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "system", "storage_free_bytes"),
			"Free system storage (HDD) space in bytes.",
			nil, nil,
		),
		storageUsedBytesDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "system", "storage_used_bytes"),
			"Used system storage (HDD) space in bytes.",
			nil, nil,
		),
		temperatureDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "health", "temperature_celsius"),
			"System temperature (often CPU) in degrees Celsius.",
			[]string{"sensor"},
			nil,
		),
		boardTemperatureDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "health", "board_temperature_celsius"),
			"Board temperature in degrees Celsius.",
			nil, nil,
		),
		voltageDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "health", "voltage_volts"),
			"System voltage.",
			nil, nil,
		),
		currentDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "health", "current_amperes"),
			"System current draw in Amperes (if available).",
			nil, nil,
		),
		powerConsumedDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "health", "power_consumed_watts"),
			"System power consumption in Watts (if available).",
			nil, nil,
		),
		fanSpeedDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "health", "fan_speed_rpm"),
			"Fan speed in RPM (if available).",
			[]string{"fan"},
			nil,
		),
	}

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
	}

	if mc.collectWireless {
		mc.wirelessInterfaceInfoDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wireless_interface", "info"),
			"Wireless interface information.",
			[]string{"name", "ssid", "frequency"},
			nil,
		)
		mc.wirelessInterfaceSignalStrengthDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wireless_interface", "signal_strength_dbm"),
			"Wireless interface signal strength in dBm (primarily for station mode).",
			[]string{"name"},
			nil,
		)
		mc.wirelessInterfaceTxRateDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wireless_interface", "transmit_rate_bps"),
			"Wireless interface transmit rate in bits per second.",
			[]string{"name"},
			nil,
		)
		mc.wirelessInterfaceRxRateDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wireless_interface", "receive_rate_bps"),
			"Wireless interface receive rate in bits per second.",
			[]string{"name"},
			nil,
		)
		mc.wirelessClientInfoDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wireless_client", "info"),
			"Connected wireless client information (1 = connected).",
			[]string{"interface", "mac_address", "uptime_text"},
			nil,
		)
		mc.wirelessClientSignalStrengthDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wireless_client", "signal_strength_dbm"),
			"Connected wireless client signal strength in dBm.",
			[]string{"interface", "mac_address"},
			nil,
		)
		mc.wirelessClientTxCCQDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wireless_client", "transmit_ccq_percent"),
			"Connected wireless client transmit CCQ (Client Connection Quality) in percent.",
			[]string{"interface", "mac_address"},
			nil,
		)
		mc.wirelessActiveClientsDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wireless_interface", "active_clients_count"),
			"Number of active clients connected to a wireless interface (AP mode).",
			[]string{"interface"},
			nil,
		)
	}

	return mc
}

// Describe sends the static descriptions of all metrics collected by this collector.
func (c *MikrotikCollector) Describe(ch chan<- *prometheus.Desc) {
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

	ch <- c.storageTotalBytesDesc
	ch <- c.storageFreeBytesDesc
	ch <- c.storageUsedBytesDesc

	ch <- c.temperatureDesc
	ch <- c.boardTemperatureDesc
	ch <- c.voltageDesc
	ch <- c.currentDesc
	ch <- c.powerConsumedDesc
	ch <- c.fanSpeedDesc

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

	if c.collectPPP {
		ch <- c.pppActiveCountDesc
		ch <- c.pppUserInfoDesc
		ch <- c.pppUserUptimeDesc
	}

	if c.collectWireless {
		ch <- c.wirelessInterfaceInfoDesc
		ch <- c.wirelessInterfaceSignalStrengthDesc
		ch <- c.wirelessInterfaceTxRateDesc
		ch <- c.wirelessInterfaceRxRateDesc
		ch <- c.wirelessClientInfoDesc
		ch <- c.wirelessClientSignalStrengthDesc
		ch <- c.wirelessClientTxCCQDesc
		ch <- c.wirelessActiveClientsDesc
	}
}

// Collect fetches metrics from the MikroTik router and sends them to the Prometheus channel.
func (c *MikrotikCollector) Collect(ch chan<- prometheus.Metric) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	start := time.Now()
	log.Printf("Starting scrape for router %s", c.client.Address)

	up := 1.0
	lastScrapeError := 0.0
	var bgpErr error
	var healthErr error
	var pppErr error
	var wirelessErr error

	if err := c.client.Connect(); err != nil {
		log.Printf("ERROR: Failed to connect to router %s: %v", c.client.Address, err)
		up = 0.0
		lastScrapeError = 1.0
		duration := time.Since(start).Seconds()
		ch <- prometheus.MustNewConstMetric(c.upDesc, prometheus.GaugeValue, up)
		ch <- prometheus.MustNewConstMetric(c.scrapeDurationDesc, prometheus.GaugeValue, duration)
		ch <- prometheus.MustNewConstMetric(c.lastScrapeErrorDesc, prometheus.GaugeValue, lastScrapeError)
		return
	}

	systemRes, sysErr := c.client.GetSystemResources()
	if sysErr != nil {
		log.Printf("ERROR: Failed to get system resources from %s: %v", c.client.Address, sysErr)
		lastScrapeError = 1.0
	} else {
		ch <- prometheus.MustNewConstMetric(c.cpuLoadDesc, prometheus.GaugeValue, float64(systemRes.CPULoad))
		ch <- prometheus.MustNewConstMetric(c.memoryUsageDesc, prometheus.GaugeValue, float64(systemRes.TotalMemory-systemRes.FreeMemory))
		ch <- prometheus.MustNewConstMetric(c.totalMemoryDesc, prometheus.GaugeValue, float64(systemRes.TotalMemory))
		ch <- prometheus.MustNewConstMetric(c.uptimeDesc, prometheus.GaugeValue, systemRes.Uptime.Seconds())
		ch <- prometheus.MustNewConstMetric(c.storageTotalBytesDesc, prometheus.GaugeValue, float64(systemRes.TotalHDDSpace))
		ch <- prometheus.MustNewConstMetric(c.storageFreeBytesDesc, prometheus.GaugeValue, float64(systemRes.FreeHDDSpace))
		ch <- prometheus.MustNewConstMetric(c.storageUsedBytesDesc, prometheus.GaugeValue, float64(systemRes.TotalHDDSpace-systemRes.FreeHDDSpace))
	}

	routerboard, rbErr := c.client.GetRouterboard()
	if rbErr != nil {
		log.Printf("ERROR: Failed to get routerboard info from %s: %v", c.client.Address, rbErr)
		if sysErr == nil {
			lastScrapeError = 1.0
		}
		if sysErr == nil {
			ch <- prometheus.MustNewConstMetric(c.boardInfoDesc, prometheus.GaugeValue, 1, "", "", "", "", "", "", "")
		}
	} else if sysErr == nil {
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

	interfaceStats, ifErr := c.client.GetInterfaceStats()
	if ifErr != nil {
		log.Printf("ERROR: Failed to get interface stats from %s: %v", c.client.Address, ifErr)
		if sysErr == nil && rbErr == nil {
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

	health, healthErr := c.client.GetSystemHealth()
	if healthErr != nil {
		log.Printf("ERROR: Failed to get system health from %s: %v", c.client.Address, healthErr)
		if sysErr == nil && rbErr == nil && ifErr == nil {
			lastScrapeError = 1.0
		}
	} else if health != nil {
		if health.Temperature != 0 {
			ch <- prometheus.MustNewConstMetric(c.temperatureDesc, prometheus.GaugeValue, health.Temperature, "cpu")
		}
		if health.BoardTemperature != 0 && health.BoardTemperature != health.Temperature {
			ch <- prometheus.MustNewConstMetric(c.temperatureDesc, prometheus.GaugeValue, health.BoardTemperature, "board")
		}
		if health.Voltage != 0 {
			ch <- prometheus.MustNewConstMetric(c.voltageDesc, prometheus.GaugeValue, health.Voltage)
		}
		if health.Current != 0 {
			ch <- prometheus.MustNewConstMetric(c.currentDesc, prometheus.GaugeValue, health.Current)
		}
		if health.PowerConsumed != 0 {
			ch <- prometheus.MustNewConstMetric(c.powerConsumedDesc, prometheus.GaugeValue, health.PowerConsumed)
		}
		if health.FanSpeed != 0 {
			ch <- prometheus.MustNewConstMetric(c.fanSpeedDesc, prometheus.GaugeValue, float64(health.FanSpeed), "fan1")
		}
	} else {
		log.Printf("Info: System health metrics not available or not supported on %s.", c.client.Address)
	}

	if c.collectBGP {
		var bgpStats []mikrotik.BGPPeerStat
		bgpStats, bgpErr = c.client.GetBGPPeerStats()
		if bgpErr != nil {
			log.Printf("ERROR: Failed to get BGP stats from %s: %v", c.client.Address, bgpErr)
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

	if c.collectPPP {
		var pppUsers []mikrotik.PPPUserStat
		pppUsers, pppErr = c.client.GetPPPActiveUsers()
		if pppErr != nil {
			log.Printf("ERROR: Failed to get PPP stats from %s: %v", c.client.Address, pppErr)
			bgpCollectionSuccessful := !c.collectBGP || bgpErr == nil
			healthCollectionSuccessful := healthErr == nil
			if sysErr == nil && rbErr == nil && ifErr == nil && bgpCollectionSuccessful && healthCollectionSuccessful {
				lastScrapeError = 1.0
			}
		} else {
			ch <- prometheus.MustNewConstMetric(c.pppActiveCountDesc, prometheus.GaugeValue, float64(len(pppUsers)))

			for _, user := range pppUsers {
				ch <- prometheus.MustNewConstMetric(c.pppUserInfoDesc, prometheus.GaugeValue, 1,
					user.Name, user.Service, user.CallerID, user.Address, user.UptimeStr,
				)
				ch <- prometheus.MustNewConstMetric(c.pppUserUptimeDesc, prometheus.GaugeValue, user.Uptime.Seconds(), user.Name)
			}
		}
	}

	if c.collectWireless {
		wirelessInterfaces, wlIfErr := c.client.FetchWirelessInterfaces()
		if wlIfErr != nil {
			log.Printf("ERROR: Failed to get Wireless Interface stats from %s: %v", c.client.Address, wlIfErr)
			wirelessErr = wlIfErr
			bgpOk := !c.collectBGP || bgpErr == nil
			pppOk := !c.collectPPP || pppErr == nil
			healthOk := healthErr == nil
			if sysErr == nil && rbErr == nil && ifErr == nil && bgpOk && pppOk && healthOk {
				lastScrapeError = 1.0
			}
		} else if wirelessInterfaces != nil {
			for _, iface := range wirelessInterfaces {
				ch <- prometheus.MustNewConstMetric(c.wirelessInterfaceInfoDesc, prometheus.GaugeValue, 1,
					iface.Name, iface.SSID, strconv.Itoa(iface.Frequency),
				)
				if iface.SignalStrength != 0 {
					ch <- prometheus.MustNewConstMetric(c.wirelessInterfaceSignalStrengthDesc, prometheus.GaugeValue, float64(iface.SignalStrength), iface.Name)
				}
				if iface.TxRate > 0 {
					ch <- prometheus.MustNewConstMetric(c.wirelessInterfaceTxRateDesc, prometheus.GaugeValue, iface.TxRate, iface.Name)
				}
				if iface.RxRate > 0 {
					ch <- prometheus.MustNewConstMetric(c.wirelessInterfaceRxRateDesc, prometheus.GaugeValue, iface.RxRate, iface.Name)
				}
			}
		}

		wirelessClients, wlClientErr := c.client.FetchWirelessClients()
		if wlClientErr != nil {
			log.Printf("ERROR: Failed to get Wireless Client stats from %s: %v", c.client.Address, wlClientErr)
			if wirelessErr == nil {
				wirelessErr = wlClientErr
			}
			bgpOk := !c.collectBGP || bgpErr == nil
			pppOk := !c.collectPPP || pppErr == nil
			healthOk := healthErr == nil
			wlIfOk := wlIfErr == nil
			if sysErr == nil && rbErr == nil && ifErr == nil && bgpOk && pppOk && healthOk && wlIfOk {
				lastScrapeError = 1.0
			}
		} else if wirelessClients != nil {
			clientCounts := make(map[string]int)
			for _, client := range wirelessClients {
				clientCounts[client.Interface]++

				ch <- prometheus.MustNewConstMetric(c.wirelessClientInfoDesc, prometheus.GaugeValue, 1,
					client.Interface, client.MacAddress, client.Uptime,
				)
				if client.SignalStrength != 0 {
					ch <- prometheus.MustNewConstMetric(c.wirelessClientSignalStrengthDesc, prometheus.GaugeValue, float64(client.SignalStrength), client.Interface, client.MacAddress)
				}
				if client.TxCCQ != 0 {
					ch <- prometheus.MustNewConstMetric(c.wirelessClientTxCCQDesc, prometheus.GaugeValue, float64(client.TxCCQ), client.Interface, client.MacAddress)
				}
			}

			for ifaceName, count := range clientCounts {
				ch <- prometheus.MustNewConstMetric(c.wirelessActiveClientsDesc, prometheus.GaugeValue, float64(count), ifaceName)
			}
		}
	}

	duration := time.Since(start).Seconds()
	log.Printf("Scrape finished for router %s in %.2f seconds", c.client.Address, duration)

	ch <- prometheus.MustNewConstMetric(c.upDesc, prometheus.GaugeValue, up)
	ch <- prometheus.MustNewConstMetric(c.scrapeDurationDesc, prometheus.GaugeValue, duration)
	ch <- prometheus.MustNewConstMetric(c.lastScrapeErrorDesc, prometheus.GaugeValue, lastScrapeError)
}
