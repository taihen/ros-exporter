package mikrotik

import (
	"fmt"
	"log"
	"strings"

	"github.com/go-routeros/routeros/v3/proto"
)

func (c *Client) GetInterfaceStats() ([]InterfaceStat, error) {
	initialReply, err := c.RunArgs([]string{"/interface/print", "without-paging", "=.proplist=name,type"})
	if err != nil {
		return nil, fmt.Errorf("failed to get initial interface names/types: %w", err)
	}

	stats := make([]InterfaceStat, 0, len(initialReply.Re))
	ifaceMap := make(map[string]*InterfaceStat)

	for _, re := range initialReply.Re {
		name := re.Map["name"]
		if name == "" || isPPPInterface(name, re.Map["type"]) {
			continue
		}
		ifaceType := re.Map["type"]
		stat := InterfaceStat{Name: name, Type: ifaceType}
		stats = append(stats, stat)
		ifaceMap[name] = &stats[len(stats)-1]
	}

	if len(stats) == 0 {
		return stats, nil
	}

	detailReply, detailErr := c.Run("/interface/print", "detail", "without-paging")
	if detailErr != nil {
		log.Printf("Warning: Failed to get detailed interface info for %s: %v", c.Address, detailErr)
	} else {
		for _, re := range detailReply.Re {
			stat := ifaceMap[re.Map["name"]]
			if stat == nil {
				continue
			}
			stat.Comment = re.Map["comment"]
			stat.MACAddress = re.Map["mac-address"]
			stat.Running = parseBool(re.Map["running"])
			stat.Disabled = parseBool(re.Map["disabled"])
		}
	}

	ethReply, ethErr := c.RunArgs([]string{
		"/interface/ethernet/print", "without-paging",
		"=.proplist=name,speed,full-duplex,running,disabled",
	})
	if ethErr == nil {
		for _, re := range ethReply.Re {
			stat := ifaceMap[re.Map["name"]]
			if stat == nil {
				continue
			}
			if speedStr := re.Map["speed"]; speedStr != "" {
				if bps, err := ParseLinkSpeedBps(speedStr); err == nil {
					stat.Speed = bps
				}
			}
			if fd, ok := re.Map["full-duplex"]; ok && fd != "" {
				stat.HasDuplex = true
				stat.FullDuplex = parseBool(fd)
			}
			if running, ok := re.Map["running"]; ok && running != "" {
				stat.Running = parseBool(running)
			}
			if disabled, ok := re.Map["disabled"]; ok && disabled != "" {
				stat.Disabled = parseBool(disabled)
			}
		}
	}

	statsReply, statsErr := c.RunArgs([]string{"/interface/print", "stats", "without-paging"})
	if statsErr != nil {
		log.Printf("Warning: '/interface/print stats' failed for %s: %v; trying combined proplist", c.Address, statsErr)
		combinedReply, combinedErr := c.RunArgs([]string{
			"/interface/print", "without-paging",
			"=.proplist=name,rx-byte,tx-byte,rx-packet,tx-packet,rx-error,tx-error,rx-drop,tx-drop",
		})
		if combinedErr != nil {
			return nil, fmt.Errorf("failed to get interface counters: %w", combinedErr)
		}
		fillInterfaceCounters(ifaceMap, combinedReply.Re)
		return stats, nil
	}

	fillInterfaceCounters(ifaceMap, statsReply.Re)
	return stats, nil
}

// isPPPInterface reports whether an interface is PPP-related (also matches "pppoe").
func isPPPInterface(name, ifaceType string) bool {
	return strings.Contains(strings.ToLower(ifaceType), "ppp") ||
		strings.Contains(strings.ToLower(name), "ppp")
}

func fillInterfaceCounters(ifaceMap map[string]*InterfaceStat, rows []*proto.Sentence) {
	for _, re := range rows {
		stat := ifaceMap[re.Map["name"]]
		if stat == nil {
			continue
		}
		assignCounter(re.Map, &stat.RxBytes, "rx-byte", "rx-bytes", "bytes-in")
		assignCounter(re.Map, &stat.TxBytes, "tx-byte", "tx-bytes", "bytes-out")
		assignCounter(re.Map, &stat.RxPackets, "rx-packet", "rx-packets", "packets-in")
		assignCounter(re.Map, &stat.TxPackets, "tx-packet", "tx-packets", "packets-out")
		assignCounter(re.Map, &stat.RxErrors, "rx-error", "rx-errors", "errors-in")
		assignCounter(re.Map, &stat.TxErrors, "tx-error", "tx-errors", "errors-out")
		assignCounter(re.Map, &stat.RxDrops, "rx-drop", "rx-drops", "drops-in")
		assignCounter(re.Map, &stat.TxDrops, "tx-drop", "tx-drops", "drops-out")
	}
}

func assignCounter(m map[string]string, dst *uint64, fields ...string) {
	for _, field := range fields {
		if v := m[field]; v != "" {
			if n, err := parseBytes(v); err == nil {
				*dst = n
				return
			}
		}
	}
}
