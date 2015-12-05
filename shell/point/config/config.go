package config

import (
	routerconfig "github.com/v2ray/v2ray-core/app/router/config"
	"github.com/v2ray/v2ray-core/common/log"
	v2net "github.com/v2ray/v2ray-core/common/net"
)

type ConnectionConfig interface {
	Protocol() string
	Settings() interface{}
}

type LogConfig interface {
	AccessLog() string
	ErrorLog() string
	LogLevel() log.LogLevel
}

type InboundDetourConfig interface {
	Protocol() string
	PortRange() v2net.PortRange
	Settings() interface{}
}

type OutboundDetourConfig interface {
	Protocol() string
	Tag() string
	Settings() interface{}
}

type PointConfig interface {
	Port() v2net.Port
	LogConfig() LogConfig
	RouterConfig() routerconfig.RouterConfig
	InboundConfig() ConnectionConfig
	OutboundConfig() ConnectionConfig
	InboundDetours() []InboundDetourConfig
	OutboundDetours() []OutboundDetourConfig
}
