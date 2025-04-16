package mikrotik

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	// Removed duplicate import "github.com/go-routeros/routeros"
)

// WirelessClient represents a connected wireless client.
type WirelessClient struct {
	Interface      string
	MacAddress     string
	SignalStrength int
	TxCCQ          int
	RxRate         string // Keep as string for now due to complex format e.g., "26Mbps-20MHz/1S"
	TxRate         string // Keep as string
	Uptime         string // Keep as string e.g., "3h37m8s"
}

// WirelessInterface represents wireless interface monitoring data.
type WirelessInterface struct {
	Name           string
	SSID           string
	Frequency      int
	SignalStrength int // For station mode primarily
	TxRate         float64 // bps
	RxRate         float64 // bps
	// Add other relevant fields from 'monitor' command as needed
}

// FetchWirelessClients retrieves the list of connected wireless clients (AP mode).
// This is now a method on the mikrotik.Client struct.
func (c *Client) FetchWirelessClients() ([]WirelessClient, error) {
	reply, err := c.Run("/interface/wireless/registration-table/print", "=.proplist=interface,mac-address,signal-strength,tx-ccq,rx-rate,tx-rate,uptime")
	if err != nil {
		// Check if the error indicates wireless package is not enabled or no wireless interfaces exist
		// TODO: Add more specific error handling if possible based on go-routeros errors
		if strings.Contains(err.Error(), "no such command") || strings.Contains(err.Error(), "disabled") {
			log.Println("Wireless package might be disabled or not installed, skipping wireless client metrics.")
			return nil, nil // Not a fatal error, just no wireless data
		}
		log.Printf("Error fetching wireless registration table: %v", err)
		return nil, fmt.Errorf("error fetching wireless registration table: %w", err)
	}

	clients := []WirelessClient{}
	for _, re := range reply.Re {
		mac := re.Map["mac-address"]
		if mac == "" {
			continue // Skip entries without MAC address
		}

		// Signal strength often includes rate info like "-74@6Mbps", parse only the dBm value
		signalStr := strings.Split(re.Map["signal-strength"], "@")[0]
		signal, _ := strconv.Atoi(signalStr) // Ignore error, default to 0 if parsing fails

		ccqStr := re.Map["tx-ccq"]
		ccq, _ := strconv.Atoi(ccqStr) // Ignore error, default to 0

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

// FetchWirelessInterfaces retrieves monitoring data for wireless interfaces.
// This is now a method on the mikrotik.Client struct.
func (c *Client) FetchWirelessInterfaces() ([]WirelessInterface, error) {
	// First, get the list of wireless interfaces
	ifListReply, err := c.Run("/interface/wireless/print", "=.proplist=.id,name")
	if err != nil {
		if strings.Contains(err.Error(), "no such command") || strings.Contains(err.Error(), "disabled") {
			log.Println("Wireless package might be disabled or not installed, skipping wireless interface metrics.")
			return nil, nil // Not a fatal error
		}
		log.Printf("Error fetching wireless interface list: %v", err)
		return nil, fmt.Errorf("error fetching wireless interface list: %w", err)
	}

	interfaces := []WirelessInterface{}
	for _, ifaceEntry := range ifListReply.Re {
		ifaceName := ifaceEntry.Map["name"]
		ifaceID := ifaceEntry.Map[".id"] // Use .id (internal number) for monitor command
		if ifaceName == "" || ifaceID == "" {
			continue
		}

		// Use 'monitor' command with the interface number (.id)
		// 'once' ensures the command returns immediately after collecting data once.
		// Use the client's RunArgs method wrapper
		monitorReply, err := c.RunArgs(
			[]string{
				"/interface/wireless/monitor",
				fmt.Sprintf("=numbers=%s", ifaceID), // Use internal ID
				"=once=", // Crucial for non-blocking call
				"=.proplist=name,ssid,frequency,signal-strength,rate-set,tx-rate,rx-rate", // Add more fields as needed
			},
		)

		if err != nil {
			// Monitor command might fail if interface is down, etc. Log but continue.
			log.Printf("Error monitoring wireless interface %s (%s): %v", ifaceName, ifaceID, err)
			continue
		}

		// Monitor command returns a single reply sentence
		if len(monitorReply.Re) > 0 {
			monData := monitorReply.Re[0].Map

			freqStr := monData["frequency"]
			freq, _ := strconv.Atoi(freqStr)

			signalStr := strings.Split(monData["signal-strength"], "@")[0] // Handle potential "@rate" suffix
			signal, _ := strconv.Atoi(signalStr)

			// Rates might be returned in bps directly by 'monitor' unlike reg-table
			txRateStr := monData["tx-rate"]
			txRate, _ := strconv.ParseFloat(txRateStr, 64)

			rxRateStr := monData["rx-rate"]
			rxRate, _ := strconv.ParseFloat(rxRateStr, 64)

			iface := WirelessInterface{
				Name:           ifaceName, // Use the actual name for labeling
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
