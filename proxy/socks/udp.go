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

var udpAddress v2net.Address

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
	udpAddress = v2net.IPAddress(conn.LocalAddr().(*net.UDPAddr).IP, uint16(conn.LocalAddr().(*net.UDPAddr).Port))

	go server.AcceptPackets(conn)
	return nil
}

func (server *SocksServer) getUDPAddr() v2net.Address {
	return udpAddress
}

func (server *SocksServer) AcceptPackets(conn *net.UDPConn) error {
	for {
		buffer := make([]byte, bufferSize)
		nBytes, addr, err := conn.ReadFromUDP(buffer)
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

		udpPacket := v2net.NewPacket(request.Destination(), request.Data, false)
		go server.handlePacket(conn, udpPacket, addr, request)
	}
}

func (server *SocksServer) handlePacket(conn *net.UDPConn, packet v2net.Packet, clientAddr *net.UDPAddr, request protocol.Socks5UDPRequest) {
	ray := server.vPoint.DispatchToOutbound(packet)
	close(ray.InboundInput())

	if data, ok := <-ray.InboundOutput(); ok {
		request.Data = data
		udpMessage := request.Bytes(nil)
		nBytes, err := conn.WriteToUDP(udpMessage, clientAddr)
		if err != nil {
			log.Error("Socks failed to write UDP message (%d bytes) to %s: %v", nBytes, clientAddr.String(), err)
		}
	}
}
