package protocol

import (
	"encoding/binary"
	"io"

	"github.com/v2ray/v2ray-core/common/errors"
	"github.com/v2ray/v2ray-core/common/log"
	v2net "github.com/v2ray/v2ray-core/common/net"
)

const (
	socksVersion  = byte(0x05)
	socks4Version = byte(0x04)

	AuthNotRequired      = byte(0x00)
	AuthGssApi           = byte(0x01)
	AuthUserPass         = byte(0x02)
	AuthNoMatchingMethod = byte(0xFF)

	Socks4RequestGranted  = byte(90)
	Socks4RequestRejected = byte(91)
)

// Authentication request header of Socks5 protocol
type Socks5AuthenticationRequest struct {
	version     byte
	nMethods    byte
	authMethods [256]byte
}

func (request *Socks5AuthenticationRequest) HasAuthMethod(method byte) bool {
	for i := 0; i < int(request.nMethods); i++ {
		if request.authMethods[i] == method {
			return true
		}
	}
	return false
}

func ReadAuthentication(reader io.Reader) (auth Socks5AuthenticationRequest, auth4 Socks4AuthenticationRequest, err error) {
	buffer := make([]byte, 256)
	nBytes, err := reader.Read(buffer)
	if err != nil {
		return
	}
	if nBytes < 2 {
		log.Info("Socks expected 2 bytes read, but only %d bytes read", nBytes)
		err = errors.NewCorruptedPacketError()
		return
	}

	if buffer[0] == socks4Version {
		auth4.Version = buffer[0]
		auth4.Command = buffer[1]
		auth4.Port = binary.BigEndian.Uint16(buffer[2:4])
		copy(auth4.IP[:], buffer[4:8])
		err = NewSocksVersion4Error()
		return
	}

	auth.version = buffer[0]
	if auth.version != socksVersion {
		err = errors.NewProtocolVersionError(int(auth.version))
		return
	}

	auth.nMethods = buffer[1]
	if auth.nMethods <= 0 {
		log.Info("Zero length of authentication methods")
		err = errors.NewCorruptedPacketError()
		return
	}

	if nBytes-2 != int(auth.nMethods) {
		log.Info("Unmatching number of auth methods, expecting %d, but got %d", auth.nMethods, nBytes)
		err = errors.NewCorruptedPacketError()
		return
	}
	copy(auth.authMethods[:], buffer[2:nBytes])
	return
}

type Socks5AuthenticationResponse struct {
	version    byte
	authMethod byte
}

func NewAuthenticationResponse(authMethod byte) *Socks5AuthenticationResponse {
	return &Socks5AuthenticationResponse{
		version:    socksVersion,
		authMethod: authMethod,
	}
}

func WriteAuthentication(writer io.Writer, r *Socks5AuthenticationResponse) error {
	_, err := writer.Write([]byte{r.version, r.authMethod})
	return err
}

type Socks5UserPassRequest struct {
	version  byte
	username string
	password string
}

func (request Socks5UserPassRequest) IsValid(username string, password string) bool {
	return request.username == username && request.password == password
}

func (request Socks5UserPassRequest) AuthDetail() string {
	return request.username + ":" + request.password
}

func ReadUserPassRequest(reader io.Reader) (request Socks5UserPassRequest, err error) {
	buffer := make([]byte, 256)
	_, err = reader.Read(buffer[0:2])
	if err != nil {
		return
	}
	request.version = buffer[0]
	nUsername := buffer[1]
	nBytes, err := reader.Read(buffer[:nUsername])
	if err != nil {
		return
	}
	request.username = string(buffer[:nBytes])

	_, err = reader.Read(buffer[0:1])
	if err != nil {
		return
	}
	nPassword := buffer[0]
	nBytes, err = reader.Read(buffer[:nPassword])
	if err != nil {
		return
	}
	request.password = string(buffer[:nBytes])
	return
}

type Socks5UserPassResponse struct {
	version byte
	status  byte
}

func NewSocks5UserPassResponse(status byte) Socks5UserPassResponse {
	return Socks5UserPassResponse{
		version: socksVersion,
		status:  status,
	}
}

func WriteUserPassResponse(writer io.Writer, response Socks5UserPassResponse) error {
	_, err := writer.Write([]byte{response.version, response.status})
	return err
}

const (
	AddrTypeIPv4   = byte(0x01)
	AddrTypeIPv6   = byte(0x04)
	AddrTypeDomain = byte(0x03)

	CmdConnect      = byte(0x01)
	CmdBind         = byte(0x02)
	CmdUdpAssociate = byte(0x03)
)

type Socks5Request struct {
	Version  byte
	Command  byte
	AddrType byte
	IPv4     [4]byte
	Domain   string
	IPv6     [16]byte
	Port     uint16
}

func ReadRequest(reader io.Reader) (request *Socks5Request, err error) {
	buffer := make([]byte, 256)
	nBytes, err := reader.Read(buffer[:4])
	if err != nil {
		return
	}
	if nBytes < 4 {
		err = errors.NewCorruptedPacketError()
		return
	}
	request = &Socks5Request{
		Version: buffer[0],
		Command: buffer[1],
		// buffer[2] is a reserved field
		AddrType: buffer[3],
	}
	switch request.AddrType {
	case AddrTypeIPv4:
		nBytes, err = reader.Read(request.IPv4[:])
		if err != nil {
			return
		}
		if nBytes != 4 {
			err = errors.NewCorruptedPacketError()
			return
		}
	case AddrTypeDomain:
		nBytes, err = reader.Read(buffer[0:1])
		if err != nil {
			return
		}
		domainLength := buffer[0]
		nBytes, err = reader.Read(buffer[:domainLength])
		if err != nil {
			return
		}

		if nBytes != int(domainLength) {
			log.Info("Unable to read domain with %d bytes, expecting %d bytes", nBytes, domainLength)
			err = errors.NewCorruptedPacketError()
			return
		}
		request.Domain = string(buffer[:domainLength])
	case AddrTypeIPv6:
		nBytes, err = reader.Read(request.IPv6[:])
		if err != nil {
			return
		}
		if nBytes != 16 {
			err = errors.NewCorruptedPacketError()
			return
		}
	default:
		log.Info("Unexpected address type %d", request.AddrType)
		err = errors.NewCorruptedPacketError()
		return
	}

	nBytes, err = reader.Read(buffer[:2])
	if err != nil {
		return
	}
	if nBytes != 2 {
		err = errors.NewCorruptedPacketError()
		return
	}

	request.Port = binary.BigEndian.Uint16(buffer)
	return
}

func (request *Socks5Request) Destination() v2net.Destination {
	var address v2net.Address
	switch request.AddrType {
	case AddrTypeIPv4:
		address = v2net.IPAddress(request.IPv4[:], request.Port)
	case AddrTypeIPv6:
		address = v2net.IPAddress(request.IPv6[:], request.Port)
	case AddrTypeDomain:
		address = v2net.DomainAddress(request.Domain, request.Port)
	default:
		panic("Unknown address type")
	}
	return v2net.NewTCPDestination(address)
}

const (
	ErrorSuccess                 = byte(0x00)
	ErrorGeneralFailure          = byte(0x01)
	ErrorConnectionNotAllowed    = byte(0x02)
	ErrorNetworkUnreachable      = byte(0x03)
	ErrorHostUnUnreachable       = byte(0x04)
	ErrorConnectionRefused       = byte(0x05)
	ErrorTTLExpired              = byte(0x06)
	ErrorCommandNotSupported     = byte(0x07)
	ErrorAddressTypeNotSupported = byte(0x08)
)

type Socks5Response struct {
	Version  byte
	Error    byte
	AddrType byte
	IPv4     [4]byte
	Domain   string
	IPv6     [16]byte
	Port     uint16
}

func NewSocks5Response() *Socks5Response {
	return &Socks5Response{
		Version: socksVersion,
	}
}

func (r *Socks5Response) SetIPv4(ipv4 []byte) {
	r.AddrType = AddrTypeIPv4
	copy(r.IPv4[:], ipv4)
}

func (r *Socks5Response) SetIPv6(ipv6 []byte) {
	r.AddrType = AddrTypeIPv6
	copy(r.IPv6[:], ipv6)
}

func (r *Socks5Response) SetDomain(domain string) {
	r.AddrType = AddrTypeDomain
	r.Domain = domain
}

func (r *Socks5Response) toBytes() []byte {
	buffer := make([]byte, 0, 300)
	buffer = append(buffer, r.Version)
	buffer = append(buffer, r.Error)
	buffer = append(buffer, 0x00) // reserved
	buffer = append(buffer, r.AddrType)
	switch r.AddrType {
	case 0x01:
		buffer = append(buffer, r.IPv4[:]...)
	case 0x03:
		buffer = append(buffer, byte(len(r.Domain)))
		buffer = append(buffer, []byte(r.Domain)...)
	case 0x04:
		buffer = append(buffer, r.IPv6[:]...)
	}
	buffer = append(buffer, byte(r.Port>>8), byte(r.Port))
	return buffer
}

func WriteResponse(writer io.Writer, response *Socks5Response) error {
	_, err := writer.Write(response.toBytes())
	return err
}
