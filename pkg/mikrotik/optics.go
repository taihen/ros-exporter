package mikrotik

import (
	"fmt"
	"log"
	"strconv"
	"strings"
)

func (c *Client) GetTransceiverStats() ([]TransceiverStat, error) {
	reply, err := c.RunArgs([]string{
		"/interface/ethernet/print", "without-paging",
		"=.proplist=name",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list ethernet interfaces for optics: %w", err)
	}

	stats := make([]TransceiverStat, 0)
	for _, re := range reply.Re {
		name := re.Map["name"]
		if name == "" {
			continue
		}
		if err := c.scrapeContext().Err(); err != nil {
			return stats, err
		}

		mon, err := c.RunArgs([]string{
			"/interface/ethernet/monitor",
			"=.id=" + name,
			"=once=",
		})
		if err != nil {
			mon, err = c.RunArgs([]string{
				"/interface/ethernet/monitor",
				"=numbers=" + name,
				"=once=",
			})
		}
		if err != nil || len(mon.Re) == 0 {
			continue
		}

		m := mon.Re[0].Map
		st := TransceiverStat{Interface: name}

		if f, ok := parseFloatField(m, "C ", "sfp-temperature", "temperature"); ok {
			st.Temperature = f
			st.HasTemp = true
		}
		if f, ok := parseFloatField(m, "dBm ", "sfp-tx-power", "tx-power"); ok {
			st.TxPowerDbm = f
			st.HasTxPower = true
		}
		if f, ok := parseFloatField(m, "dBm ", "sfp-rx-power", "rx-power"); ok {
			st.RxPowerDbm = f
			st.HasRxPower = true
		}

		if st.HasTemp || st.HasTxPower || st.HasRxPower {
			stats = append(stats, st)
		}
	}

	if len(stats) == 0 {
		log.Printf("No transceiver optics data on %s", c.Address)
	}
	return stats, nil
}

func parseFloatField(m map[string]string, trimRight string, keys ...string) (float64, bool) {
	v := firstNonEmpty(m, keys...)
	if v == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(strings.TrimRight(v, trimRight), 64)
	if err != nil {
		return 0, false
	}
	return f, true
}
