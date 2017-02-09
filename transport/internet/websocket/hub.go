package websocket

import (
	"crypto/tls"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"v2ray.com/core/app/log"
	"v2ray.com/core/common"
	"v2ray.com/core/common/errors"
	v2net "v2ray.com/core/common/net"
	"v2ray.com/core/transport/internet"
	"v2ray.com/core/transport/internet/internal"
	v2tls "v2ray.com/core/transport/internet/tls"
)

var (
	ErrClosedListener = errors.New("Listener is closed.")
)

type ConnectionWithError struct {
	conn net.Conn
	err  error
}

type WSListener struct {
	sync.Mutex
	acccepting    bool
	awaitingConns chan *ConnectionWithError
	listener      net.Listener
	tlsConfig     *tls.Config
	config        *Config
}

func ListenWS(address v2net.Address, port v2net.Port, options internet.ListenOptions) (internet.Listener, error) {
	networkSettings, err := options.Stream.GetEffectiveTransportSettings()
	if err != nil {
		return nil, err
	}
	wsSettings := networkSettings.(*Config)

	l := &WSListener{
		acccepting:    true,
		awaitingConns: make(chan *ConnectionWithError, 32),
		config:        wsSettings,
	}
	if options.Stream != nil && options.Stream.HasSecuritySettings() {
		securitySettings, err := options.Stream.GetEffectiveSecuritySettings()
		if err != nil {
			log.Error("WebSocket: Failed to create apply TLS config: ", err)
			return nil, err
		}
		tlsConfig, ok := securitySettings.(*v2tls.Config)
		if ok {
			l.tlsConfig = tlsConfig.GetTLSConfig()
		}
	}

	err = l.listenws(address, port)

	return l, err
}

func (wsl *WSListener) listenws(address v2net.Address, port v2net.Port) error {
	http.HandleFunc("/"+wsl.config.Path, func(w http.ResponseWriter, r *http.Request) {
		conn, err := wsl.converttovws(w, r)
		if err != nil {
			log.Warning("WebSocket|Listener: Failed to convert connection: ", err)
			return
		}

		select {
		case wsl.awaitingConns <- &ConnectionWithError{
			conn: conn,
		}:
		default:
			if conn != nil {
				conn.Close()
			}
		}
		return
	})

	netAddr := address.String() + ":" + strconv.Itoa(int(port.Value()))
	var listener net.Listener
	if wsl.tlsConfig == nil {
		l, err := net.Listen("tcp", netAddr)
		if err != nil {
			return errors.Base(err).Message("WebSocket|Listener: Failed to listen TCP ", netAddr)
		}
		listener = l
	} else {
		l, err := tls.Listen("tcp", netAddr, wsl.tlsConfig)
		if err != nil {
			return errors.Base(err).Message("WebSocket|Listener: Failed to listen TLS ", netAddr)
		}
		listener = l
	}
	wsl.listener = listener

	go func() {
		http.Serve(listener, nil)
	}()

	return nil
}

func (wsl *WSListener) converttovws(w http.ResponseWriter, r *http.Request) (*wsconn, error) {
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  32 * 1024,
		WriteBufferSize: 32 * 1024,
	}
	conn, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		return nil, err
	}

	return &wsconn{wsc: conn}, nil
}

func (v *WSListener) Accept() (internet.Connection, error) {
	for v.acccepting {
		select {
		case connErr, open := <-v.awaitingConns:
			if !open {
				return nil, ErrClosedListener
			}
			if connErr.err != nil {
				return nil, connErr.err
			}
			return internal.NewConnection(internal.ConnectionID{}, connErr.conn, v, internal.ReuseConnection(v.config.IsConnectionReuse())), nil
		case <-time.After(time.Second * 2):
		}
	}
	return nil, ErrClosedListener
}

func (v *WSListener) Put(id internal.ConnectionID, conn net.Conn) {
	v.Lock()
	defer v.Unlock()
	if !v.acccepting {
		return
	}
	select {
	case v.awaitingConns <- &ConnectionWithError{conn: conn}:
	default:
		conn.Close()
	}
}

func (v *WSListener) Addr() net.Addr {
	return nil
}

func (v *WSListener) Close() error {
	v.Lock()
	defer v.Unlock()
	v.acccepting = false

	v.listener.Close()

	close(v.awaitingConns)
	for connErr := range v.awaitingConns {
		if connErr.conn != nil {
			connErr.conn.Close()
		}
	}
	return nil
}

func init() {
	common.Must(internet.RegisterTransportListener(internet.TransportProtocol_WebSocket, ListenWS))
}
