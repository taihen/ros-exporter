package mikrotik

import (
	"fmt"
	"log"
	"strconv"
	"strings"
)

func (c *Client) GetOSPFNeighbors() ([]OSPFNeighborStat, error) {
	paths := [][]string{
		{"/routing/ospf/neighbor/print", "without-paging"},
		{"/routing/ospf/v3/neighbor/print", "without-paging"},
		{"/routing/ospf/v2/neighbor/print", "without-paging"},
	}

	var lastErr error
	for _, cmd := range paths {
		reply, err := c.Run(cmd...)
		if err != nil {
			lastErr = err
			if isUnsupportedCommand(err) {
				continue
			}
			return nil, fmt.Errorf("failed to get OSPF neighbors via %v: %w", cmd, err)
		}

		stats := make([]OSPFNeighborStat, 0, len(reply.Re))
		for _, re := range reply.Re {
			state := strings.ToLower(firstNonEmpty(re.Map, "state", "adjacency"))
			val := 0.0
			if strings.Contains(state, "full") {
				val = 1.0
			}
			prio, _ := strconv.ParseUint(re.Map["priority"], 10, 64)
			stats = append(stats, OSPFNeighborStat{
				RouterID:   firstNonEmpty(re.Map, "router-id", "routerID"),
				Address:    firstNonEmpty(re.Map, "address", "neighbor-address"),
				Interface:  firstNonEmpty(re.Map, "interface", "iface"),
				State:      state,
				StateValue: val,
				Priority:   prio,
			})
		}
		return stats, nil
	}

	if lastErr != nil && isUnsupportedCommand(lastErr) {
		log.Printf("OSPF not available on %s", c.Address)
		return nil, nil
	}
	return nil, lastErr
}
