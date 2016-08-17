package blackhole

import (
	"github.com/v2ray/v2ray-core/app"
	"github.com/v2ray/v2ray-core/common/alloc"
	v2net "github.com/v2ray/v2ray-core/common/net"
	"github.com/v2ray/v2ray-core/proxy"
	"github.com/v2ray/v2ray-core/proxy/registry"
	"github.com/v2ray/v2ray-core/transport/internet"
	"github.com/v2ray/v2ray-core/transport/ray"
)

// BlackHole is an outbound connection that sliently swallow the entire payload.
type BlackHole struct {
	meta     *proxy.OutboundHandlerMeta
	response Response
}

func NewBlackHole(space app.Space, config *Config, meta *proxy.OutboundHandlerMeta) *BlackHole {
	return &BlackHole{
		meta:     meta,
		response: config.Response,
	}
}

func (this *BlackHole) Dispatch(destination v2net.Destination, payload *alloc.Buffer, ray ray.OutboundRay) error {
	payload.Release()

	this.response.WriteTo(ray.OutboundOutput())
	ray.OutboundOutput().Close()

	ray.OutboundInput().Release()

	return nil
}

type Factory struct{}

func (this *Factory) StreamCapability() internet.StreamConnectionType {
	return internet.StreamConnectionTypeRawTCP
}

func (this *Factory) Create(space app.Space, config interface{}, meta *proxy.OutboundHandlerMeta) (proxy.OutboundHandler, error) {
	return NewBlackHole(space, config.(*Config), meta), nil
}

func init() {
	registry.MustRegisterOutboundHandlerCreator("blackhole", new(Factory))
}
