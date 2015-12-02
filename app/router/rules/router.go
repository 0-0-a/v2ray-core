package rules

import (
	"errors"

	"github.com/v2ray/v2ray-core/app/router"
	"github.com/v2ray/v2ray-core/app/router/rules/config"
	v2net "github.com/v2ray/v2ray-core/common/net"
)

var (
	InvalidRule      = errors.New("Invalid Rule")
	NoRuleApplicable = errors.New("No rule applicable")
)

type Router struct {
	rules []config.Rule
}

func (this *Router) TakeDetour(dest v2net.Destination) (string, error) {
	for _, rule := range this.rules {
		if rule.Apply(dest) {
			return rule.Tag(), nil
		}
	}
	return "", NoRuleApplicable
}

type RouterFactory struct {
}

func (this *RouterFactory) Create(rawConfig interface{}) (router.Router, error) {
	config := rawConfig.(config.RouterRuleConfig)
	rules := config.Rules()
	for _, rule := range rules {
		if rule == nil {
			return nil, InvalidRule
		}
	}
	return &Router{
		rules: rules,
	}, nil
}

func init() {
	router.RegisterRouter("rules", &RouterFactory{})
}
