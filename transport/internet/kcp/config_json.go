// +build json

package kcp

import (
	"encoding/json"

	"github.com/golang/protobuf/proto"
	"v2ray.com/core/common"
	"v2ray.com/core/common/loader"
	"v2ray.com/core/common/log"
	"v2ray.com/core/transport/internet/authenticators/noop"
	"v2ray.com/core/transport/internet/authenticators/srtp"
	"v2ray.com/core/transport/internet/authenticators/utp"
)

func (this *Config) UnmarshalJSON(data []byte) error {
	type JSONConfig struct {
		Mtu             *uint32         `json:"mtu"`
		Tti             *uint32         `json:"tti"`
		UpCap           *uint32         `json:"uplinkCapacity"`
		DownCap         *uint32         `json:"downlinkCapacity"`
		Congestion      *bool           `json:"congestion"`
		ReadBufferSize  *uint32         `json:"readBufferSize"`
		WriteBufferSize *uint32         `json:"writeBufferSize"`
		HeaderConfig    json.RawMessage `json:"header"`
	}
	jsonConfig := new(JSONConfig)
	if err := json.Unmarshal(data, &jsonConfig); err != nil {
		return err
	}
	if jsonConfig.Mtu != nil {
		mtu := *jsonConfig.Mtu
		if mtu < 576 || mtu > 1460 {
			log.Error("KCP|Config: Invalid MTU size: ", mtu)
			return common.ErrBadConfiguration
		}
		this.Mtu = &MTU{Value: *jsonConfig.Mtu}
	}
	if jsonConfig.Tti != nil {
		tti := *jsonConfig.Tti
		if tti < 10 || tti > 100 {
			log.Error("KCP|Config: Invalid TTI: ", tti)
			return common.ErrBadConfiguration
		}
		this.Tti = &TTI{Value: *jsonConfig.Tti}
	}
	if jsonConfig.UpCap != nil {
		this.UplinkCapacity = &UplinkCapacity{Value: *jsonConfig.UpCap}
	}
	if jsonConfig.DownCap != nil {
		this.DownlinkCapacity = &DownlinkCapacity{Value: *jsonConfig.DownCap}
	}
	if jsonConfig.Congestion != nil {
		this.Congestion = *jsonConfig.Congestion
	}
	if jsonConfig.ReadBufferSize != nil {
		size := *jsonConfig.ReadBufferSize
		if size > 0 {
			this.ReadBuffer = &ReadBuffer{Size: size * 1024 * 1024}
		} else {
			this.ReadBuffer = &ReadBuffer{Size: 512 * 1024}
		}
	}
	if jsonConfig.WriteBufferSize != nil {
		size := *jsonConfig.WriteBufferSize
		if size > 0 {
			this.WriteBuffer = &WriteBuffer{Size: size * 1024 * 1024}
		} else {
			this.WriteBuffer = &WriteBuffer{Size: 512 * 1024}
		}
	}
	if len(jsonConfig.HeaderConfig) > 0 {
		config, _, err := headerLoader.Load(jsonConfig.HeaderConfig)
		if err != nil {
			log.Error("KCP|Config: Failed to parse header config: ", err)
			return err
		}
		this.HeaderConfig = loader.NewTypedSettings(config.(proto.Message))
	}

	return nil
}

var (
	headerLoader = loader.NewJSONConfigLoader(loader.NamedTypeMap{
		"none": loader.GetType(new(noop.Config)),
		"srtp": loader.GetType(new(srtp.Config)),
		"utp":  loader.GetType(new(utp.Config)),
	}, "type", "")
)
