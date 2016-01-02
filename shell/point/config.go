package point

import (
	"github.com/v2ray/v2ray-core/app/dns"
	"github.com/v2ray/v2ray-core/app/router"
	"github.com/v2ray/v2ray-core/common/log"
	v2net "github.com/v2ray/v2ray-core/common/net"
)

type ConnectionConfig interface {
	Protocol() string
	Settings() []byte
}

type LogConfig interface {
	AccessLog() string
	ErrorLog() string
	LogLevel() log.LogLevel
}

type DnsConfig interface {
	Enabled() bool
	Settings() dns.CacheConfig
}

const (
	AllocationStrategyAlways   = "always"
	AllocationStrategyRandom   = "random"
	AllocationStrategyExternal = "external"
)

type InboundDetourAllocationConfig interface {
	Strategy() string // Allocation strategy of this inbound detour.
	Concurrency() int // Number of handlers (ports) running in parallel.
	Refresh() int     // Number of seconds before a handler is regenerated.
}

type InboundDetourConfig interface {
	Protocol() string
	PortRange() v2net.PortRange
	Tag() string
	Allocation() InboundDetourAllocationConfig
	Settings() []byte
}

type OutboundDetourConfig interface {
	Protocol() string
	Tag() string
	Settings() []byte
}

type PointConfig interface {
	Port() v2net.Port
	LogConfig() LogConfig
	RouterConfig() router.Config
	InboundConfig() ConnectionConfig
	OutboundConfig() ConnectionConfig
	InboundDetours() []InboundDetourConfig
	OutboundDetours() []OutboundDetourConfig
}
