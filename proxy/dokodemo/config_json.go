// +build json

package dokodemo

import (
	"encoding/json"

	v2net "github.com/v2ray/v2ray-core/common/net"
	v2netjson "github.com/v2ray/v2ray-core/common/net/json"
	"github.com/v2ray/v2ray-core/proxy/internal/config"
)

func init() {
	config.RegisterInboundConnectionConfig("dokodemo-door",
		func(data []byte) (interface{}, error) {
			type DokodemoConfig struct {
				Host         *v2netjson.Host        `json:"address"`
				PortValue    v2net.Port             `json:"port"`
				NetworkList  *v2netjson.NetworkList `json:"network"`
				TimeoutValue int                    `json:"timeout"`
			}
			rawConfig := new(DokodemoConfig)
			if err := json.Unmarshal(data, rawConfig); err != nil {
				return nil, err
			}
			return &Config{
				Address: rawConfig.Host.Address(),
				Port:    rawConfig.PortValue,
				Network: rawConfig.NetworkList,
				Timeout: rawConfig.TimeoutValue,
			}, nil
		})
}
