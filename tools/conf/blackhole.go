package conf

import (
	"encoding/json"
	"v2ray.com/core/common/errors"
	"v2ray.com/core/common/loader"
	"v2ray.com/core/proxy/blackhole"
)

type NoneResponse struct{}

func (*NoneResponse) Build() (*loader.TypedSettings, error) {
	return loader.NewTypedSettings(new(blackhole.NoneResponse)), nil
}

type HttpResponse struct{}

func (*HttpResponse) Build() (*loader.TypedSettings, error) {
	return loader.NewTypedSettings(new(blackhole.HTTPResponse)), nil
}

type BlackholeConfig struct {
	Response json.RawMessage `json:"response"`
}

func (v *BlackholeConfig) Build() (*loader.TypedSettings, error) {
	config := new(blackhole.Config)
	if v.Response != nil {
		response, _, err := configLoader.Load(v.Response)
		if err != nil {
			return nil, errors.Base(err).Message("Blackhole: Failed to parse response config.")
		}
		responseSettings, err := response.(Buildable).Build()
		if err != nil {
			return nil, err
		}
		config.Response = responseSettings
	}

	return loader.NewTypedSettings(config), nil
}

var (
	configLoader = NewJSONConfigLoader(
		ConfigCreatorCache{
			"none": func() interface{} { return new(NoneResponse) },
			"http": func() interface{} { return new(HttpResponse) },
		},
		"type",
		"")
)
