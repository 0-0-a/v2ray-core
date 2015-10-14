package point

import (
	"github.com/v2ray/v2ray-core/common/log"
	v2net "github.com/v2ray/v2ray-core/common/net"
	"github.com/v2ray/v2ray-core/common/retry"
	"github.com/v2ray/v2ray-core/config"
	"github.com/v2ray/v2ray-core/proxy"
	"github.com/v2ray/v2ray-core/transport/ray"
)

// Point is an single server in V2Ray system.
type Point struct {
	port uint16
	ich  proxy.InboundConnectionHandler
	och  proxy.OutboundConnectionHandler
}

// NewPoint returns a new Point server based on given configuration.
// The server is not started at this point.
func NewPoint(pConfig config.PointConfig) (*Point, error) {
	var vpoint = new(Point)
	vpoint.port = pConfig.Port()

	ichFactory := proxy.GetInboundConnectionHandlerFactory(pConfig.InboundConfig().Protocol())
	if ichFactory == nil {
		log.Error("Unknown inbound connection handler factory %s", pConfig.InboundConfig().Protocol())
		return nil, config.BadConfiguration
	}
	ichConfig := pConfig.InboundConfig().Settings(config.TypeInbound)
	ich, err := ichFactory.Create(vpoint, ichConfig)
	if err != nil {
		log.Error("Failed to create inbound connection handler: %v", err)
		return nil, err
	}
	vpoint.ich = ich

	ochFactory := proxy.GetOutboundConnectionHandlerFactory(pConfig.OutboundConfig().Protocol())
	if ochFactory == nil {
		log.Error("Unknown outbound connection handler factory %s", pConfig.OutboundConfig().Protocol())
		return nil, config.BadConfiguration
	}
	ochConfig := pConfig.OutboundConfig().Settings(config.TypeOutbound)
	och, err := ochFactory.Create(ochConfig)
	if err != nil {
		log.Error("Failed to create outbound connection handler: %v", err)
		return nil, err
	}
	vpoint.och = och

	return vpoint, nil
}

// Start starts the Point server, and return any error during the process.
// In the case of any errors, the state of the server is unpredicatable.
func (vp *Point) Start() error {
	if vp.port <= 0 {
		log.Error("Invalid port %d", vp.port)
		return config.BadConfiguration
	}

	return retry.Timed(100 /* times */, 100 /* ms */).On(func() error {
		err := vp.ich.Listen(vp.port)
		if err == nil {
			log.Warning("Point server started on port %d", vp.port)
			return nil
		}
		return err
	})
}

func (p *Point) DispatchToOutbound(packet v2net.Packet) ray.InboundRay {
	direct := ray.NewRay()
	go p.och.Dispatch(packet, direct)
	return direct
}
