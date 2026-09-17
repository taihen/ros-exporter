package metrics

import (
	"context"
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
	collectOSPF     bool
	collectOptics   bool

	upDesc                 *prometheus.Desc
	connectedDesc          *prometheus.Desc
	scrapeSuccessDesc      *prometheus.Desc
	scrapeDurationDesc     *prometheus.Desc
	lastScrapeErrorDesc    *prometheus.Desc
	collectorErrorDesc     *prometheus.Desc
	collectorSupportedDesc *prometheus.Desc
	buildInfoDesc          *prometheus.Desc

	mutex sync.Mutex

	cpuLoadDesc     *prometheus.Desc
	memoryUsageDesc *prometheus.Desc
	totalMemoryDesc *prometheus.Desc
	uptimeDesc      *prometheus.Desc
	boardInfoDesc   *prometheus.Desc

	interfaceInfoDesc      *prometheus.Desc
	interfaceAdminUpDesc   *prometheus.Desc
	interfaceSpeedDesc     *prometheus.Desc
	interfaceDuplexDesc    *prometheus.Desc
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
	pppUserRxBytesDesc *prometheus.Desc
	pppUserTxBytesDesc *prometheus.Desc

	wirelessInterfaceInfoDesc           *prometheus.Desc
	wirelessInterfaceSignalStrengthDesc *prometheus.Desc
	wirelessInterfaceTxRateDesc         *prometheus.Desc
	wirelessInterfaceRxRateDesc         *prometheus.Desc
	wirelessInterfaceNoiseFloorDesc     *prometheus.Desc
	wirelessClientInfoDesc              *prometheus.Desc
	wirelessClientSignalStrengthDesc    *prometheus.Desc
	wirelessClientTxCCQDesc             *prometheus.Desc
	wirelessClientTxRateDesc            *prometheus.Desc
	wirelessClientRxRateDesc            *prometheus.Desc
	wirelessClientNoiseFloorDesc        *prometheus.Desc
	wirelessClientUptimeDesc            *prometheus.Desc
	wirelessActiveClientsDesc           *prometheus.Desc

	ospfNeighborInfoDesc  *prometheus.Desc
	ospfNeighborStateDesc *prometheus.Desc

	transceiverTempDesc    *prometheus.Desc
	transceiverTxPowerDesc *prometheus.Desc
	transceiverRxPowerDesc *prometheus.Desc

	version string
	commit  string
}

type CollectorOptions struct {
	CollectBGP      bool
	CollectPPP      bool
	CollectWireless bool
	CollectOSPF     bool
	CollectOptics   bool
	Version         string
	Commit          string
}

// NewMikrotikCollector initializes a new collector instance.
func NewMikrotikCollector(client *mikrotik.Client, collectBGP, collectPPP, collectWireless bool) *MikrotikCollector {
	return NewMikrotikCollectorWithOptions(client, CollectorOptions{
		CollectBGP:      collectBGP,
		CollectPPP:      collectPPP,
		CollectWireless: collectWireless,
	})
}

func NewMikrotikCollectorWithOptions(client *mikrotik.Client, opts CollectorOptions) *MikrotikCollector {
	mc := &MikrotikCollector{
		client:          client,
		collectBGP:      opts.CollectBGP,
		collectPPP:      opts.CollectPPP,
		collectWireless: opts.CollectWireless,
		collectOSPF:     opts.CollectOSPF,
		collectOptics:   opts.CollectOptics,
		version:         opts.Version,
		commit:          opts.Commit,
		upDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "up"),
			"1 if the exporter connected to the MikroTik API (not full scrape success). Prefer mikrotik_connected.",
			nil, nil,
		),
		connectedDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "connected"),
			"1 if the last scrape established an API connection.",
			nil, nil,
		),
		scrapeSuccessDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "scrape_success"),
			"1 if the last scrape completed without collector errors.",
			nil, nil,
		),
		scrapeDurationDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "scrape_duration_seconds"),
			"Duration of the last scrape.",
			nil, nil,
		),
		lastScrapeErrorDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "last_scrape_error"),
			"1 if the last scrape had any collector error (inverse of scrape_success when connected).",
			nil, nil,
		),
		collectorErrorDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "collector", "error"),
			"1 if the named collector failed during the last scrape.",
			[]string{"collector"}, nil,
		),
		collectorSupportedDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "collector", "supported"),
			"1 if the named collector is supported/enabled on the target for this scrape.",
			[]string{"collector"}, nil,
		),
		buildInfoDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "build_info"),
			"Build information of the exporter (always 1).",
			[]string{"version", "commit"}, nil,
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
			"Interface operational status (1 = running).",
			[]string{"name", "type", "comment", "mac_address"},
			nil,
		),
		interfaceAdminUpDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "interface", "admin_up"),
			"1 if the interface is administratively enabled (not disabled).",
			[]string{"name"}, nil,
		),
		interfaceSpeedDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "interface", "speed_bps"),
			"Negotiated or configured interface speed in bits per second.",
			[]string{"name"}, nil,
		),
		interfaceDuplexDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "interface", "full_duplex"),
			"1 if full duplex, 0 if half duplex (only when reported).",
			[]string{"name"}, nil,
		),
		interfaceRxBytesDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "interface", "receive_bytes_total"),
			"Total number of bytes received.",
			[]string{"name"}, nil,
		),
		interfaceTxBytesDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "interface", "transmit_bytes_total"),
			"Total number of bytes transmitted.",
			[]string{"name"}, nil,
		),
		interfaceRxPacketsDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "interface", "receive_packets_total"),
			"Total number of packets received.",
			[]string{"name"}, nil,
		),
		interfaceTxPacketsDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "interface", "transmit_packets_total"),
			"Total number of packets transmitted.",
			[]string{"name"}, nil,
		),
		interfaceRxErrorsDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "interface", "receive_errors_total"),
			"Total number of receive errors.",
			[]string{"name"}, nil,
		),
		interfaceTxErrorsDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "interface", "transmit_errors_total"),
			"Total number of transmit errors.",
			[]string{"name"}, nil,
		),
		interfaceRxDropsDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "interface", "receive_drops_total"),
			"Total number of received packets dropped.",
			[]string{"name"}, nil,
		),
		interfaceTxDropsDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "interface", "transmit_drops_total"),
			"Total number of transmitted packets dropped.",
			[]string{"name"}, nil,
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
			"System temperature in degrees Celsius.",
			[]string{"sensor"}, nil,
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
			[]string{"fan"}, nil,
		),
	}

	if mc.collectBGP {
		mc.bgpPeerInfoDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "bgp_peer", "info"),
			"BGP peer information.",
			[]string{"name", "routing_instance", "remote_address", "remote_as", "local_address", "local_role", "remote_role", "disabled"},
			nil,
		)
		mc.bgpPeerStateDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "bgp_peer", "state"),
			"BGP peer state (1 = Established, 0 = Other).",
			[]string{"name", "state_text"}, nil,
		)
		mc.bgpPeerUptimeDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "bgp_peer", "uptime_seconds"),
			"BGP peer session uptime in seconds.",
			[]string{"name"}, nil,
		)
		mc.bgpPeerPrefixCountDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "bgp_peer", "prefix_count"),
			"Number of prefixes received from the BGP peer.",
			[]string{"name"}, nil,
		)
		mc.bgpPeerUpdatesSentDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "bgp_peer", "updates_sent_total"),
			"Total number of BGP update messages sent.",
			[]string{"name"}, nil,
		)
		mc.bgpPeerUpdatesRecvDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "bgp_peer", "updates_received_total"),
			"Total number of BGP update messages received.",
			[]string{"name"}, nil,
		)
		mc.bgpPeerWithdrawsSentDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "bgp_peer", "withdraws_sent_total"),
			"Total number of BGP withdraw messages sent.",
			[]string{"name"}, nil,
		)
		mc.bgpPeerWithdrawsRecvDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "bgp_peer", "withdraws_received_total"),
			"Total number of BGP withdraw messages received.",
			[]string{"name"}, nil,
		)
	}

	if mc.collectPPP {
		mc.pppActiveCountDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "ppp", "active_users_count"),
			"Total number of active PPP users.",
			nil, nil,
		)
		mc.pppUserInfoDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "ppp_user", "info"),
			"PPP user session information (1 = active).",
			[]string{"name", "service", "caller_id", "address"}, nil,
		)
		mc.pppUserUptimeDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "ppp_user", "uptime_seconds"),
			"PPP user session uptime in seconds.",
			[]string{"name"}, nil,
		)
		mc.pppUserRxBytesDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "ppp_user", "receive_bytes_total"),
			"Bytes received by the PPP user session.",
			[]string{"name"}, nil,
		)
		mc.pppUserTxBytesDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "ppp_user", "transmit_bytes_total"),
			"Bytes transmitted by the PPP user session.",
			[]string{"name"}, nil,
		)
	}

	if mc.collectWireless {
		mc.wirelessInterfaceInfoDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wireless_interface", "info"),
			"Wireless interface information.",
			[]string{"name", "ssid", "frequency"}, nil,
		)
		mc.wirelessInterfaceSignalStrengthDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wireless_interface", "signal_strength_dbm"),
			"Wireless interface signal strength in dBm.",
			[]string{"name"}, nil,
		)
		mc.wirelessInterfaceTxRateDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wireless_interface", "transmit_rate_bps"),
			"Wireless interface transmit rate in bits per second.",
			[]string{"name"}, nil,
		)
		mc.wirelessInterfaceRxRateDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wireless_interface", "receive_rate_bps"),
			"Wireless interface receive rate in bits per second.",
			[]string{"name"}, nil,
		)
		mc.wirelessInterfaceNoiseFloorDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wireless_interface", "noise_floor_dbm"),
			"Wireless interface noise floor in dBm.",
			[]string{"name"}, nil,
		)
		mc.wirelessClientInfoDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wireless_client", "info"),
			"Connected wireless client information (1 = connected).",
			[]string{"interface", "mac_address"}, nil,
		)
		mc.wirelessClientSignalStrengthDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wireless_client", "signal_strength_dbm"),
			"Connected wireless client signal strength in dBm.",
			[]string{"interface", "mac_address"}, nil,
		)
		mc.wirelessClientTxCCQDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wireless_client", "transmit_ccq_percent"),
			"Connected wireless client transmit CCQ in percent.",
			[]string{"interface", "mac_address"}, nil,
		)
		mc.wirelessClientTxRateDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wireless_client", "transmit_rate_bps"),
			"Connected wireless client transmit rate in bits per second.",
			[]string{"interface", "mac_address"}, nil,
		)
		mc.wirelessClientRxRateDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wireless_client", "receive_rate_bps"),
			"Connected wireless client receive rate in bits per second.",
			[]string{"interface", "mac_address"}, nil,
		)
		mc.wirelessClientNoiseFloorDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wireless_client", "noise_floor_dbm"),
			"Connected wireless client noise floor in dBm.",
			[]string{"interface", "mac_address"}, nil,
		)
		mc.wirelessClientUptimeDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wireless_client", "uptime_seconds"),
			"Connected wireless client session uptime in seconds.",
			[]string{"interface", "mac_address"}, nil,
		)
		mc.wirelessActiveClientsDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wireless_interface", "active_clients_count"),
			"Number of active clients connected to a wireless interface.",
			[]string{"interface"}, nil,
		)
	}

	if mc.collectOSPF {
		mc.ospfNeighborInfoDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "ospf_neighbor", "info"),
			"OSPF neighbor information (always 1).",
			[]string{"router_id", "address", "interface", "state_text"}, nil,
		)
		mc.ospfNeighborStateDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "ospf_neighbor", "state"),
			"OSPF neighbor state (1 = Full).",
			[]string{"router_id", "interface"}, nil,
		)
	}

	if mc.collectOptics {
		mc.transceiverTempDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "transceiver", "temperature_celsius"),
			"SFP/transceiver temperature in Celsius.",
			[]string{"interface"}, nil,
		)
		mc.transceiverTxPowerDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "transceiver", "transmit_power_dbm"),
			"SFP/transceiver TX power in dBm.",
			[]string{"interface"}, nil,
		)
		mc.transceiverRxPowerDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "transceiver", "receive_power_dbm"),
			"SFP/transceiver RX power in dBm.",
			[]string{"interface"}, nil,
		)
	}

	return mc
}

func (c *MikrotikCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.upDesc
	ch <- c.connectedDesc
	ch <- c.scrapeSuccessDesc
	ch <- c.scrapeDurationDesc
	ch <- c.lastScrapeErrorDesc
	ch <- c.collectorErrorDesc
	ch <- c.collectorSupportedDesc
	ch <- c.buildInfoDesc
	ch <- c.cpuLoadDesc
	ch <- c.memoryUsageDesc
	ch <- c.totalMemoryDesc
	ch <- c.uptimeDesc
	ch <- c.boardInfoDesc
	ch <- c.interfaceInfoDesc
	ch <- c.interfaceAdminUpDesc
	ch <- c.interfaceSpeedDesc
	ch <- c.interfaceDuplexDesc
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
		ch <- c.pppUserRxBytesDesc
		ch <- c.pppUserTxBytesDesc
	}
	if c.collectWireless {
		ch <- c.wirelessInterfaceInfoDesc
		ch <- c.wirelessInterfaceSignalStrengthDesc
		ch <- c.wirelessInterfaceTxRateDesc
		ch <- c.wirelessInterfaceRxRateDesc
		ch <- c.wirelessInterfaceNoiseFloorDesc
		ch <- c.wirelessClientInfoDesc
		ch <- c.wirelessClientSignalStrengthDesc
		ch <- c.wirelessClientTxCCQDesc
		ch <- c.wirelessClientTxRateDesc
		ch <- c.wirelessClientRxRateDesc
		ch <- c.wirelessClientNoiseFloorDesc
		ch <- c.wirelessClientUptimeDesc
		ch <- c.wirelessActiveClientsDesc
	}
	if c.collectOSPF {
		ch <- c.ospfNeighborInfoDesc
		ch <- c.ospfNeighborStateDesc
	}
	if c.collectOptics {
		ch <- c.transceiverTempDesc
		ch <- c.transceiverTxPowerDesc
		ch <- c.transceiverRxPowerDesc
	}
}

func (c *MikrotikCollector) Collect(ch chan<- prometheus.Metric) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	start := time.Now()
	log.Printf("Starting scrape for router %s", c.client.Address)

	version := c.version
	if version == "" {
		version = "dev"
	}
	ch <- prometheus.MustNewConstMetric(c.buildInfoDesc, prometheus.GaugeValue, 1, version, c.commit)

	ctx, cancel := c.client.BeginScrape(context.Background())
	defer cancel()

	scrape := &scrapeState{
		ch:     ch,
		addr:   c.client.Address,
		errors: map[string]bool{},
		supported: map[string]float64{
			"system":     1,
			"interfaces": 1,
			"health":     1,
		},
	}

	if err := c.client.ConnectContext(ctx); err != nil {
		duration := time.Since(start).Seconds()
		c.emitScrapeStatus(ch, false, false, duration)
		ch <- prometheus.MustNewConstMetric(c.collectorErrorDesc, prometheus.GaugeValue, 1, "connect")
		ch <- prometheus.MustNewConstMetric(c.collectorSupportedDesc, prometheus.GaugeValue, 1, "connect")
		return
	}

	c.collectSystem(scrape)
	c.collectRouterboard(scrape)
	c.collectInterfaces(scrape)
	c.collectHealth(scrape)
	if c.collectBGP {
		c.collectBGPPeers(scrape)
	}
	if c.collectPPP {
		c.collectPPPUsers(scrape)
	}
	if c.collectWireless {
		c.collectWirelessMetrics(scrape)
	}
	if c.collectOSPF {
		c.collectOSPFNeighbors(scrape)
	}
	if c.collectOptics {
		c.collectOpticsMetrics(scrape)
	}

	hasError := len(scrape.errors) > 0 || ctx.Err() != nil
	scrape.emitCollectorStatus(c)
	duration := time.Since(start).Seconds()
	log.Printf("Scrape finished for router %s in %.2f seconds (success=%v)", c.client.Address, duration, !hasError)
	c.emitScrapeStatus(ch, true, !hasError, duration)
}

type scrapeState struct {
	ch        chan<- prometheus.Metric
	addr      string
	errors    map[string]bool
	supported map[string]float64
}

func (s *scrapeState) markErr(name string, err error) {
	if err != nil {
		log.Printf("ERROR: collector %s on %s: %v", name, s.addr, err)
		s.errors[name] = true
	}
}

func (s *scrapeState) emitCollectorStatus(c *MikrotikCollector) {
	for name, supported := range s.supported {
		s.ch <- prometheus.MustNewConstMetric(c.collectorSupportedDesc, prometheus.GaugeValue, supported, name)
		s.ch <- prometheus.MustNewConstMetric(c.collectorErrorDesc, prometheus.GaugeValue, boolToFloat(s.errors[name]), name)
	}
	for name := range s.errors {
		if _, ok := s.supported[name]; !ok {
			s.ch <- prometheus.MustNewConstMetric(c.collectorErrorDesc, prometheus.GaugeValue, 1, name)
			s.ch <- prometheus.MustNewConstMetric(c.collectorSupportedDesc, prometheus.GaugeValue, 1, name)
		}
	}
}

func (c *MikrotikCollector) emitScrapeStatus(ch chan<- prometheus.Metric, connected, success bool, duration float64) {
	ch <- prometheus.MustNewConstMetric(c.upDesc, prometheus.GaugeValue, boolToFloat(connected))
	ch <- prometheus.MustNewConstMetric(c.connectedDesc, prometheus.GaugeValue, boolToFloat(connected))
	ch <- prometheus.MustNewConstMetric(c.scrapeSuccessDesc, prometheus.GaugeValue, boolToFloat(success))
	ch <- prometheus.MustNewConstMetric(c.scrapeDurationDesc, prometheus.GaugeValue, duration)
	ch <- prometheus.MustNewConstMetric(c.lastScrapeErrorDesc, prometheus.GaugeValue, boolToFloat(!success))
}

func boolToFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func (c *MikrotikCollector) collectSystem(s *scrapeState) {
	systemRes, err := c.client.GetSystemResources()
	s.markErr("system", err)
	if err != nil {
		return
	}
	s.ch <- prometheus.MustNewConstMetric(c.cpuLoadDesc, prometheus.GaugeValue, float64(systemRes.CPULoad))
	s.ch <- prometheus.MustNewConstMetric(c.memoryUsageDesc, prometheus.GaugeValue, float64(systemRes.TotalMemory-systemRes.FreeMemory))
	s.ch <- prometheus.MustNewConstMetric(c.totalMemoryDesc, prometheus.GaugeValue, float64(systemRes.TotalMemory))
	s.ch <- prometheus.MustNewConstMetric(c.uptimeDesc, prometheus.GaugeValue, systemRes.Uptime.Seconds())
	s.ch <- prometheus.MustNewConstMetric(c.storageTotalBytesDesc, prometheus.GaugeValue, float64(systemRes.TotalHDDSpace))
	s.ch <- prometheus.MustNewConstMetric(c.storageFreeBytesDesc, prometheus.GaugeValue, float64(systemRes.FreeHDDSpace))
	s.ch <- prometheus.MustNewConstMetric(c.storageUsedBytesDesc, prometheus.GaugeValue, float64(systemRes.TotalHDDSpace-systemRes.FreeHDDSpace))
}

func (c *MikrotikCollector) collectRouterboard(s *scrapeState) {
	s.supported["routerboard"] = 1
	routerboard, err := c.client.GetRouterboard()
	s.markErr("routerboard", err)
	if err != nil {
		return
	}
	s.ch <- prometheus.MustNewConstMetric(c.boardInfoDesc, prometheus.GaugeValue, 1,
		routerboard.BoardName, routerboard.Model, routerboard.SerialNumber,
		routerboard.FirmwareType, routerboard.FactoryFirmware, routerboard.CurrentFirmware, routerboard.UpgradeFirmware,
	)
}

func (c *MikrotikCollector) collectInterfaces(s *scrapeState) {
	interfaceStats, err := c.client.GetInterfaceStats()
	s.markErr("interfaces", err)
	if err != nil {
		return
	}
	for _, iface := range interfaceStats {
		s.ch <- prometheus.MustNewConstMetric(c.interfaceInfoDesc, prometheus.GaugeValue, boolToFloat(iface.Running),
			iface.Name, iface.Type, iface.Comment, iface.MACAddress,
		)
		s.ch <- prometheus.MustNewConstMetric(c.interfaceAdminUpDesc, prometheus.GaugeValue, boolToFloat(!iface.Disabled), iface.Name)
		if iface.Speed > 0 {
			s.ch <- prometheus.MustNewConstMetric(c.interfaceSpeedDesc, prometheus.GaugeValue, float64(iface.Speed), iface.Name)
		}
		if iface.HasDuplex {
			s.ch <- prometheus.MustNewConstMetric(c.interfaceDuplexDesc, prometheus.GaugeValue, boolToFloat(iface.FullDuplex), iface.Name)
		}
		s.ch <- prometheus.MustNewConstMetric(c.interfaceRxBytesDesc, prometheus.CounterValue, float64(iface.RxBytes), iface.Name)
		s.ch <- prometheus.MustNewConstMetric(c.interfaceTxBytesDesc, prometheus.CounterValue, float64(iface.TxBytes), iface.Name)
		s.ch <- prometheus.MustNewConstMetric(c.interfaceRxPacketsDesc, prometheus.CounterValue, float64(iface.RxPackets), iface.Name)
		s.ch <- prometheus.MustNewConstMetric(c.interfaceTxPacketsDesc, prometheus.CounterValue, float64(iface.TxPackets), iface.Name)
		s.ch <- prometheus.MustNewConstMetric(c.interfaceRxErrorsDesc, prometheus.CounterValue, float64(iface.RxErrors), iface.Name)
		s.ch <- prometheus.MustNewConstMetric(c.interfaceTxErrorsDesc, prometheus.CounterValue, float64(iface.TxErrors), iface.Name)
		s.ch <- prometheus.MustNewConstMetric(c.interfaceRxDropsDesc, prometheus.CounterValue, float64(iface.RxDrops), iface.Name)
		s.ch <- prometheus.MustNewConstMetric(c.interfaceTxDropsDesc, prometheus.CounterValue, float64(iface.TxDrops), iface.Name)
	}
}

func (c *MikrotikCollector) collectHealth(s *scrapeState) {
	health, err := c.client.GetSystemHealth()
	s.markErr("health", err)
	if err != nil || health == nil {
		return
	}
	if !health.Supported {
		s.supported["health"] = 0
		return
	}
	if health.Temperature != 0 {
		s.ch <- prometheus.MustNewConstMetric(c.temperatureDesc, prometheus.GaugeValue, health.Temperature, "cpu")
	}
	if health.BoardTemperature != 0 {
		s.ch <- prometheus.MustNewConstMetric(c.temperatureDesc, prometheus.GaugeValue, health.BoardTemperature, "board")
		s.ch <- prometheus.MustNewConstMetric(c.boardTemperatureDesc, prometheus.GaugeValue, health.BoardTemperature)
	}
	if health.Voltage != 0 {
		s.ch <- prometheus.MustNewConstMetric(c.voltageDesc, prometheus.GaugeValue, health.Voltage)
	}
	if health.Current != 0 {
		s.ch <- prometheus.MustNewConstMetric(c.currentDesc, prometheus.GaugeValue, health.Current)
	}
	if health.PowerConsumed != 0 {
		s.ch <- prometheus.MustNewConstMetric(c.powerConsumedDesc, prometheus.GaugeValue, health.PowerConsumed)
	}
	if health.FanSpeed != 0 {
		s.ch <- prometheus.MustNewConstMetric(c.fanSpeedDesc, prometheus.GaugeValue, float64(health.FanSpeed), "fan1")
	}
}

func (c *MikrotikCollector) collectBGPPeers(s *scrapeState) {
	s.supported["bgp"] = 1
	bgpStats, err := c.client.GetBGPPeerStats()
	s.markErr("bgp", err)
	if err != nil {
		return
	}
	for _, peer := range bgpStats {
		s.ch <- prometheus.MustNewConstMetric(c.bgpPeerInfoDesc, prometheus.GaugeValue, 1,
			peer.Name, peer.RoutingInstance, peer.RemoteAddress, peer.RemoteAS, peer.LocalAddress, peer.LocalRole, peer.RemoteRole, strconv.FormatBool(peer.Disabled),
		)
		s.ch <- prometheus.MustNewConstMetric(c.bgpPeerStateDesc, prometheus.GaugeValue, boolToFloat(peer.State == "established"), peer.Name, peer.State)
		s.ch <- prometheus.MustNewConstMetric(c.bgpPeerUptimeDesc, prometheus.GaugeValue, peer.Uptime.Seconds(), peer.Name)
		s.ch <- prometheus.MustNewConstMetric(c.bgpPeerPrefixCountDesc, prometheus.GaugeValue, float64(peer.PrefixCount), peer.Name)
		s.ch <- prometheus.MustNewConstMetric(c.bgpPeerUpdatesSentDesc, prometheus.CounterValue, float64(peer.UpdatesSent), peer.Name)
		s.ch <- prometheus.MustNewConstMetric(c.bgpPeerUpdatesRecvDesc, prometheus.CounterValue, float64(peer.UpdatesRecv), peer.Name)
		s.ch <- prometheus.MustNewConstMetric(c.bgpPeerWithdrawsSentDesc, prometheus.CounterValue, float64(peer.WithdrawsSent), peer.Name)
		s.ch <- prometheus.MustNewConstMetric(c.bgpPeerWithdrawsRecvDesc, prometheus.CounterValue, float64(peer.WithdrawsRecv), peer.Name)
	}
}

func (c *MikrotikCollector) collectPPPUsers(s *scrapeState) {
	s.supported["ppp"] = 1
	pppUsers, err := c.client.GetPPPActiveUsers()
	s.markErr("ppp", err)
	if err != nil {
		return
	}
	s.ch <- prometheus.MustNewConstMetric(c.pppActiveCountDesc, prometheus.GaugeValue, float64(len(pppUsers)))
	for _, user := range pppUsers {
		s.ch <- prometheus.MustNewConstMetric(c.pppUserInfoDesc, prometheus.GaugeValue, 1,
			user.Name, user.Service, user.CallerID, user.Address,
		)
		s.ch <- prometheus.MustNewConstMetric(c.pppUserUptimeDesc, prometheus.GaugeValue, user.Uptime.Seconds(), user.Name)
		s.ch <- prometheus.MustNewConstMetric(c.pppUserRxBytesDesc, prometheus.CounterValue, float64(user.RxBytes), user.Name)
		s.ch <- prometheus.MustNewConstMetric(c.pppUserTxBytesDesc, prometheus.CounterValue, float64(user.TxBytes), user.Name)
	}
}

func (c *MikrotikCollector) collectWirelessMetrics(s *scrapeState) {
	s.supported["wireless"] = 1

	wirelessInterfaces, err := c.client.FetchWirelessInterfaces()
	s.markErr("wireless", err)
	if err == nil && wirelessInterfaces != nil {
		for _, iface := range wirelessInterfaces {
			s.ch <- prometheus.MustNewConstMetric(c.wirelessInterfaceInfoDesc, prometheus.GaugeValue, 1,
				iface.Name, iface.SSID, strconv.Itoa(iface.Frequency),
			)
			if iface.SignalStrength != 0 {
				s.ch <- prometheus.MustNewConstMetric(c.wirelessInterfaceSignalStrengthDesc, prometheus.GaugeValue, float64(iface.SignalStrength), iface.Name)
			}
			if iface.TxRate > 0 {
				s.ch <- prometheus.MustNewConstMetric(c.wirelessInterfaceTxRateDesc, prometheus.GaugeValue, iface.TxRate, iface.Name)
			}
			if iface.RxRate > 0 {
				s.ch <- prometheus.MustNewConstMetric(c.wirelessInterfaceRxRateDesc, prometheus.GaugeValue, iface.RxRate, iface.Name)
			}
			if iface.HasNoiseFloor {
				s.ch <- prometheus.MustNewConstMetric(c.wirelessInterfaceNoiseFloorDesc, prometheus.GaugeValue, float64(iface.NoiseFloor), iface.Name)
			}
		}
	}

	wirelessClients, err := c.client.FetchWirelessClients()
	if err != nil {
		s.markErr("wireless", err)
		return
	}
	if wirelessClients == nil {
		return
	}
	clientCounts := make(map[string]int)
	for _, client := range wirelessClients {
		clientCounts[client.Interface]++
		s.ch <- prometheus.MustNewConstMetric(c.wirelessClientInfoDesc, prometheus.GaugeValue, 1,
			client.Interface, client.MacAddress,
		)
		if client.SignalStrength != 0 {
			s.ch <- prometheus.MustNewConstMetric(c.wirelessClientSignalStrengthDesc, prometheus.GaugeValue, float64(client.SignalStrength), client.Interface, client.MacAddress)
		}
		if client.TxCCQ != 0 {
			s.ch <- prometheus.MustNewConstMetric(c.wirelessClientTxCCQDesc, prometheus.GaugeValue, float64(client.TxCCQ), client.Interface, client.MacAddress)
		}
		if client.TxRateBps > 0 {
			s.ch <- prometheus.MustNewConstMetric(c.wirelessClientTxRateDesc, prometheus.GaugeValue, client.TxRateBps, client.Interface, client.MacAddress)
		}
		if client.RxRateBps > 0 {
			s.ch <- prometheus.MustNewConstMetric(c.wirelessClientRxRateDesc, prometheus.GaugeValue, client.RxRateBps, client.Interface, client.MacAddress)
		}
		if client.HasNoiseFloor {
			s.ch <- prometheus.MustNewConstMetric(c.wirelessClientNoiseFloorDesc, prometheus.GaugeValue, float64(client.NoiseFloor), client.Interface, client.MacAddress)
		}
		s.ch <- prometheus.MustNewConstMetric(c.wirelessClientUptimeDesc, prometheus.GaugeValue, client.Uptime.Seconds(), client.Interface, client.MacAddress)
	}
	for ifaceName, count := range clientCounts {
		s.ch <- prometheus.MustNewConstMetric(c.wirelessActiveClientsDesc, prometheus.GaugeValue, float64(count), ifaceName)
	}
}

func (c *MikrotikCollector) collectOSPFNeighbors(s *scrapeState) {
	s.supported["ospf"] = 1
	neighbors, err := c.client.GetOSPFNeighbors()
	s.markErr("ospf", err)
	if err != nil || neighbors == nil {
		return
	}
	for _, n := range neighbors {
		s.ch <- prometheus.MustNewConstMetric(c.ospfNeighborInfoDesc, prometheus.GaugeValue, 1,
			n.RouterID, n.Address, n.Interface, n.State,
		)
		s.ch <- prometheus.MustNewConstMetric(c.ospfNeighborStateDesc, prometheus.GaugeValue, n.StateValue, n.RouterID, n.Interface)
	}
}

func (c *MikrotikCollector) collectOpticsMetrics(s *scrapeState) {
	s.supported["optics"] = 1
	optics, err := c.client.GetTransceiverStats()
	s.markErr("optics", err)
	if err != nil {
		return
	}
	for _, t := range optics {
		if t.HasTemp {
			s.ch <- prometheus.MustNewConstMetric(c.transceiverTempDesc, prometheus.GaugeValue, t.Temperature, t.Interface)
		}
		if t.HasTxPower {
			s.ch <- prometheus.MustNewConstMetric(c.transceiverTxPowerDesc, prometheus.GaugeValue, t.TxPowerDbm, t.Interface)
		}
		if t.HasRxPower {
			s.ch <- prometheus.MustNewConstMetric(c.transceiverRxPowerDesc, prometheus.GaugeValue, t.RxPowerDbm, t.Interface)
		}
	}
}
