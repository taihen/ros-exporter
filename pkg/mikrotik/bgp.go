package mikrotik

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
)

// GetBGPPeerStats fetches statistics for all configured BGP peers using the correct API command.
func (c *Client) GetBGPPeerStats() ([]BGPPeerStat, error) {
	// Try the new path first (RouterOS 7+)
	cmd := []string{
		"/routing/bgp/peer/print",
		"without-paging",
	}
	reply, err := c.Run(cmd...)

	// If the new path fails, try the old path (RouterOS 6.x)
	if err != nil && (strings.Contains(err.Error(), "no such command") || strings.Contains(err.Error(), "disabled")) {
		cmd = []string{
			"/ip/bgp/peer/print",
			"without-paging",
		}
		reply, err = c.Run(cmd...)
	}
	if err != nil {
		if strings.Contains(err.Error(), "no such command") || strings.Contains(err.Error(), "disabled") {
			log.Printf("BGP package/feature might be disabled on %s. Skipping BGP metrics.", c.Address)
			return []BGPPeerStat{}, nil
		}
		// Include the command in the error message for better debugging
		return nil, fmt.Errorf("failed to get BGP peer details using command %v: %w", cmd, err)
	}

	stats := make([]BGPPeerStat, 0, len(reply.Re))

	for _, re := range reply.Re {
		// Debug: Print all available fields for this BGP peer
		log.Printf("DEBUG: BGP peer fields available: %v", re.Map)

		name := re.Map["name"]
		if name == "" {
			log.Printf("Warning: Skipping BGP peer with empty name: %v", re.Map)
			continue
		}

		// Handle uptime - in RouterOS 6.48, uptime might be empty or in a different field
		var uptime time.Duration
		uptimeStr := re.Map["uptime"]
		if uptimeStr != "" {
			uptime, err = parseMikrotikDuration(uptimeStr)
			if err != nil {
				log.Printf("Warning: Could not parse BGP peer uptime '%s' for peer '%s': %v", uptimeStr, name, err)
			}
		} else {
			// Try alternative fields that might contain uptime in RouterOS 6.48
			// Check for established-for field which might be used in RouterOS 6.48
			if establishedFor, ok := re.Map["established-for"]; ok && establishedFor != "" {
				uptime, err = parseMikrotikDuration(establishedFor)
				if err != nil {
					log.Printf("Warning: Could not parse BGP peer established-for '%s' for peer '%s': %v", establishedFor, name, err)
				} else {
					log.Printf("Using established-for field for BGP peer '%s' uptime", name)
				}
			} else {
				// For RouterOS 6.48, we'll set uptime to 0 if not available
				log.Printf("BGP peer '%s' has empty uptime field, setting to 0", name)
				uptime = 0
			}
		}

		// Parse fields returned by /peer/print status
		// For RouterOS 6.48, some fields might have different names or be missing
		prefixCount := uint64(0)
		// Try different field names for prefix count
		prefixCountFields := []string{"prefix-count", "prefixes", "prefixes-count", "received-prefixes"}
		for _, field := range prefixCountFields {
			if prefixCountStr, ok := re.Map[field]; ok && prefixCountStr != "" {
				prefixCount, _ = strconv.ParseUint(prefixCountStr, 10, 64)
				log.Printf("Using field '%s' for BGP peer '%s' prefix count: %d", field, name, prefixCount)
				break
			}
		}

		updatesSent := uint64(0)
		// Try different field names for updates sent
		updatesSentFields := []string{"updates-sent", "sent-updates", "updates-out"}
		for _, field := range updatesSentFields {
			if updatesSentStr, ok := re.Map[field]; ok && updatesSentStr != "" {
				updatesSent, _ = strconv.ParseUint(updatesSentStr, 10, 64)
				log.Printf("Using field '%s' for BGP peer '%s' updates sent: %d", field, name, updatesSent)
				break
			}
		}

		updatesRecv := uint64(0)
		// Try different field names for updates received
		updatesRecvFields := []string{"updates-received", "received-updates", "updates-in"}
		for _, field := range updatesRecvFields {
			if updatesRecvStr, ok := re.Map[field]; ok && updatesRecvStr != "" {
				updatesRecv, _ = strconv.ParseUint(updatesRecvStr, 10, 64)
				log.Printf("Using field '%s' for BGP peer '%s' updates received: %d", field, name, updatesRecv)
				break
			}
		}

		withdrawsSent := uint64(0)
		// Try different field names for withdraws sent
		withdrawsSentFields := []string{"withdraws-sent", "sent-withdraws", "withdraws-out"}
		for _, field := range withdrawsSentFields {
			if withdrawsSentStr, ok := re.Map[field]; ok && withdrawsSentStr != "" {
				withdrawsSent, _ = strconv.ParseUint(withdrawsSentStr, 10, 64)
				log.Printf("Using field '%s' for BGP peer '%s' withdraws sent: %d", field, name, withdrawsSent)
				break
			}
		}

		withdrawsRecv := uint64(0)
		// Try different field names for withdraws received
		withdrawsRecvFields := []string{"withdraws-received", "received-withdraws", "withdraws-in"}
		for _, field := range withdrawsRecvFields {
			if withdrawsRecvStr, ok := re.Map[field]; ok && withdrawsRecvStr != "" {
				withdrawsRecv, _ = strconv.ParseUint(withdrawsRecvStr, 10, 64)
				log.Printf("Using field '%s' for BGP peer '%s' withdraws received: %d", field, name, withdrawsRecv)
				break
			}
		}

		// Get the state field, which might have different names in RouterOS 6.48
		state := ""
		stateFields := []string{"state", "connection-state", "status"}
		for _, field := range stateFields {
			if stateStr, ok := re.Map[field]; ok && stateStr != "" {
				state = stateStr
				log.Printf("Using field '%s' for BGP peer '%s' state: %s", field, name, state)
				break
			}
		}

		// Get the disabled field, which might have different names in RouterOS 6.48
		disabled := false
		disabledFields := []string{"disabled", "inactive"}
		for _, field := range disabledFields {
			if disabledStr, ok := re.Map[field]; ok && disabledStr != "" {
				disabled = parseBool(disabledStr)
				log.Printf("Using field '%s' for BGP peer '%s' disabled: %t", field, name, disabled)
				break
			}
		}

		stat := BGPPeerStat{
			Name:          name,
			Instance:      re.Map["instance"],
			RemoteAddress: re.Map["remote-address"],
			RemoteAS:      re.Map["remote-as"],
			LocalAddress:  re.Map["local-address"],
			LocalRole:     re.Map["local-role"],
			RemoteRole:    re.Map["remote-role"],
			State:         state,
			Uptime:        uptime,
			PrefixCount:   prefixCount,
			UpdatesSent:   updatesSent,
			UpdatesRecv:   updatesRecv,
			WithdrawsSent: withdrawsSent,
			WithdrawsRecv: withdrawsRecv,
			Disabled:      disabled,
		}
		stats = append(stats, stat)
	}

	return stats, nil
}
