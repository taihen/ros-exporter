package mikrotik

import (
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/go-routeros/routeros/v3"
)

const defaultMikrotikAPIPort = "8728"
const DefaultTimeout = 10 * time.Second

type Client struct {
	Address  string
	Username string
	Password string
	Timeout  time.Duration
	client   *routeros.Client
}

func NewClient(address, username, password string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &Client{
		Address:  address,
		Username: username,
		Password: password,
		Timeout:  timeout,
	}
}

func (c *Client) Connect() error {
	addr := c.Address
	_, _, err := net.SplitHostPort(addr)
	if err != nil {
		addr = net.JoinHostPort(addr, defaultMikrotikAPIPort)
	}

	log.Printf("Connecting to MikroTik router at %s with timeout %s...", addr, c.Timeout)
	client, err := routeros.DialTimeout(addr, c.Username, c.Password, c.Timeout)
	if err != nil {
		log.Printf("Error dialing MikroTik router %s: %v", addr, err)
		return err
	}
	c.client = client
	return nil
}

func (c *Client) Close() {
	if c.client != nil {
		log.Printf("Closing connection to MikroTik router %s", c.Address)
		c.client.Close()
		c.client = nil
	}
}

func (c *Client) Run(cmd ...string) (*routeros.Reply, error) {
	if c.client == nil {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	replyCh := make(chan *routeros.Reply, 1)
	errCh := make(chan error, 1)

	go func() {
		reply, err := c.client.Run(cmd...)
		if err != nil {
			errCh <- err
			return
		}
		replyCh <- reply
	}()

	select {
	case reply := <-replyCh:
		return reply, nil
	case err := <-errCh:
		log.Printf("Error running command on %s: %v", c.Address, err)
		c.Close()
		return nil, err
	case <-time.After(c.Timeout):
		log.Printf("Timeout running command on %s", c.Address)
		c.Close()
		return nil, fmt.Errorf("command timeout after %s", c.Timeout)
	}
}

func (c *Client) RunArgs(args []string) (*routeros.Reply, error) {
	if c.client == nil {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	replyCh := make(chan *routeros.Reply, 1)
	errCh := make(chan error, 1)

	go func() {
		reply, err := c.client.RunArgs(args)
		if err != nil {
			errCh <- err
			return
		}
		replyCh <- reply
	}()

	select {
	case reply := <-replyCh:
		return reply, nil
	case err := <-errCh:
		log.Printf("Error running command with args on %s: %v", c.Address, err)
		c.Close()
		return nil, err
	case <-time.After(c.Timeout):
		log.Printf("Timeout running command with args on %s", c.Address)
		c.Close()
		return nil, fmt.Errorf("command timeout after %s", c.Timeout)
	}
}

type SystemResource struct {
	Uptime        time.Duration
	FreeMemory    uint64
	TotalMemory   uint64
	CPULoad       uint64
	FreeHDDSpace  uint64
	TotalHDDSpace uint64
	BoardName     string
	Model         string
	SerialNumber  string
}

type Routerboard struct {
	BoardName       string
	Model           string
	SerialNumber    string
	FirmwareType    string
	FactoryFirmware string
	CurrentFirmware string
	UpgradeFirmware string
}

type InterfaceStat struct {
	Name       string
	Type       string
	Comment    string
	MACAddress string
	Running    bool
	Disabled   bool
	RxBytes    uint64
	TxBytes    uint64
	RxPackets  uint64
	TxPackets  uint64
	RxErrors   uint64
	TxErrors   uint64
	RxDrops    uint64
	TxDrops    uint64
}

type BGPPeerStat struct {
	Name          string
	Instance      string
	RemoteAddress string
	RemoteAS      string
	LocalAddress  string
	LocalRole     string
	RemoteRole    string
	State         string
	Uptime        time.Duration
	PrefixCount   uint64
	UpdatesSent   uint64
	UpdatesRecv   uint64
	WithdrawsSent uint64
	WithdrawsRecv uint64
	Disabled      bool
}

type PPPUserStat struct {
	Name      string
	Service   string
	CallerID  string
	Address   string
	Uptime    time.Duration
	UptimeStr string
	RxBytes   uint64
	TxBytes   uint64
}

type SystemHealth struct {
	Temperature      float64
	BoardTemperature float64
	Voltage          float64
	Current          float64
	PowerConsumed    float64
	FanSpeed         uint64
}

func (c *Client) GetSystemResources() (*SystemResource, error) {
	reply, err := c.Run("/system/resource/print")
	if err != nil {
		return nil, fmt.Errorf("failed to get system resources: %w", err)
	}

	if len(reply.Re) == 0 {
		return nil, errors.New("no system resource data received")
	}
	res := reply.Re[0]

	uptime, err := parseMikrotikDuration(res.Map["uptime"])
	if err != nil {
		log.Printf("Warning: Could not parse uptime '%s': %v", res.Map["uptime"], err)
	}

	freeMem, err := parseBytes(res.Map["free-memory"])
	if err != nil {
		log.Printf("Warning: Could not parse free-memory '%s': %v", res.Map["free-memory"], err)
	}

	totalMem, err := parseBytes(res.Map["total-memory"])
	if err != nil {
		log.Printf("Warning: Could not parse total-memory '%s': %v", res.Map["total-memory"], err)
	}

	cpuLoad, err := strconv.ParseUint(res.Map["cpu-load"], 10, 64)
	if err != nil {
		log.Printf("Warning: Could not parse cpu-load '%s': %v", res.Map["cpu-load"], err)
	}

	freeHDDSpaceKiB, err := parseBytes(res.Map["free-hdd-space"])
	if err != nil {
		log.Printf("Warning: Could not parse free-hdd-space '%s': %v", res.Map["free-hdd-space"], err)
	}
	totalHDDSpaceKiB, err := parseBytes(res.Map["total-hdd-space"])
	if err != nil {
		log.Printf("Warning: Could not parse total-hdd-space '%s': %v", res.Map["total-hdd-space"], err)
	}

	return &SystemResource{
		Uptime:        uptime,
		FreeMemory:    freeMem,
		TotalMemory:   totalMem,
		CPULoad:       cpuLoad,
		FreeHDDSpace:  freeHDDSpaceKiB * 1024,
		TotalHDDSpace: totalHDDSpaceKiB * 1024,
		BoardName:     res.Map["board-name"],
		Model:         res.Map["model"],
		SerialNumber:  res.Map["serial-number"],
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
	rb := reply.Re[0]

	return &Routerboard{
		BoardName:       rb.Map["board-name"],
		Model:           rb.Map["model"],
		SerialNumber:    rb.Map["serial-number"],
		FirmwareType:    rb.Map["firmware-type"],
		FactoryFirmware: rb.Map["factory-firmware"],
		CurrentFirmware: rb.Map["current-firmware"],
		UpgradeFirmware: rb.Map["upgrade-firmware"],
	}, nil
}

func parseMikrotikDuration(durationStr string) (time.Duration, error) {
	if durationStr == "" {
		return 0, errors.New("empty duration string")
	}

	var totalDuration time.Duration
	var currentVal strings.Builder
	var unit rune

	for _, r := range durationStr {
		if r >= '0' && r <= '9' || r == '.' {
			currentVal.WriteRune(r)
		} else {
			unit = r
			valStr := currentVal.String()
			if valStr == "" {
				return 0, fmt.Errorf("invalid duration format near unit '%c' in '%s'", unit, durationStr)
			}

			var val int64
			var err error
			if unit == 's' {
				fVal, fErr := strconv.ParseFloat(valStr, 64)
				if fErr == nil {
					totalDuration += time.Duration(fVal * float64(time.Second))
					currentVal.Reset()
					continue
				}
			}

			val, err = strconv.ParseInt(valStr, 10, 64)
			if err != nil {
				return 0, fmt.Errorf("could not parse value '%s' in duration '%s': %w", valStr, durationStr, err)
			}

			switch unit {
			case 'w':
				totalDuration += time.Duration(val) * 7 * 24 * time.Hour
			case 'd':
				totalDuration += time.Duration(val) * 24 * time.Hour
			case 'h':
				totalDuration += time.Duration(val) * time.Hour
			case 'm':
				totalDuration += time.Duration(val) * time.Minute
			case 's':
				totalDuration += time.Duration(val) * time.Second
			default:
				return 0, fmt.Errorf("unknown duration unit '%c' in '%s'", unit, durationStr)
			}
			currentVal.Reset()
		}
	}
	if currentVal.Len() > 0 {
		return 0, fmt.Errorf("trailing number without unit in duration '%s'", durationStr)
	}

	return totalDuration, nil
}

func parseBytes(byteStr string) (uint64, error) {
	if byteStr == "" {
		return 0, errors.New("empty byte string")
	}
	bytes, err := strconv.ParseUint(byteStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("could not parse byte value '%s': %w", byteStr, err)
	}
	return bytes, nil
}

func parseBool(boolStr string) bool {
	return strings.ToLower(boolStr) == "true"
}

func (c *Client) GetInterfaceStats() ([]InterfaceStat, error) {
	initialReply, err := c.Run("/interface/print")
	if err != nil {
		return nil, fmt.Errorf("failed to get initial interface names/types: %w", err)
	}

	stats := make([]InterfaceStat, 0, len(initialReply.Re))
	ifaceMap := make(map[string]*InterfaceStat)

	for _, re := range initialReply.Re {
		name := re.Map["name"]
		if name == "" {
			log.Printf("Warning: Skipping interface with empty name: %v", re.Map)
			continue
		}

		ifaceType := re.Map["type"]
		if strings.Contains(strings.ToLower(ifaceType), "ppp") ||
			strings.Contains(strings.ToLower(ifaceType), "pppoe") ||
			strings.Contains(strings.ToLower(name), "ppp") ||
			strings.Contains(strings.ToLower(name), "pppoe") {
			log.Printf("Skipping PPP/PPPoE interface: %s (type: %s)", name, ifaceType)
			continue
		}

		stat := InterfaceStat{
			Name: name,
			Type: ifaceType,
		}
		stats = append(stats, stat)
		ifaceMap[name] = &stats[len(stats)-1]
	}

	if len(stats) == 0 {
		log.Println("No non-PPP/PPPoE interfaces found to monitor traffic for.")
		return stats, nil
	}

	interfaceNames := make([]string, 0, len(stats))
	for _, s := range stats {
		interfaceNames = append(interfaceNames, s.Name)
	}

	detailReply, detailErr := c.Run("/interface/print", "detail", "without-paging")
	if detailErr != nil {
		log.Printf("Warning: Failed to get detailed interface info for %s: %v. Proceeding without comment/mac/status.", c.Address, detailErr)
	} else {
		log.Printf("Successfully got detailed interface info for %s", c.Address)
		for _, re := range detailReply.Re {
			name := re.Map["name"]
			if statPtr, ok := ifaceMap[name]; ok && statPtr != nil {
				statPtr.Comment = re.Map["comment"]
				statPtr.MACAddress = re.Map["mac-address"]
				statPtr.Running = parseBool(re.Map["running"])
				statPtr.Disabled = parseBool(re.Map["disabled"])
			}
		}
	}

	monitoredNames := make([]string, 0, len(ifaceMap))
	for name := range ifaceMap {
		monitoredNames = append(monitoredNames, name)
	}
	log.Printf("DEBUG: Attempting to fetch stats for interfaces: %v", monitoredNames)

	statsCmd := []string{"/interface/print", "stats", "without-paging"}
	statsReply, statsErr := c.Run(statsCmd...)

	if statsErr != nil {
		log.Printf("Warning: Failed to get interface traffic counters using '/interface/print stats' for %s: %v. Returning interface info without traffic counters.", c.Address, statsErr)
		return stats, nil
	}
	if len(statsReply.Re) == 0 {
		log.Printf("Warning: Received empty reply for '/interface/print stats' from %s. No traffic counters available.", c.Address)
		return stats, nil
	}

	log.Printf("Successfully got interface stats reply using '/interface/print stats' from %s", c.Address)
	if len(statsReply.Re) > 0 {
		log.Printf("DEBUG: Sample stats fields available for %s: %v", statsReply.Re[0].Map["name"], statsReply.Re[0].Map)
	}

	defer func() {
		if r := recover(); r != nil {
			log.Printf("ERROR: Recovered from panic while processing interface stats for %s: %v", c.Address, r)
		}
	}()

	for _, re := range statsReply.Re {
		name := re.Map["name"]
		stat, ok := ifaceMap[name]
		if !ok || stat == nil {
			log.Printf("Warning: Skipping interface '%s' from stats reply because it's not in the initial map or stat is nil.", name)
			continue
		}

		rxBytesFields := []string{"rx-byte", "rx-bytes", "bytes-in"}
		for _, field := range rxBytesFields {
			if rxBytesStr, ok := re.Map[field]; ok && rxBytesStr != "" {
				stat.RxBytes, _ = parseBytes(rxBytesStr)
				log.Printf("Using field '%s' for interface '%s' rx bytes: %d", field, name, stat.RxBytes)
				break
			}
		}

		txBytesFields := []string{"tx-byte", "tx-bytes", "bytes-out"}
		for _, field := range txBytesFields {
			if txBytesStr, ok := re.Map[field]; ok && txBytesStr != "" {
				stat.TxBytes, _ = parseBytes(txBytesStr)
				log.Printf("Using field '%s' for interface '%s' tx bytes: %d", field, name, stat.TxBytes)
				break
			}
		}

		rxPacketsFields := []string{"rx-packet", "rx-packets", "packets-in"}
		for _, field := range rxPacketsFields {
			if rxPacketsStr, ok := re.Map[field]; ok && rxPacketsStr != "" {
				stat.RxPackets, _ = parseBytes(rxPacketsStr)
				log.Printf("Using field '%s' for interface '%s' rx packets: %d", field, name, stat.RxPackets)
				break
			}
		}

		txPacketsFields := []string{"tx-packet", "tx-packets", "packets-out"}
		for _, field := range txPacketsFields {
			if txPacketsStr, ok := re.Map[field]; ok && txPacketsStr != "" {
				stat.TxPackets, _ = parseBytes(txPacketsStr)
				log.Printf("Using field '%s' for interface '%s' tx packets: %d", field, name, stat.TxPackets)
				break
			}
		}

		rxErrorsFields := []string{"rx-error", "rx-errors", "errors-in"}
		for _, field := range rxErrorsFields {
			if rxErrorsStr, ok := re.Map[field]; ok && rxErrorsStr != "" {
				stat.RxErrors, _ = parseBytes(rxErrorsStr)
				log.Printf("Using field '%s' for interface '%s' rx errors: %d", field, name, stat.RxErrors)
				break
			}
		}

		txErrorsFields := []string{"tx-error", "tx-errors", "errors-out"}
		for _, field := range txErrorsFields {
			if txErrorsStr, ok := re.Map[field]; ok && txErrorsStr != "" {
				stat.TxErrors, _ = parseBytes(txErrorsStr)
				log.Printf("Using field '%s' for interface '%s' tx errors: %d", field, name, stat.TxErrors)
				break
			}
		}

		rxDropsFields := []string{"rx-drop", "rx-drops", "drops-in"}
		for _, field := range rxDropsFields {
			if rxDropsStr, ok := re.Map[field]; ok && rxDropsStr != "" {
				stat.RxDrops, _ = parseBytes(rxDropsStr)
				log.Printf("Using field '%s' for interface '%s' rx drops: %d", field, name, stat.RxDrops)
				break
			}
		}

		txDropsFields := []string{"tx-drop", "tx-drops", "drops-out"}
		for _, field := range txDropsFields {
			if txDropsStr, ok := re.Map[field]; ok && txDropsStr != "" {
				stat.TxDrops, _ = parseBytes(txDropsStr)
				log.Printf("Using field '%s' for interface '%s' tx drops: %d", field, name, stat.TxDrops)
				break
			}
		}
	}

	return stats, nil
}

func (c *Client) GetSystemHealth() (*SystemHealth, error) {
	reply, err := c.Run("/system/health/print")
	if err != nil {
		if strings.Contains(err.Error(), "no such command") || strings.Contains(err.Error(), "unknown command name") {
			log.Printf("Info: /system/health/print command not found on %s. Temperature monitoring might not be supported.", c.Address)
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get system health: %w", err)
	}

	if len(reply.Re) == 0 {
		log.Printf("Warning: No system health data received from %s.", c.Address)
		return nil, nil
	}
	healthData := reply.Re[0]

	parseFloat := func(key string) float64 {
		valStr := healthData.Map[key]
		if valStr == "" {
			return 0
		}
		valStr = strings.TrimRight(valStr, "CVW RPM")
		val, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			log.Printf("Warning: Could not parse health value for key '%s' ('%s') on %s: %v", key, healthData.Map[key], c.Address, err)
			return 0
		}
		return val
	}

	parseUint := func(key string) uint64 {
		valStr := healthData.Map[key]
		if valStr == "" {
			return 0
		}
		valStr = strings.TrimRight(valStr, " RPM")
		val, err := strconv.ParseUint(valStr, 10, 64)
		if err != nil {
			log.Printf("Warning: Could not parse health value for key '%s' ('%s') on %s: %v", key, healthData.Map[key], c.Address, err)
			return 0
		}
		return val
	}

	temp := parseFloat("temperature")
	boardTemp := parseFloat("board-temperature")
	if boardTemp == 0 {
		boardTemp = parseFloat("cpu-temperature")
	}
	if temp == 0 && boardTemp != 0 && healthData.Map["temperature"] == "" && healthData.Map["cpu-temperature"] != "" {
		temp = boardTemp
	}

	health := &SystemHealth{
		Temperature:      temp,
		BoardTemperature: boardTemp,
		Voltage:          parseFloat("voltage"),
		Current:          parseFloat("current"),
		PowerConsumed:    parseFloat("power-consumption"),
		FanSpeed:         parseUint("fan1-speed"),
	}

	log.Printf("Debug: Parsed health data for %s: %+v", c.Address, health)

	return health, nil
}
