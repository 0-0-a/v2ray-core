package protocol

import (
	"encoding/binary"
	"errors"

	"github.com/v2ray/v2ray-core/common/log"
	v2net "github.com/v2ray/v2ray-core/common/net"
)

var (
	ErrorUnknownAddressType = errors.New("Unknown Address Type.")
)

type Socks5UDPRequest struct {
	fragment byte
	address  v2net.Address
	data     []byte
}

func (request *Socks5UDPRequest) Destination() v2net.Destination {
	return v2net.NewUDPDestination(request.address)
}

func ReadUDPRequest(packet []byte) (request Socks5UDPRequest, err error) {
	// packet[0] and packet[1] are reserved
	request.fragment = packet[2]

	addrType := packet[3]
	var dataBegin int

	switch addrType {
	case AddrTypeIPv4:
		ip := packet[4:8]
		port := binary.BigEndian.Uint16(packet[8:10])
		request.address = v2net.IPAddress(ip, port)
		dataBegin = 10
	case AddrTypeIPv6:
		ip := packet[4:20]
		port := binary.BigEndian.Uint16(packet[20:22])
		request.address = v2net.IPAddress(ip, port)
		dataBegin = 22
	case AddrTypeDomain:
		domainLength := int(packet[4])
		domain := string(packet[5 : 5+domainLength])
		port := binary.BigEndian.Uint16(packet[5+domainLength : 5+domainLength+2])
		request.address = v2net.DomainAddress(domain, port)
		dataBegin = 5 + domainLength + 2
	default:
		log.Warning("Unknown address type %d", addrType)
		err = ErrorUnknownAddressType
		return
	}

	request.data = make([]byte, len(packet)-dataBegin)
	copy(request.data, packet[dataBegin:])

	return
}
