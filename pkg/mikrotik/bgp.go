package mikrotik

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
)

func (c *Client) GetBGPPeerStats() ([]BGPPeerStat, error) {
	cmd := []string{
		"/routing/bgp/peer/print",
		"without-paging",
	}
	reply, err := c.Run(cmd...)

	if err != nil && (strings.Contains(err.Error(), "no such command") || strings.Contains(err.Error(), "disabled")) {
		cmd = []string{
			"/ip/bgp/peer/print",
			"without-paging",
		}
		reply, err = c.Run(cmd...)
	}
	if err != nil {
		if strings.Contains(err.Error(), "no such command") || strings.Contains(err.Error(), "disabled")) {
			log.Printf("BGP package/feature might be disabled on %s. Skipping BGP metrics.", c.Address)
			return []BGPPeerStat{}, nil
		}
		return nil, fmt.Errorf("failed to get BGP peer details using command %v: %w", cmd, err)
	}

	stats := make([]BGPPeerStat, 0, len(reply.Re))

	for _, re := range reply.Re {
		name := re.Map["name"]
		if name == "" {
			log.Printf("Warning: Skipping BGP peer with empty name: %v", re.Map)
			continue
		}

		uptime := time.Duration(0)
		uptimeStr := re.Map["uptime"]
		if uptimeStr != "" {
			uptime, err = parseMikrotikDuration(uptimeStr)
			if err != nil {
				log.Printf("Warning: Could not parse BGP peer uptime '%s' for peer '%s': %v", uptimeStr, name, err)
			}
		} else if establishedFor, ok := re.Map["established-for"]; ok && establishedFor != "" {
			uptime, err = parseMikrotikDuration(establishedFor)
			if err != nil {
				log.Printf("Warning: Could not parse BGP peer established-for '%s' for peer '%s': %v", establishedFor, name, err)
			}
		}

		prefixCount := uint64(0)
		prefixCountFields := []string{"prefix-count", "prefixes", "prefixes-count", "received-prefixes"}
		for _, field := range prefixCountFields {
			if prefixCountStr, ok := re.Map[field]; ok && prefixCountStr != "" {
				prefixCount, _ = strconv.ParseUint(prefixCountStr, 10, 64)
				break
			}
		}

		updatesSent := uint64(0)
		updatesSentFields := []string{"updates-sent", "sent-updates", "updates-out"}
		for _, field := range updatesSentFields {
			if updatesSentStr, ok := re.Map[field]; ok && updatesSentStr != "" {
				updatesSent, _ = strconv.ParseUint(updatesSentStr, 10, 64)
				break
			}
		}

		updatesRecv := uint64(0)
		updatesRecvFields := []string{"updates-received", "received-updates", "updates-in"}
		for _, field := range updatesRecvFields {
			if updatesRecvStr, ok := re.Map[field]; ok && updatesRecvStr != "" {
				updatesRecv, _ = strconv.ParseUint(updatesRecvStr, 10, 64)
				break
			}
		}

		withdrawsSent := uint64(0)
		withdrawsSentFields := []string{"withdraws-sent", "sent-withdraws", "withdraws-out"}
		for _, field := range withdrawsSentFields {
			if withdrawsSentStr, ok := re.Map[field]; ok && withdrawsSentStr != "" {
				withdrawsSent, _ = strconv.ParseUint(withdrawsSentStr, 10, 64)
				break
			}
		}

		withdrawsRecv := uint64(0)
		withdrawsRecvFields := []string{"withdraws-received", "received-withdraws", "withdraws-in"}
		for _, field := range withdrawsRecvFields {
			if withdrawsRecvStr, ok := re.Map[field]; ok && withdrawsRecvStr != "" {
				withdrawsRecv, _ = strconv.ParseUint(withdrawsRecvStr, 10, 64)
				break
			}
		}

		state := ""
		stateFields := []string{"state", "connection-state", "status"}
		for _, field := range stateFields {
			if stateStr, ok := re.Map[field]; ok && stateStr != "" {
				state = stateStr
				break
			}
		}

		disabled := false
		disabledFields := []string{"disabled", "inactive"}
		for _, field := range disabledFields {
			if disabledStr, ok := re.Map[field]; ok && disabledStr != "" {
				disabled = parseBool(disabledStr)
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
