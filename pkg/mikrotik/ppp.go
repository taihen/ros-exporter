package mikrotik

import (
	"fmt"
	"log"
	"strconv"
	"strings"
)

// GetPPPActiveUsers fetches statistics for all active PPP users.
func (c *Client) GetPPPActiveUsers() ([]PPPUserStat, error) {
	reply, err := c.Run("/ppp/active/print", "without-paging")
	if err != nil {
		if strings.Contains(err.Error(), "no such command") || strings.Contains(err.Error(), "disabled") {
			log.Printf("PPP feature might be disabled on %s. Skipping PPP metrics.", c.Address)
			return []PPPUserStat{}, nil
		}
		return nil, fmt.Errorf("failed to get active PPP users: %w", err)
	}

	stats := make([]PPPUserStat, 0, len(reply.Re))

	for _, re := range reply.Re {
		name := re.Map["name"]
		if name == "" {
			log.Printf("Skipping PPP user with empty name: %v", re.Map)
			continue
		}

		uptime, err := parseMikrotikDuration(re.Map["uptime"])
		if err != nil {
			uptime = 0
			log.Printf("Could not parse uptime for user '%s': %v", name, err)
		}

		rxBytes := uint64(0)
		if value, ok := re.Map["bytes-in"]; ok && value != "" {
			if bytes, err := strconv.ParseUint(value, 10, 64); err == nil {
				rxBytes = bytes
			}
		}

		txBytes := uint64(0)
		if value, ok := re.Map["bytes-out"]; ok && value != "" {
			if bytes, err := strconv.ParseUint(value, 10, 64); err == nil {
				txBytes = bytes
			}
		}

		stat := PPPUserStat{
			Name:      name,
			Service:   re.Map["service"],
			CallerID:  re.Map["caller-id"],
			Address:   re.Map["address"],
			Uptime:    uptime,
			UptimeStr: re.Map["uptime"],
			RxBytes:   rxBytes,
			TxBytes:   txBytes,
		}
		stats = append(stats, stat)
	}

	return stats, nil
}
