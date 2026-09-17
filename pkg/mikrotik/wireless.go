package mikrotik

import (
	"fmt"
	"log"
	"strconv"
	"strings"
)

func (c *Client) FetchWirelessClients() ([]WirelessClient, error) {
	paths := []string{
		"/interface/wireless/registration-table/print",
		"/interface/wifi/registration-table/print", // wifiwave2 / ROS7 wifi package
	}
	var lastErr error
	for _, path := range paths {
		clients, err := c.fetchWirelessClientsPath(path)
		if err == nil {
			return clients, nil
		}
		lastErr = err
		if !isUnsupportedCommand(err) {
			return nil, err
		}
	}
	if lastErr != nil {
		log.Println("Wireless package might be disabled or not installed, skipping wireless client metrics.")
	}
	return nil, nil
}

func (c *Client) fetchWirelessClientsPath(path string) ([]WirelessClient, error) {
	reply, err := c.Run(path, "=.proplist=interface,mac-address,signal-strength,tx-ccq,rx-rate,tx-rate,uptime,noise-floor,signal-to-noise")
	if err != nil {
		return nil, err
	}

	clients := make([]WirelessClient, 0, len(reply.Re))
	for _, re := range reply.Re {
		mac := re.Map["mac-address"]
		if mac == "" {
			continue
		}

		ccq, _ := strconv.Atoi(re.Map["tx-ccq"])

		rxRate, _ := ParseRateBps(re.Map["rx-rate"])
		txRate, _ := ParseRateBps(re.Map["tx-rate"])
		uptime, _ := parseMikrotikDuration(re.Map["uptime"])
		noise, hasNoise := parseNoiseFloor(firstNonEmpty(re.Map, "noise-floor", "signal-to-noise"))

		clients = append(clients, WirelessClient{
			Interface:      re.Map["interface"],
			MacAddress:     mac,
			SignalStrength: parseSignalDBM(re.Map["signal-strength"]),
			TxCCQ:          ccq,
			RxRateBps:      rxRate,
			TxRateBps:      txRate,
			Uptime:         uptime,
			NoiseFloor:     noise,
			HasNoiseFloor:  hasNoise,
		})
	}
	return clients, nil
}

func (c *Client) FetchWirelessInterfaces() ([]WirelessInterface, error) {
	ifaces, err := c.fetchLegacyWirelessInterfaces()
	if err == nil && len(ifaces) > 0 {
		return ifaces, nil
	}
	if err != nil && !isUnsupportedCommand(err) {
		return nil, err
	}

	ifaces, err = c.fetchWifiWave2Interfaces()
	if err != nil {
		if isUnsupportedCommand(err) {
			log.Println("Wireless package might be disabled or not installed, skipping wireless interface metrics.")
			return nil, nil
		}
		return nil, err
	}
	return ifaces, nil
}

func (c *Client) fetchLegacyWirelessInterfaces() ([]WirelessInterface, error) {
	ifListReply, err := c.Run("/interface/wireless/print", "=.proplist=.id,name")
	if err != nil {
		return nil, err
	}

	interfaces := make([]WirelessInterface, 0, len(ifListReply.Re))
	for _, ifaceEntry := range ifListReply.Re {
		ifaceName := ifaceEntry.Map["name"]
		ifaceID := ifaceEntry.Map[".id"]
		if ifaceName == "" || ifaceID == "" {
			continue
		}
		if err := c.scrapeContext().Err(); err != nil {
			return interfaces, err
		}

		monitorReply, err := c.RunArgs([]string{
			"/interface/wireless/monitor",
			fmt.Sprintf("=numbers=%s", ifaceID),
			"=once=",
			"=.proplist=name,ssid,frequency,signal-strength,tx-rate,rx-rate,noise-floor",
		})
		if err != nil {
			log.Printf("Error monitoring wireless interface %s (%s): %v", ifaceName, ifaceID, err)
			continue
		}
		if len(monitorReply.Re) == 0 {
			continue
		}
		interfaces = append(interfaces, parseWirelessMonitor(ifaceName, monitorReply.Re[0].Map))
	}
	return interfaces, nil
}

func (c *Client) fetchWifiWave2Interfaces() ([]WirelessInterface, error) {
	ifListReply, err := c.Run("/interface/wifi/print", "=.proplist=.id,name,configuration.ssid,configuration")
	if err != nil {
		// Fallback without nested props
		ifListReply, err = c.Run("/interface/wifi/print", "=.proplist=.id,name")
		if err != nil {
			return nil, err
		}
	}

	interfaces := make([]WirelessInterface, 0, len(ifListReply.Re))
	for _, ifaceEntry := range ifListReply.Re {
		ifaceName := ifaceEntry.Map["name"]
		ifaceID := ifaceEntry.Map[".id"]
		if ifaceName == "" || ifaceID == "" {
			continue
		}
		if err := c.scrapeContext().Err(); err != nil {
			return interfaces, err
		}

		ssid := firstNonEmpty(ifaceEntry.Map, "configuration.ssid", "ssid")

		monitorReply, err := c.RunArgs([]string{
			"/interface/wifi/monitor",
			fmt.Sprintf("=numbers=%s", ifaceID),
			"=once=",
		})
		if err != nil {
			log.Printf("Error monitoring wifi interface %s (%s): %v", ifaceName, ifaceID, err)
			continue
		}
		if len(monitorReply.Re) == 0 {
			continue
		}
		mon := monitorReply.Re[0].Map
		iface := parseWirelessMonitor(ifaceName, mon)
		if iface.SSID == "" {
			iface.SSID = ssid
		}
		interfaces = append(interfaces, iface)
	}
	return interfaces, nil
}

func parseWirelessMonitor(name string, mon map[string]string) WirelessInterface {
	freq, _ := strconv.Atoi(firstNonEmpty(mon, "frequency", "channel"))
	txRate, _ := ParseRateBps(firstNonEmpty(mon, "tx-rate", "tx-bitrate"))
	rxRate, _ := ParseRateBps(firstNonEmpty(mon, "rx-rate", "rx-bitrate"))
	noise, hasNoise := parseNoiseFloor(mon["noise-floor"])

	return WirelessInterface{
		Name:           name,
		SSID:           firstNonEmpty(mon, "ssid", "configuration.ssid"),
		Frequency:      freq,
		SignalStrength: parseSignalDBM(firstNonEmpty(mon, "signal-strength", "signal")),
		TxRate:         txRate,
		RxRate:         rxRate,
		NoiseFloor:     noise,
		HasNoiseFloor:  hasNoise,
	}
}

func parseSignalDBM(raw string) int {
	n, _ := strconv.Atoi(strings.Split(raw, "@")[0])
	return n
}

func parseNoiseFloor(raw string) (int, bool) {
	if raw == "" {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(strings.Split(raw, "@")[0]))
	if err != nil {
		return 0, false
	}
	return n, true
}
