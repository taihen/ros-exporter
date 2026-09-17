package mikrotik

import "time"

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
	Speed      uint64 // bits per second; 0 if unknown
	FullDuplex bool
	HasDuplex  bool // true when duplex was reported by the device
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
	Name             string
	RoutingInstance  string
	RemoteAddress    string
	RemoteAS         string
	LocalAddress     string
	LocalRole        string
	RemoteRole       string
	State            string
	Uptime           time.Duration
	PrefixCount      uint64
	UpdatesSent      uint64
	UpdatesRecv      uint64
	WithdrawsSent    uint64
	WithdrawsRecv    uint64
	Disabled         bool
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
	Supported        bool
}

type WirelessClient struct {
	Interface      string
	MacAddress     string
	SignalStrength int
	TxCCQ          int
	RxRateBps      float64
	TxRateBps      float64
	Uptime         time.Duration
	NoiseFloor     int
	HasNoiseFloor  bool
}

type WirelessInterface struct {
	Name           string
	SSID           string
	Frequency      int
	SignalStrength int
	TxRate         float64
	RxRate         float64
	NoiseFloor     int
	HasNoiseFloor  bool
}

type OSPFNeighborStat struct {
	RouterID   string
	Address    string
	Interface  string
	State      string
	StateValue float64 // 1 = Full, 0 = other
	Priority   uint64
}

type TransceiverStat struct {
	Interface   string
	Temperature float64
	TxPowerDbm  float64
	RxPowerDbm  float64
	HasTemp     bool
	HasTxPower  bool
	HasRxPower  bool
}
