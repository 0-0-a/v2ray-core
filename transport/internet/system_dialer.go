package internet

import (
	"net"
	"time"

	v2net "github.com/v2ray/v2ray-core/common/net"
)

var (
	effectiveSystemDialer SystemDialer
)

type SystemDialer interface {
	Dial(source v2net.Address, destination v2net.Destination) (net.Conn, error)
}

type DefaultSystemDialer struct {
}

func (this *DefaultSystemDialer) Dial(src v2net.Address, dest v2net.Destination) (net.Conn, error) {
	dialer := &net.Dialer{
		Timeout:   time.Second * 60,
		DualStack: true,
	}
	if src != nil && src != v2net.AnyIP {
		var addr net.Addr
		if dest.IsTCP() {
			addr = &net.TCPAddr{
				IP:   src.IP(),
				Port: 0,
			}
		} else {
			addr = &net.UDPAddr{
				IP:   src.IP(),
				Port: 0,
			}
		}
		dialer.LocalAddr = addr
	}
	return dialer.Dial(dest.Network().String(), dest.NetAddr())
}

type SystemDialerAdapter interface {
	Dial(network string, address string) (net.Conn, error)
}

type SimpleSystemDialer struct {
	adapter SystemDialerAdapter
}

func (this *SimpleSystemDialer) Dial(src v2net.Address, dest v2net.Destination) (net.Conn, error) {
	return this.adapter.Dial(dest.Network().String(), dest.NetAddr())
}

func UseAlternativeSystemDialer(dialer SystemDialer) {
	effectiveSystemDialer = dialer
}

func UseAlternativeSimpleSystemDialer(dialer SystemDialerAdapter) {
	effectiveSystemDialer = &SimpleSystemDialer{
		adapter: dialer,
	}
}

// @Deprecated: Use UseAlternativeSimpleSystemDialer.
func SubstituteDialer(dialer SystemDialerAdapter) error {
	UseAlternativeSimpleSystemDialer(dialer)
	return nil
}

func init() {
	effectiveSystemDialer = &DefaultSystemDialer{}
}
