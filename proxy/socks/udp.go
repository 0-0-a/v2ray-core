package socks

import (
	"net"

	"github.com/v2ray/v2ray-core/common/log"
	v2net "github.com/v2ray/v2ray-core/common/net"
	"github.com/v2ray/v2ray-core/proxy/socks/protocol"
)

const (
	bufferSize = 2 * 1024
)

func (server *SocksServer) ListenUDP(port uint16) error {
	addr := &net.UDPAddr{
		IP:   net.IP{0, 0, 0, 0},
		Port: int(port),
		Zone: "",
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Error("Socks failed to listen UDP on port %d: %v", port, err)
		return err
	}

	go server.AcceptPackets(conn)
	return nil
}

func (server *SocksServer) AcceptPackets(conn *net.UDPConn) error {
	for {
		buffer := make([]byte, 0, bufferSize)
		nBytes, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Error("Socks failed to read UDP packets: %v", err)
			return err
		}
		request, err := protocol.ReadUDPRequest(buffer[:nBytes])
		if err != nil {
			log.Error("Socks failed to parse UDP request: %v", err)
			return err
		}
		if request.Fragment != 0 {
			// TODO handle fragments
			continue
		}

		udpPacket := v2net.NewUDPPacket(request.Destination(), request.Data)
		server.vPoint.DispatchToOutbound(udpPacket)
	}
}
