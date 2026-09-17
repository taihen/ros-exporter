package mikrotik

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
)

func (c *Client) GetSystemResources() (*SystemResource, error) {
	reply, err := c.Run("/system/resource/print")
	if err != nil {
		return nil, fmt.Errorf("failed to get system resources: %w", err)
	}
	if len(reply.Re) == 0 {
		return nil, errors.New("no system resource data received")
	}
	res := reply.Re[0].Map

	uptime := parseOrWarn("uptime", res["uptime"], parseMikrotikDuration)
	freeMem := parseOrWarn("free-memory", res["free-memory"], parseBytes)
	totalMem := parseOrWarn("total-memory", res["total-memory"], parseBytes)
	cpuLoad := parseOrWarn("cpu-load", res["cpu-load"], parseUint)
	freeHDDSpaceKiB := parseOrWarn("free-hdd-space", res["free-hdd-space"], parseBytes)
	totalHDDSpaceKiB := parseOrWarn("total-hdd-space", res["total-hdd-space"], parseBytes)

	return &SystemResource{
		Uptime:        uptime,
		FreeMemory:    freeMem,
		TotalMemory:   totalMem,
		CPULoad:       cpuLoad,
		FreeHDDSpace:  freeHDDSpaceKiB * 1024,
		TotalHDDSpace: totalHDDSpaceKiB * 1024,
		BoardName:     res["board-name"],
		Model:         res["model"],
		SerialNumber:  res["serial-number"],
	}, nil
}

func (c *Client) GetRouterboard() (*Routerboard, error) {
	reply, err := c.Run("/system/routerboard/print")
	if err != nil {
		return nil, fmt.Errorf("failed to get routerboard info: %w", err)
	}
	if len(reply.Re) == 0 {
		return nil, errors.New("no routerboard data received")
	}
	rb := reply.Re[0].Map

	return &Routerboard{
		BoardName:       rb["board-name"],
		Model:           rb["model"],
		SerialNumber:    rb["serial-number"],
		FirmwareType:    rb["firmware-type"],
		FactoryFirmware: rb["factory-firmware"],
		CurrentFirmware: rb["current-firmware"],
		UpgradeFirmware: rb["upgrade-firmware"],
	}, nil
}

func (c *Client) GetSystemHealth() (*SystemHealth, error) {
	reply, err := c.Run("/system/health/print")
	if err != nil {
		if isUnsupportedCommand(err) {
			log.Printf("Info: /system/health/print not available on %s", c.Address)
			return &SystemHealth{Supported: false}, nil
		}
		return nil, fmt.Errorf("failed to get system health: %w", err)
	}

	if len(reply.Re) == 0 {
		log.Printf("Warning: No system health data received from %s.", c.Address)
		return &SystemHealth{Supported: false}, nil
	}

	// RouterOS 7 often returns one row per sensor (name/value). ROS6 may use a flat map.
	health := &SystemHealth{Supported: true}
	flat := reply.Re[0].Map

	if _, hasName := flat["name"]; hasName || len(reply.Re) > 1 {
		for _, re := range reply.Re {
			name := strings.ToLower(re.Map["name"])
			valStr := re.Map["value"]
			if valStr == "" {
				valStr = re.Map["temperature"]
			}
			applyHealthValue(health, name, valStr)
		}
	} else {
		applyHealthFlat(health, flat)
	}

	return health, nil
}

func parseOrWarn[T any](field, raw string, parse func(string) (T, error)) T {
	var zero T
	v, err := parse(raw)
	if err != nil {
		log.Printf("Warning: Could not parse %s '%s': %v", field, raw, err)
		return zero
	}
	return v
}

func parseUint(s string) (uint64, error) {
	return strconv.ParseUint(s, 10, 64)
}

func applyHealthFlat(health *SystemHealth, m map[string]string) {
	health.Temperature = healthFloat(m, "temperature")
	health.BoardTemperature = healthFloat(m, "board-temperature")
	if health.BoardTemperature == 0 {
		health.BoardTemperature = healthFloat(m, "cpu-temperature")
	}
	if health.Temperature == 0 && health.BoardTemperature != 0 && m["temperature"] == "" && m["cpu-temperature"] != "" {
		health.Temperature = health.BoardTemperature
	}
	health.Voltage = healthFloat(m, "voltage")
	health.Current = healthFloat(m, "current")
	health.PowerConsumed = healthFloat(m, "power-consumption")
	health.FanSpeed = healthUint(m, "fan1-speed")
}

func healthFloat(m map[string]string, key string) float64 {
	valStr := m[key]
	if valStr == "" {
		return 0
	}
	val, err := strconv.ParseFloat(strings.TrimRight(valStr, "CVW RPM"), 64)
	if err != nil {
		return 0
	}
	return val
}

func healthUint(m map[string]string, key string) uint64 {
	valStr := m[key]
	if valStr == "" {
		return 0
	}
	val, err := strconv.ParseUint(strings.TrimRight(valStr, " RPM"), 10, 64)
	if err != nil {
		return 0
	}
	return val
}

func applyHealthValue(health *SystemHealth, name, valStr string) {
	if valStr == "" {
		return
	}
	cleaned := strings.TrimSpace(strings.TrimRight(valStr, "CVW°cC RPM%"))
	f, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return
	}
	switch {
	case name == "temperature" || name == "cpu-temperature" || strings.Contains(name, "cpu"):
		if health.Temperature == 0 {
			health.Temperature = f
		}
	case name == "board-temperature" || strings.Contains(name, "board"):
		health.BoardTemperature = f
	case strings.Contains(name, "voltage"):
		health.Voltage = f
	case name == "current" || strings.Contains(name, "current"):
		health.Current = f
	case strings.Contains(name, "power"):
		health.PowerConsumed = f
	case strings.Contains(name, "fan"):
		health.FanSpeed = uint64(f)
	}
}
