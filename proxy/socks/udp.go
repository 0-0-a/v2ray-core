package socks

import (
	"net"

	"github.com/v2ray/v2ray-core/common/alloc"
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
	// TODO: make this configurable
	udpAddress = v2net.IPAddress([]byte{127, 0, 0, 1}, port)

	go server.AcceptPackets(conn)
	return nil
}

func (server *SocksServer) getUDPAddr() v2net.Address {
	return udpAddress
}

func (server *SocksServer) AcceptPackets(conn *net.UDPConn) error {
	for {
		buffer := alloc.NewBuffer()
		nBytes, addr, err := conn.ReadFromUDP(buffer.Value)
		if err != nil {
			log.Error("Socks failed to read UDP packets: %v", err)
			buffer.Release()
			continue
		}
		buffer.Slice(0, nBytes)
		log.Info("Client UDP connection from %v", addr)
		request, err := protocol.ReadUDPRequest(buffer.Value)
		buffer.Release()
		if err != nil {
			log.Error("Socks failed to parse UDP request: %v", err)
			request.Data.Release()
			continue
		}
		if request.Fragment != 0 {
			log.Warning("Dropping framented UDP packets.")
			// TODO handle fragments
			request.Data.Release()
			continue
		}

		udpPacket := v2net.NewPacket(request.Destination(), request.Data, false)
		log.Info("Send packet to %s with %d bytes", udpPacket.Destination().String(), request.Data.Len())
		go server.handlePacket(conn, udpPacket, addr, request.Address)
	}
}

func (server *SocksServer) handlePacket(conn *net.UDPConn, packet v2net.Packet, clientAddr *net.UDPAddr, targetAddr v2net.Address) {
	ray := server.vPoint.DispatchToOutbound(packet)
	close(ray.InboundInput())

	if data, ok := <-ray.InboundOutput(); ok {
		response := &protocol.Socks5UDPRequest{
			Fragment: 0,
			Address:  targetAddr,
			Data:     data,
		}
		log.Info("Writing back UDP response with %d bytes from %s to %s", data.Len(), targetAddr.String(), clientAddr.String())
		udpMessage := response.Bytes(nil)
		nBytes, err := conn.WriteToUDP(udpMessage, clientAddr)
		response.Data.Release()
		if err != nil {
			log.Error("Socks failed to write UDP message (%d bytes) to %s: %v", nBytes, clientAddr.String(), err)
		}
	}
}
