package testing

import (
	"fmt"

	"github.com/v2ray/v2ray-core/proxy/internal"
)

var count = 0

func randomString() string {
	count++
	return fmt.Sprintf("-%d", count)
}

func RegisterInboundConnectionHandlerCreator(prefix string, creator internal.InboundHandlerFactory) (string, error) {
	for {
		name := prefix + randomString()
		err := internal.RegisterInboundHandlerCreator(name, creator)
		if err != internal.ErrNameExists {
			return name, err
		}
	}
}

func RegisterOutboundConnectionHandlerCreator(prefix string, creator internal.OutboundHandlerFactory) (string, error) {
	for {
		name := prefix + randomString()
		err := internal.RegisterOutboundHandlerCreator(name, creator)
		if err != internal.ErrNameExists {
			return name, err
		}
	}
}
