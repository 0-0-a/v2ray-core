package internet

import (
	"errors"
	"net"
	"sync"

	"github.com/v2ray/v2ray-core/common/log"
	v2net "github.com/v2ray/v2ray-core/common/net"
)

var (
	ErrorClosedConnection = errors.New("Connection already closed.")

	KCPListenFunc    ListenFunc
	TCPListenFunc    ListenFunc
	RawTCPListenFunc ListenFunc
)

type ListenFunc func(address v2net.Address, port v2net.Port) (Listener, error)
type Listener interface {
	Accept() (Connection, error)
	Close() error
	Addr() net.Addr
}

type TCPHub struct {
	sync.Mutex
	listener     Listener
	connCallback ConnectionHandler
	accepting    bool
}

func ListenTCP(address v2net.Address, port v2net.Port, callback ConnectionHandler, settings *StreamSettings) (*TCPHub, error) {
	var listener Listener
	var err error
	switch {
	case settings.IsCapableOf(StreamConnectionTypeTCP):
		listener, err = TCPListenFunc(address, port)
	case settings.IsCapableOf(StreamConnectionTypeKCP):
		listener, err = KCPListenFunc(address, port)
	case settings.IsCapableOf(StreamConnectionTypeRawTCP):
		listener, err = RawTCPListenFunc(address, port)
	default:
		log.Error("Internet|Listener: Unknown stream type: ", settings.Type)
		err = ErrUnsupportedStreamType
	}

	if err != nil {
		log.Warning("Internet|Listener: Failed to listen on ", address, ":", port)
		return nil, err
	}

	hub := &TCPHub{
		listener:     listener,
		connCallback: callback,
	}

	go hub.start()
	return hub, nil
}

func (this *TCPHub) Close() {
	this.accepting = false
	this.listener.Close()
}

func (this *TCPHub) start() {
	this.accepting = true
	for this.accepting {
		conn, err := this.listener.Accept()

		if err != nil {
			if this.accepting {
				log.Warning("Internet|Listener: Failed to accept new TCP connection: ", err)
			}
			continue
		}
		go this.connCallback(conn)
	}
}
