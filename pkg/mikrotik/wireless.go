package mikrotik

import (
	"fmt"
	"log"
	"strconv"
	"strings"
)

func (c *Client) FetchWirelessClients() ([]WirelessClient, error) {
	return mergeWirelessSources(
		"Wireless package might be disabled or not installed, skipping wireless client metrics.",
		func() ([]WirelessClient, error) {
			return c.fetchWirelessClientsPath("/interface/wireless/registration-table/print")
		},
		func() ([]WirelessClient, error) {
			return c.fetchWirelessClientsPath("/interface/wifi/registration-table/print")
		},
		func() ([]WirelessClient, error) {
			return c.fetchWirelessClientsPath("/interface/wifiwave2/registration-table/print")
		},
	)
}

func (c *Client) fetchWirelessClientsPath(path string) ([]WirelessClient, error) {
	reply, err := c.Run(path, "=.proplist=interface,mac-address,ssid,signal-strength,signal,tx-ccq,rx-ccq,rx-rate,tx-rate,uptime,noise-floor,signal-to-noise,ap")
	if err != nil {
		return nil, err
	}

	clients := make([]WirelessClient, 0, len(reply.Re))
	for _, re := range reply.Re {
		mac := re.Map["mac-address"]
		if mac == "" {
			continue
		}

		txCCQ, hasTxCCQ := parseOptionalInt(re.Map["tx-ccq"])
		rxCCQ, hasRxCCQ := parseOptionalInt(re.Map["rx-ccq"])
		rxRate, _ := ParseRateBps(re.Map["rx-rate"])
		txRate, _ := ParseRateBps(re.Map["tx-rate"])
		uptime, _ := parseMikrotikDuration(re.Map["uptime"])
		noise, hasNoise := parseOptionalInt(re.Map["noise-floor"])
		snr, hasSNR := parseOptionalInt(re.Map["signal-to-noise"])

		clients = append(clients, WirelessClient{
			Interface:      re.Map["interface"],
			MacAddress:     mac,
			SSID:           re.Map["ssid"],
			SignalStrength: parseSignalDBM(firstNonEmpty(re.Map, "signal-strength", "signal")),
			TxCCQ:          txCCQ,
			RxCCQ:          rxCCQ,
			RxRateBps:      rxRate,
			TxRateBps:      txRate,
			Uptime:         uptime,
			NoiseFloor:     noise,
			SNR:            snr,
			HasNoiseFloor:  hasNoise,
			HasTxCCQ:       hasTxCCQ,
			HasRxCCQ:       hasRxCCQ,
			HasSNR:         hasSNR,
		})
	}
	return clients, nil
}

func (c *Client) FetchWirelessInterfaces() ([]WirelessInterface, error) {
	return mergeWirelessSources(
		"Wireless package might be disabled or not installed, skipping wireless interface metrics.",
		c.fetchLegacyWirelessInterfaces,
		c.fetchWifiWave2Interfaces,
	)
}

func mergeWirelessSources[T any](skipLog string, fetchers ...func() ([]T, error)) ([]T, error) {
	var (
		all     []T
		lastErr error
		anyOK   bool
	)
	for _, fetch := range fetchers {
		items, err := fetch()
		if err != nil {
			lastErr = err
			if isUnsupportedCommand(err) {
				continue
			}
			// Keep already-fetched data from other packages; soft-fail secondary errors.
			if anyOK {
				log.Printf("wireless source error after partial success (keeping %d items): %v", len(all), err)
				continue
			}
			return nil, err
		}
		anyOK = true
		all = append(all, items...)
	}
	if !anyOK {
		if lastErr != nil {
			log.Println(skipLog)
		}
		return nil, nil
	}
	return all, nil
}

func (c *Client) fetchLegacyWirelessInterfaces() ([]WirelessInterface, error) {
	ifListReply, err := c.Run("/interface/wireless/print", "=.proplist=.id,name,mode,ssid,disabled,running")
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

		mode := ifaceEntry.Map["mode"]
		if parseBool(ifaceEntry.Map["disabled"]) {
			continue
		}
		running := parseBool(ifaceEntry.Map["running"])
		ssid := ifaceEntry.Map["ssid"]

		monitorReply, err := c.RunArgs([]string{
			"/interface/wireless/monitor",
			fmt.Sprintf("=numbers=%s", ifaceID),
			"=once=",
			"=.proplist=name,ssid,frequency,channel,signal-strength,tx-rate,rx-rate,noise-floor,signal-to-noise,status,bssid,tx-ccq,rx-ccq,overall-tx-ccq",
		})
		if err != nil {
			log.Printf("Error monitoring wireless interface %s (%s): %v", ifaceName, ifaceID, err)
			continue
		}
		if len(monitorReply.Re) == 0 {
			continue
		}
		iface := parseWirelessMonitor(ifaceName, mode, running, monitorReply.Re[0].Map)
		if iface.SSID == "" {
			iface.SSID = ssid
		}
		interfaces = append(interfaces, iface)
	}
	return interfaces, nil
}

func (c *Client) fetchWifiWave2Interfaces() ([]WirelessInterface, error) {
	paths := []struct {
		print, monitor string
	}{
		{"/interface/wifi/print", "/interface/wifi/monitor"},
		{"/interface/wifiwave2/print", "/interface/wifiwave2/monitor"},
	}
	var lastErr error
	for _, p := range paths {
		ifaces, err := c.fetchWifiInterfacesPath(p.print, p.monitor)
		if err == nil {
			return ifaces, nil
		}
		lastErr = err
		if !isUnsupportedCommand(err) {
			return nil, err
		}
	}
	return nil, lastErr
}

func (c *Client) fetchWifiInterfacesPath(printPath, monitorPath string) ([]WirelessInterface, error) {
	ifListReply, err := c.Run(printPath, "=.proplist=.id,name,configuration.mode,mode,configuration.ssid,ssid,disabled,running")
	if err != nil {
		// Fallback without nested props
		ifListReply, err = c.Run(printPath, "=.proplist=.id,name,mode,ssid,disabled,running")
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
		if parseBool(ifaceEntry.Map["disabled"]) {
			continue
		}
		if err := c.scrapeContext().Err(); err != nil {
			return interfaces, err
		}

		mode := firstNonEmpty(ifaceEntry.Map, "configuration.mode", "mode")
		ssid := firstNonEmpty(ifaceEntry.Map, "configuration.ssid", "ssid")
		running := parseBool(ifaceEntry.Map["running"])

		monitorReply, err := c.RunArgs([]string{
			monitorPath,
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
		iface := parseWirelessMonitor(ifaceName, mode, running, monitorReply.Re[0].Map)
		if iface.SSID == "" {
			iface.SSID = ssid
		}
		interfaces = append(interfaces, iface)
	}
	return interfaces, nil
}

func parseWirelessMonitor(name, mode string, running bool, mon map[string]string) WirelessInterface {
	freq, widthMHz, widthLabel := parseChannel(firstNonEmpty(mon, "channel", "frequency"))
	if freq == 0 {
		freq, _ = strconv.Atoi(firstNonEmpty(mon, "frequency", "channel"))
	}
	txRate, _ := ParseRateBps(firstNonEmpty(mon, "tx-rate", "tx-bitrate"))
	rxRate, _ := ParseRateBps(firstNonEmpty(mon, "rx-rate", "rx-bitrate"))
	noise, hasNoise := parseOptionalInt(mon["noise-floor"])
	snr, hasSNR := parseOptionalInt(mon["signal-to-noise"])
	txCCQ, hasTxCCQ := parseOptionalInt(firstNonEmpty(mon, "overall-tx-ccq", "tx-ccq"))
	rxCCQ, hasRxCCQ := parseOptionalInt(mon["rx-ccq"])

	role := wirelessRole(mode)
	status := firstNonEmpty(mon, "status", "state")
	bssid := firstNonEmpty(mon, "bssid", "ap-address")

	return WirelessInterface{
		Name:            name,
		SSID:            firstNonEmpty(mon, "ssid", "configuration.ssid"),
		Mode:            mode,
		Role:            role,
		Frequency:       freq,
		ChannelWidth:    widthLabel,
		ChannelWidthMHz: widthMHz,
		SignalStrength:  parseSignalDBM(firstNonEmpty(mon, "signal-strength", "signal")),
		TxRate:          txRate,
		RxRate:          rxRate,
		NoiseFloor:      noise,
		SNR:             snr,
		TxCCQ:           txCCQ,
		RxCCQ:           rxCCQ,
		BSSID:           bssid,
		Running:         running,
		Connected:       wirelessConnected(role, running, status, bssid),
		HasNoiseFloor:   hasNoise,
		HasSNR:          hasSNR,
		HasTxCCQ:        hasTxCCQ,
		HasRxCCQ:        hasRxCCQ,
	}
}

// wirelessRole maps RouterOS wireless/wifi mode strings to ap|station|unknown.
func wirelessRole(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "ap-bridge", "bridge", "ap", "wds-slave":
		return "ap"
	case "station", "station-bridge", "station-wds", "station-pseudobridge", "station-pseudobridge-clone":
		return "station"
	default:
		return "unknown"
	}
}

func wirelessConnected(role string, running bool, status, bssid string) bool {
	_ = bssid // retained for call-site compatibility; never treat alone as associated
	status = strings.ToLower(strings.TrimSpace(status))
	switch status {
	case "scanning", "searching-for-network", "inactive", "disabled":
		if role == "station" || role == "unknown" {
			return false
		}
	}
	associated := status == "connected-to-ess" || status == "authorized" || status == "connected"
	switch role {
	case "station":
		if associated {
			return true
		}
		// Some wifi builds omit status/state; running still means associated to an AP.
		if status == "" {
			return running
		}
		return false
	case "ap":
		return running
	default:
		if status == "running-ap" || status == "running" {
			return running
		}
		return associated || running
	}
}

// parseChannel extracts frequency (MHz) and channel width (MHz) from RouterOS
// channel strings such as:
//
//	"5825/20/ac(10dBm)", "5180/40-Ce/ac", "5180/20-Ceee/ac"  (legacy)
//	"5260/ax/Ceee", "2447/ax", "5600/ac/eCee"               (wifi / wifiwave2)
func parseChannel(raw string) (freqMHz, widthMHz int, widthLabel string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, 0, ""
	}
	if i := strings.IndexByte(raw, '('); i >= 0 {
		raw = strings.TrimSpace(raw[:i])
	}
	parts := strings.Split(raw, "/")
	freqMHz, _ = strconv.Atoi(strings.TrimSpace(parts[0]))
	if len(parts) < 2 {
		return freqMHz, 0, ""
	}

	tok2 := strings.TrimSpace(parts[1])
	if tok2 == "" {
		return freqMHz, 0, ""
	}

	// Wifi form: FREQ/ax|/ac|/be[/Ceee]
	if !isDigit(tok2[0]) {
		widthMHz = 20
		if len(parts) >= 3 {
			if ext := channelExtensionWidth(parts[2]); ext > 0 {
				widthMHz = ext
			}
		}
		return freqMHz, widthMHz, strconv.Itoa(widthMHz)
	}

	// Legacy form: FREQ/20|/40-Ce|/20-Ceee|/80/...
	num := tok2
	ext := ""
	for i, r := range tok2 {
		if r < '0' || r > '9' {
			num = tok2[:i]
			ext = tok2[i:]
			break
		}
	}
	if num == "" {
		return freqMHz, 0, ""
	}
	n, err := strconv.Atoi(num)
	if err != nil || n <= 0 {
		return freqMHz, 0, ""
	}
	widthMHz = n
	// "20-Ceee" → operational 80 MHz (four 20 MHz slots).
	if strings.HasPrefix(ext, "-") {
		if slots := channelExtensionWidth(ext[1:]); slots > widthMHz {
			widthMHz = slots
		}
	}
	return freqMHz, widthMHz, strconv.Itoa(widthMHz)
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

// channelExtensionWidth counts C/e channel letters (e.g. Ceee → 80, eC → 40).
func channelExtensionWidth(ext string) int {
	n := 0
	for _, r := range strings.ToLower(ext) {
		if r == 'c' || r == 'e' {
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return n * 20
}

func parseSignalDBM(raw string) int {
	n, _ := strconv.Atoi(strings.Split(raw, "@")[0])
	return n
}

func parseNoiseFloor(raw string) (int, bool) {
	return parseOptionalInt(raw)
}

func parseOptionalInt(raw string) (int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	n, err := strconv.Atoi(strings.Split(raw, "@")[0])
	if err != nil {
		return 0, false
	}
	return n, true
}
