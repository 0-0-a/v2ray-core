package kcp

type Config struct {
	Mtu              uint32 // Maximum transmission unit
	Tti              uint32
	UplinkCapacity   uint32
	DownlinkCapacity uint32
	Congestion       bool
	WriteBuffer      uint32
	ReadBuffer       uint32
}

func (this *Config) Apply() {
	effectiveConfig = *this
}

func (this *Config) GetSendingInFlightSize() uint32 {
	size := this.UplinkCapacity * 1024 * 1024 / this.Mtu / (1000 / this.Tti) / 2
	if size == 0 {
		size = 8
	}
	return size
}

func (this *Config) GetSendingWindowSize() uint32 {
	return this.GetSendingInFlightSize() * 4
}

func (this *Config) GetSendingQueueSize() uint32 {
	return this.WriteBuffer / this.Mtu
}

func (this *Config) GetReceivingWindowSize() uint32 {
	size := this.DownlinkCapacity * 1024 * 1024 / this.Mtu / (1000 / this.Tti) / 2
	if size == 0 {
		size = 8
	}
	return size
}

func (this *Config) GetReceivingQueueSize() uint32 {
	return this.ReadBuffer / this.Mtu
}

func DefaultConfig() Config {
	return Config{
		Mtu:              1350,
		Tti:              20,
		UplinkCapacity:   5,
		DownlinkCapacity: 20,
		Congestion:       false,
		WriteBuffer:      4 * 1024 * 1024,
		ReadBuffer:       4 * 1024 * 1024,
	}
}

var (
	effectiveConfig = DefaultConfig()
)
