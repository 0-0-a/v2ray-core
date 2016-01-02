package repo

import (
	"github.com/v2ray/v2ray-core/app"
	"github.com/v2ray/v2ray-core/proxy"
	"github.com/v2ray/v2ray-core/proxy/internal"
)

func CreateInboundConnectionHandler(name string, space app.Space, rawConfig []byte) (proxy.InboundConnectionHandler, error) {
	return internal.CreateInboundConnectionHandler(name, space, rawConfig)
}

func CreateOutboundConnectionHandler(name string, space app.Space, rawConfig []byte) (proxy.OutboundConnectionHandler, error) {
	return internal.CreateOutboundConnectionHandler(name, space, rawConfig)
}
