package mikrotik

import (
	"fmt"
	"log"
	"strconv"
	"time"
)

func (c *Client) GetBGPPeerStats() ([]BGPPeerStat, error) {
	cmd := []string{"/routing/bgp/peer/print", "without-paging"}
	reply, err := c.Run(cmd...)

	if err != nil && isUnsupportedCommand(err) {
		cmd = []string{"/ip/bgp/peer/print", "without-paging"}
		reply, err = c.Run(cmd...)
	}
	if err != nil {
		if isUnsupportedCommand(err) {
			log.Printf("BGP package/feature might be disabled on %s. Skipping BGP metrics.", c.Address)
			return []BGPPeerStat{}, nil
		}
		return nil, fmt.Errorf("failed to get BGP peer details using command %v: %w", cmd, err)
	}

	stats := make([]BGPPeerStat, 0, len(reply.Re))
	for _, re := range reply.Re {
		name := re.Map["name"]
		if name == "" {
			continue
		}

		uptime := time.Duration(0)
		uptimeStr := firstNonEmpty(re.Map, "uptime", "established-for")
		if uptimeStr != "" {
			var parseErr error
			uptime, parseErr = parseMikrotikDuration(uptimeStr)
			if parseErr != nil {
				log.Printf("Warning: Could not parse BGP peer uptime '%s' for peer '%s': %v", uptimeStr, name, parseErr)
			}
		}

		parseU64 := func(fields ...string) uint64 {
			for _, field := range fields {
				if v, ok := re.Map[field]; ok && v != "" {
					n, _ := strconv.ParseUint(v, 10, 64)
					return n
				}
			}
			return 0
		}

		state := firstNonEmpty(re.Map, "state", "connection-state", "status")
		disabled := false
		if d := firstNonEmpty(re.Map, "disabled", "inactive"); d != "" {
			disabled = parseBool(d)
		}

		stats = append(stats, BGPPeerStat{
			Name:            name,
			RoutingInstance: firstNonEmpty(re.Map, "instance", "routing-table"),
			RemoteAddress:   re.Map["remote-address"],
			RemoteAS:        re.Map["remote-as"],
			LocalAddress:    re.Map["local-address"],
			LocalRole:       re.Map["local-role"],
			RemoteRole:      re.Map["remote-role"],
			State:           state,
			Uptime:          uptime,
			PrefixCount:     parseU64("prefix-count", "prefixes", "prefixes-count", "received-prefixes"),
			UpdatesSent:     parseU64("updates-sent", "sent-updates", "updates-out"),
			UpdatesRecv:     parseU64("updates-received", "received-updates", "updates-in"),
			WithdrawsSent:   parseU64("withdraws-sent", "sent-withdraws", "withdraws-out"),
			WithdrawsRecv:   parseU64("withdraws-received", "received-withdraws", "withdraws-in"),
			Disabled:        disabled,
		})
	}
	return stats, nil
}
