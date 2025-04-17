package mikrotik

import (
	"fmt"
	"log"
	"strconv"
	"strings"
)

// WirelessClient represents a connected wireless client.
type WirelessClient struct {
	Interface      string
	MacAddress     string
	SignalStrength int
	TxCCQ          int
	RxRate         string
	TxRate         string
	Uptime         string
}

// WirelessInterface represents wireless interface monitoring data.
type WirelessInterface struct {
	Name           string
	SSID           string
	Frequency      int
	SignalStrength int
	TxRate         float64
	RxRate         float64
}

func (c *Client) FetchWirelessClients() ([]WirelessClient, error) {
	reply, err := c.Run("/interface/wireless/registration-table/print", "=.proplist=interface,mac-address,signal-strength,tx-ccq,rx-rate,tx-rate,uptime")
	if err != nil {
		if strings.Contains(err.Error(), "no such command") || strings.Contains(err.Error(), "disabled") {
			log.Println("Wireless package might be disabled or not installed, skipping wireless client metrics.")
			return nil, nil
		}
		log.Printf("Error fetching wireless registration table: %v", err)
		return nil, fmt.Errorf("error fetching wireless registration table: %w", err)
	}

	clients := []WirelessClient{}
	for _, re := range reply.Re {
		mac := re.Map["mac-address"]
		if mac == "" {
			continue
		}

		signalStr := strings.Split(re.Map["signal-strength"], "@")[0]
		signal, _ := strconv.Atoi(signalStr)

		ccqStr := re.Map["tx-ccq"]
		ccq, _ := strconv.Atoi(ccqStr)

		client := WirelessClient{
			Interface:      re.Map["interface"],
			MacAddress:     mac,
			SignalStrength: signal,
			TxCCQ:          ccq,
			RxRate:         re.Map["rx-rate"],
			TxRate:         re.Map["tx-rate"],
			Uptime:         re.Map["uptime"],
		}
		clients = append(clients, client)
	}

	return clients, nil
}

func (c *Client) FetchWirelessInterfaces() ([]WirelessInterface, error) {
	ifListReply, err := c.Run("/interface/wireless/print", "=.proplist=.id,name")
	if err != nil {
		if strings.Contains(err.Error(), "no such command") || strings.Contains(err.Error(), "disabled") {
			log.Println("Wireless package might be disabled or not installed, skipping wireless interface metrics.")
			return nil, nil
		}
		log.Printf("Error fetching wireless interface list: %v", err)
		return nil, fmt.Errorf("error fetching wireless interface list: %w", err)
	}

	interfaces := []WirelessInterface{}
	for _, ifaceEntry := range ifListReply.Re {
		ifaceName := ifaceEntry.Map["name"]
		ifaceID := ifaceEntry.Map[".id"]
		if ifaceName == "" || ifaceID == "" {
			continue
		}

		monitorReply, err := c.RunArgs(
			[]string{
				"/interface/wireless/monitor",
				fmt.Sprintf("=numbers=%s", ifaceID),
				"=once=",
				"=.proplist=name,ssid,frequency,signal-strength,rate-set,tx-rate,rx-rate",
			},
		)

		if err != nil {
			log.Printf("Error monitoring wireless interface %s (%s): %v", ifaceName, ifaceID, err)
			continue
		}

		if len(monitorReply.Re) > 0 {
			monData := monitorReply.Re[0].Map

			freqStr := monData["frequency"]
			freq, _ := strconv.Atoi(freqStr)

			signalStr := strings.Split(monData["signal-strength"], "@")[0]
			signal, _ := strconv.Atoi(signalStr)

			txRateStr := monData["tx-rate"]
			txRate, _ := strconv.ParseFloat(txRateStr, 64)

			rxRateStr := monData["rx-rate"]
			rxRate, _ := strconv.ParseFloat(rxRateStr, 64)

			iface := WirelessInterface{
				Name:           ifaceName,
				SSID:           monData["ssid"],
				Frequency:      freq,
				SignalStrength: signal,
				TxRate:         txRate,
				RxRate:         rxRate,
			}
			interfaces = append(interfaces, iface)
		}
	}

	return interfaces, nil
}
