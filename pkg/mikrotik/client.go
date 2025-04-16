package mikrotik

import (
	"errors"
	"fmt"
	"log"
	"net" // Import net package for SplitHostPort
	"strconv"
	"strings"
	"time"

	"gopkg.in/routeros.v2"
)

const defaultMikrotikAPIPort = "8728"

// Client holds the connection details and client instance for a MikroTik router.
type Client struct {
	Address  string
	Username string
	Password string
	Timeout  time.Duration
	client   *routeros.Client
}

// NewClient creates a new MikroTik client configuration.
func NewClient(address, username, password string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 10 * time.Second // Default timeout
	}
	return &Client{
		Address:  address,
		Username: username,
		Password: password,
		Timeout:  timeout,
	}
}

// Connect establishes a connection to the MikroTik router.
func (c *Client) Connect() error {
	addr := c.Address
	_, _, err := net.SplitHostPort(addr)
	if err != nil {
		addr = net.JoinHostPort(addr, defaultMikrotikAPIPort)
		log.Printf("Port not specified for %s, using default: %s", c.Address, addr)
	}

	log.Printf("Connecting to MikroTik router at %s with timeout %s...", addr, c.Timeout)
	client, err := routeros.DialTimeout(addr, c.Username, c.Password, c.Timeout)
	if err != nil {
		log.Printf("Error dialing MikroTik router %s (timeout %s): %v", addr, c.Timeout, err)
		return err
	}
	c.client = client
	log.Printf("Successfully connected to MikroTik router %s", addr)
	return nil
}

// Close terminates the connection to the MikroTik router.
func (c *Client) Close() {
	if c.client != nil {
		// Use the address stored in our struct for logging closure
		log.Printf("Closing connection to MikroTik router %s", c.Address)
		c.client.Close()
		c.client = nil
	}
}

// Run executes a command on the MikroTik router and returns the reply.
func (c *Client) Run(cmd ...string) (*routeros.Reply, error) {
	if c.client == nil {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	// Use the address stored in our struct for logging errors
	reply, err := c.client.Run(cmd...)
	if err != nil {
		log.Printf("Error running command on %s (%v): %v", c.Address, cmd, err)
		return nil, err
	}
	return reply, nil
}

// RunArgs executes a command with specific arguments on the MikroTik router.
// Useful for commands like 'monitor' that take arguments differently.
func (c *Client) RunArgs(args []string) (*routeros.Reply, error) {
	if c.client == nil {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	// Use the address stored in our struct for logging errors
	reply, err := c.client.RunArgs(args)
	if err != nil {
		log.Printf("Error running command with args on %s (%v): %v", c.Address, args, err)
		return nil, err
	}
	return reply, nil
}

// SystemResource holds information about system resources.
type SystemResource struct {
	Uptime       time.Duration
	FreeMemory   uint64
	TotalMemory  uint64
	CPULoad       uint64
	FreeHDDSpace  uint64 // Added for storage
	TotalHDDSpace uint64 // Added for storage
	BoardName     string
	Model         string
	SerialNumber  string
}

// Routerboard holds information about the routerboard hardware.
type Routerboard struct {
	BoardName       string
	Model           string
	SerialNumber    string
	FirmwareType    string
	FactoryFirmware string
	CurrentFirmware string
	UpgradeFirmware string
}

// InterfaceStat holds statistics for a single network interface.
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

// BGPPeerStat holds statistics for a single BGP peer.
// Fetched using /routing/bgp/peer/print with .proplist
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

// PPPUserStat holds statistics for a single active PPP user session.
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

// SystemHealth holds information about system health sensors.
type SystemHealth struct {
	Temperature    float64 // Often CPU temperature
	BoardTemperature float64 // Sometimes available as a separate sensor
	Voltage        float64
	Current        float64 // Sometimes available
	PowerConsumed  float64 // Sometimes available
	FanSpeed       uint64  // Sometimes available
	// Add other relevant fields as needed based on /system health print output
}


// GetSystemResources fetches system resource information from the router.
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

	// Parse HDD space (reported in KiB, convert to Bytes)
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
		FreeHDDSpace:  freeHDDSpaceKiB * 1024, // Convert KiB to Bytes
		TotalHDDSpace: totalHDDSpaceKiB * 1024, // Convert KiB to Bytes
		BoardName:     res.Map["board-name"],
		Model:        res.Map["model"],
		SerialNumber: res.Map["serial-number"],
	}, nil
}

// GetRouterboard fetches routerboard hardware information.
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

// --- Helper Functions ---

func parseMikrotikDuration(durationStr string) (time.Duration, error) {
	if durationStr == "" {
		return 0, errors.New("empty duration string")
	}

	var totalDuration time.Duration
	var currentVal strings.Builder
	var unit rune

	for _, r := range durationStr {
		if r >= '0' && r <= '9' || r == '.' { // Allow decimal point for seconds potentially
			currentVal.WriteRune(r)
		} else {
			unit = r
			valStr := currentVal.String()
			if valStr == "" {
				return 0, fmt.Errorf("invalid duration format near unit '%c' in '%s'", unit, durationStr)
			}

			var val int64
			var err error
			// Try parsing as float first for seconds
			if unit == 's' {
				fVal, fErr := strconv.ParseFloat(valStr, 64)
				if fErr == nil {
					totalDuration += time.Duration(fVal * float64(time.Second))
					currentVal.Reset()
					continue // Skip normal integer parsing for 's' if float parse worked
				}
				// Fallback to integer parsing if float fails
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

// GetInterfaceStats fetches statistics for all interfaces, excluding PPP and PPPoE interfaces.
func (c *Client) GetInterfaceStats() ([]InterfaceStat, error) {
	reply, err := c.Run("/interface/print", "detail", "without-paging")
	if err != nil {
		return nil, fmt.Errorf("failed to get interface details: %w", err)
	}

	stats := make([]InterfaceStat, 0, len(reply.Re))
	ifaceMap := make(map[string]*InterfaceStat)

	for _, re := range reply.Re {
		name := re.Map["name"]
		if name == "" {
			log.Printf("Warning: Skipping interface with empty name: %v", re.Map)
			continue
		}

		// Skip PPP and PPPoE interfaces
		ifaceType := re.Map["type"]
		if strings.Contains(strings.ToLower(ifaceType), "ppp") ||
		   strings.Contains(strings.ToLower(ifaceType), "pppoe") ||
		   strings.Contains(strings.ToLower(name), "ppp") ||
		   strings.Contains(strings.ToLower(name), "pppoe") {
			log.Printf("Skipping PPP/PPPoE interface: %s (type: %s)", name, ifaceType)
			continue
		}

		stat := InterfaceStat{
			Name:       name,
			Type:       ifaceType,
			Comment:    re.Map["comment"],
			MACAddress: re.Map["mac-address"],
			Running:    parseBool(re.Map["running"]),
			Disabled:   parseBool(re.Map["disabled"]),
		}
		stats = append(stats, stat)
		ifaceMap[name] = &stats[len(stats)-1]
	}

	if len(stats) == 0 {
		log.Println("No interfaces found to monitor traffic for.")
		return stats, nil
	}

	interfaceNames := make([]string, 0, len(stats))
	for _, s := range stats {
		interfaceNames = append(interfaceNames, s.Name)
	}
	// Debug: Print all available fields for interfaces
	log.Printf("DEBUG: Interface names to monitor: %v", interfaceNames)

	// Try to get interface stats directly from interface print stats command
	statsCmd := []string{"/interface/print", "stats", "without-paging"}
	statsReply, statsErr := c.Run(statsCmd...)

	// If getting stats failed, log a warning and return the basic stats (without traffic counters)
	if statsErr != nil || len(statsReply.Re) == 0 {
		log.Printf("Warning: Failed to get interface traffic counters using '/interface/print stats' for %s: %v. Returning basic interface info only.", c.Address, statsErr)
		// Returning basic stats (name, type, status) collected earlier
		return stats, nil
	}

	// If successful, process the statsReply
	log.Printf("Successfully got interface stats using '/interface/print stats'")
	// Debug: Print all available fields for interface stats
	for _, re := range statsReply.Re {
		log.Printf("DEBUG: Interface stats fields available for %s: %v", re.Map["name"], re.Map)
	}

	// Process interface stats from print stats command
	// Add defer/recover to catch potential panics during processing
	defer func() {
		if r := recover(); r != nil {
			log.Printf("ERROR: Recovered from panic while processing interface stats for %s: %v", c.Address, r)
			// Optionally, you could add more context here, like the specific interface being processed if possible
		}
	}()

	for _, re := range statsReply.Re {
		name := re.Map["name"]
		stat, ok := ifaceMap[name]
		if !ok || stat == nil { // Add explicit nil check for stat
			log.Printf("Warning: Skipping interface '%s' from stats reply because it's not in the initial map or stat is nil.", name)
			continue // Skip interfaces we're not tracking or if stat is nil
		}

		// Try different field names for rx/tx bytes
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

	// Return the stats populated with traffic counters
	return stats, nil
}

// GetSystemHealth fetches system health information (like temperature, voltage).
func (c *Client) GetSystemHealth() (*SystemHealth, error) {
	reply, err := c.Run("/system/health/print")
	if err != nil {
		// Check if the command itself is not supported (common on some models/versions)
		if strings.Contains(err.Error(), "no such command") || strings.Contains(err.Error(), "unknown command name") {
			log.Printf("Info: /system/health/print command not found on %s. Temperature monitoring might not be supported.", c.Address)
			return nil, nil // Return nil, nil to indicate not supported, not an error preventing other metrics
		}
		return nil, fmt.Errorf("failed to get system health: %w", err)
	}

	if len(reply.Re) == 0 {
		log.Printf("Warning: No system health data received from %s.", c.Address)
		// It's possible the command exists but returns no data if sensors aren't present/enabled.
		// Treat this similarly to the command not being found.
		return nil, nil
	}
	healthData := reply.Re[0] // Assuming health info is in the first sentence

	// Helper to parse float values, logging warnings on error
	parseFloat := func(key string) float64 {
		valStr := healthData.Map[key]
		if valStr == "" {
			// log.Printf("Debug: Health key '%s' not found or empty on %s", key, c.Address)
			return 0 // Or perhaps math.NaN() if you prefer to distinguish missing from zero
		}
		// RouterOS often includes units like 'C' or 'V'. Remove them.
		valStr = strings.TrimRight(valStr, "CVW RPM") // Add other units as needed
		val, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			log.Printf("Warning: Could not parse health value for key '%s' ('%s') on %s: %v", key, healthData.Map[key], c.Address, err)
			return 0 // Or math.NaN()
		}
		return val
	}
	// Helper to parse uint values, logging warnings on error
	parseUint := func(key string) uint64 {
		valStr := healthData.Map[key]
		if valStr == "" {
			// log.Printf("Debug: Health key '%s' not found or empty on %s", key, c.Address)
			return 0
		}
		// Remove units if necessary (e.g., RPM)
		valStr = strings.TrimRight(valStr, " RPM")
		val, err := strconv.ParseUint(valStr, 10, 64)
		if err != nil {
			log.Printf("Warning: Could not parse health value for key '%s' ('%s') on %s: %v", key, healthData.Map[key], c.Address, err)
			return 0
		}
		return val
	}


	// Try common keys for temperature. 'temperature' is often CPU temp.
	// 'board-temperature', 'cpu-temperature', 'sfp-temperature' might also exist.
	temp := parseFloat("temperature")
	boardTemp := parseFloat("board-temperature")
	if boardTemp == 0 { // Fallback if board-temperature doesn't exist
		boardTemp = parseFloat("cpu-temperature") // Sometimes 'cpu-temperature' is used instead of 'temperature'
	}
	// If 'temperature' was 0 but 'cpu-temperature' exists, use it for the main temp
	if temp == 0 && boardTemp != 0 && healthData.Map["temperature"] == "" && healthData.Map["cpu-temperature"] != "" {
		temp = boardTemp
		// Decide if boardTemp should be reset or kept same as temp in this case
		// boardTemp = 0 // Option 1: Reset boardTemp if it was just a fallback for the main temp
	}


	health := &SystemHealth{
		Temperature:    temp,
		BoardTemperature: boardTemp,
		Voltage:        parseFloat("voltage"),
		Current:        parseFloat("current"),
		PowerConsumed:  parseFloat("power-consumption"), // Check exact key name
		FanSpeed:       parseUint("fan1-speed"), // Check exact key names (fan2-speed etc.)
	}

	log.Printf("Debug: Parsed health data for %s: %+v", c.Address, health)

	return health, nil
}
