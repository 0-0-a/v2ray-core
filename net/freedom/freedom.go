package freedom

import (
	"io"
	"net"

	"github.com/v2ray/v2ray-core"
	"github.com/v2ray/v2ray-core/log"
	v2net "github.com/v2ray/v2ray-core/net"
)

type VFreeConnection struct {
	dest v2net.VAddress
}

func NewVFreeConnection(dest v2net.VAddress) *VFreeConnection {
	conn := new(VFreeConnection)
	conn.dest = dest
	return conn
}

func (vconn *VFreeConnection) Start(vRay core.OutboundVRay) error {
	input := vRay.OutboundInput()
	output := vRay.OutboundOutput()
	conn, err := net.Dial("tcp", vconn.dest.String())
	if err != nil {
		return log.Error("Failed to open tcp: %s", vconn.dest.String())
	}
	log.Debug("Sending outbound tcp: %s", vconn.dest.String())

	finish := make(chan bool, 2)
	go vconn.DumpInput(conn, input, finish)
	go vconn.DumpOutput(conn, output, finish)
	go vconn.CloseConn(conn, finish)
	return nil
}

func (vconn *VFreeConnection) DumpInput(conn net.Conn, input <-chan []byte, finish chan<- bool) {
	for {
		data, open := <-input
		if !open {
			finish <- true
			log.Debug("Freedom finishing input.")
			break
		}
		nBytes, err := conn.Write(data)
		log.Debug("Freedom wrote %d bytes with error %v", nBytes, err)
	}
}

func (vconn *VFreeConnection) DumpOutput(conn net.Conn, output chan<- []byte, finish chan<- bool) {
	for {
		buffer := make([]byte, 512)
		nBytes, err := conn.Read(buffer)
		log.Debug("Freedom reading %d bytes with error %v", nBytes, err)
		if err == io.EOF {
			close(output)
			finish <- true
			log.Debug("Freedom finishing output.")
			break
		}
		output <- buffer[:nBytes]
	}
}

func (vconn *VFreeConnection) CloseConn(conn net.Conn, finish <-chan bool) {
	for i := 0; i < 2; i++ {
		<-finish
	}
	conn.Close()
}
