package mocks

import (
	"io"
	"sync"

	"v2ray.com/core/app/dispatcher"
	"v2ray.com/core/common/buf"
	v2net "v2ray.com/core/common/net"
	"v2ray.com/core/proxy"
)

type InboundConnectionHandler struct {
	ListeningPort    v2net.Port
	ListeningAddress v2net.Address
	PacketDispatcher dispatcher.PacketDispatcher
	ConnInput        io.Reader
	ConnOutput       io.Writer
}

func (v *InboundConnectionHandler) Start() error {
	return nil
}

func (v *InboundConnectionHandler) Port() v2net.Port {
	return v.ListeningPort
}

func (v *InboundConnectionHandler) Close() {

}

func (v *InboundConnectionHandler) Communicate(destination v2net.Destination) error {
	ray := v.PacketDispatcher.DispatchToOutbound(&proxy.SessionInfo{
		Source:      v2net.TCPDestination(v2net.LocalHostIP, v2net.Port(0)),
		Destination: destination,
		Inbound: &proxy.InboundHandlerMeta{
			AllowPassiveConnection: false,
		},
	})

	input := ray.InboundInput()
	output := ray.InboundOutput()

	readFinish := &sync.Mutex{}
	writeFinish := &sync.Mutex{}

	readFinish.Lock()
	writeFinish.Lock()

	go func() {
		v2reader := buf.NewReader(v.ConnInput)
		defer v2reader.Release()

		buf.Pipe(v2reader, input)
		input.Close()
		readFinish.Unlock()
	}()

	go func() {
		v2writer := buf.NewWriter(v.ConnOutput)
		defer v2writer.Release()

		buf.Pipe(output, v2writer)
		output.Release()
		writeFinish.Unlock()
	}()

	readFinish.Lock()
	writeFinish.Lock()
	return nil
}
