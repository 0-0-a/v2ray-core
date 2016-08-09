package outbound

import (
	"io"
	"sync"

	"github.com/v2ray/v2ray-core/app"
	"github.com/v2ray/v2ray-core/common/alloc"
	v2io "github.com/v2ray/v2ray-core/common/io"
	"github.com/v2ray/v2ray-core/common/log"
	v2net "github.com/v2ray/v2ray-core/common/net"
	"github.com/v2ray/v2ray-core/common/protocol"
	"github.com/v2ray/v2ray-core/common/retry"
	"github.com/v2ray/v2ray-core/proxy"
	"github.com/v2ray/v2ray-core/proxy/internal"
	"github.com/v2ray/v2ray-core/proxy/vmess/encoding"
	vmessio "github.com/v2ray/v2ray-core/proxy/vmess/io"
	"github.com/v2ray/v2ray-core/transport/internet"
	"github.com/v2ray/v2ray-core/transport/ray"
)

type VMessOutboundHandler struct {
	serverList   *protocol.ServerList
	serverPicker protocol.ServerPicker
	meta         *proxy.OutboundHandlerMeta
}

func (this *VMessOutboundHandler) Dispatch(target v2net.Destination, payload *alloc.Buffer, ray ray.OutboundRay) error {
	defer ray.OutboundInput().Release()
	defer ray.OutboundOutput().Close()

	var rec *protocol.ServerSpec
	var conn internet.Connection

	err := retry.Timed(5, 100).On(func() error {
		rec = this.serverPicker.PickServer()
		rawConn, err := internet.Dial(this.meta.Address, rec.Destination(), this.meta.StreamSettings)
		if err != nil {
			return err
		}
		conn = rawConn

		return nil
	})
	if err != nil {
		log.Error("VMess|Outbound: Failed to find an available destination:", err)
		return err
	}
	log.Info("VMess|Outbound: Tunneling request to ", target, " via ", rec.Destination())

	command := protocol.RequestCommandTCP
	if target.IsUDP() {
		command = protocol.RequestCommandUDP
	}
	request := &protocol.RequestHeader{
		Version: encoding.Version,
		User:    rec.PickUser(),
		Command: command,
		Address: target.Address(),
		Port:    target.Port(),
		Option:  protocol.RequestOptionChunkStream,
	}

	defer conn.Close()

	conn.SetReusable(true)
	if conn.Reusable() { // Conn reuse may be disabled on transportation layer
		request.Option.Set(protocol.RequestOptionConnectionReuse)
	}

	input := ray.OutboundInput()
	output := ray.OutboundOutput()

	var requestFinish, responseFinish sync.Mutex
	requestFinish.Lock()
	responseFinish.Lock()

	session := encoding.NewClientSession(protocol.DefaultIDHash)

	go this.handleRequest(session, conn, request, payload, input, &requestFinish)
	go this.handleResponse(session, conn, request, rec.Destination(), output, &responseFinish)

	requestFinish.Lock()
	responseFinish.Lock()
	return nil
}

func (this *VMessOutboundHandler) handleRequest(session *encoding.ClientSession, conn internet.Connection, request *protocol.RequestHeader, payload *alloc.Buffer, input v2io.Reader, finish *sync.Mutex) {
	defer finish.Unlock()

	writer := v2io.NewBufferedWriter(conn)
	defer writer.Release()
	session.EncodeRequestHeader(request, writer)

	bodyWriter := session.EncodeRequestBody(writer)
	var streamWriter v2io.Writer = v2io.NewAdaptiveWriter(bodyWriter)
	if request.Option.Has(protocol.RequestOptionChunkStream) {
		streamWriter = vmessio.NewAuthChunkWriter(streamWriter)
	}
	if err := streamWriter.Write(payload); err != nil {
		conn.SetReusable(false)
	}
	writer.SetCached(false)

	err := v2io.Pipe(input, streamWriter)
	if err != io.EOF {
		conn.SetReusable(false)
	}

	if request.Option.Has(protocol.RequestOptionChunkStream) {
		err := streamWriter.Write(alloc.NewLocalBuffer(32).Clear())
		if err != nil {
			conn.SetReusable(false)
		}
	}
	streamWriter.Release()
	return
}

func (this *VMessOutboundHandler) handleResponse(session *encoding.ClientSession, conn internet.Connection, request *protocol.RequestHeader, dest v2net.Destination, output v2io.Writer, finish *sync.Mutex) {
	defer finish.Unlock()

	reader := v2io.NewBufferedReader(conn)
	defer reader.Release()

	header, err := session.DecodeResponseHeader(reader)
	if err != nil {
		conn.SetReusable(false)
		log.Warning("VMess|Outbound: Failed to read response from ", request.Destination(), ": ", err)
		return
	}
	go this.handleCommand(dest, header.Command)

	if !header.Option.Has(protocol.ResponseOptionConnectionReuse) {
		conn.SetReusable(false)
	}

	reader.SetCached(false)
	decryptReader := session.DecodeResponseBody(reader)

	var bodyReader v2io.Reader
	if request.Option.Has(protocol.RequestOptionChunkStream) {
		bodyReader = vmessio.NewAuthChunkReader(decryptReader)
	} else {
		bodyReader = v2io.NewAdaptiveReader(decryptReader)
	}

	err = v2io.Pipe(bodyReader, output)
	if err != io.EOF {
		conn.SetReusable(false)
	}

	bodyReader.Release()

	return
}

type Factory struct{}

func (this *Factory) StreamCapability() internet.StreamConnectionType {
	return internet.StreamConnectionTypeRawTCP | internet.StreamConnectionTypeTCP | internet.StreamConnectionTypeKCP
}

func (this *Factory) Create(space app.Space, rawConfig interface{}, meta *proxy.OutboundHandlerMeta) (proxy.OutboundHandler, error) {
	vOutConfig := rawConfig.(*Config)

	serverList := protocol.NewServerList()
	for _, rec := range vOutConfig.Receivers {
		serverList.AddServer(rec)
	}
	handler := &VMessOutboundHandler{
		serverList:   serverList,
		serverPicker: protocol.NewRoundRobinServerPicker(serverList),
		meta:         meta,
	}

	return handler, nil
}

func init() {
	internal.MustRegisterOutboundHandlerCreator("vmess", new(Factory))
}
