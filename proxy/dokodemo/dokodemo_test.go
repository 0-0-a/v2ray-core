package dokodemo_test

import (
	"net"
	"testing"

	v2nettesting "github.com/v2ray/v2ray-core/common/net/testing"
	_ "github.com/v2ray/v2ray-core/proxy/freedom"
	"github.com/v2ray/v2ray-core/shell/point"
	v2testing "github.com/v2ray/v2ray-core/testing"
	"github.com/v2ray/v2ray-core/testing/assert"
	"github.com/v2ray/v2ray-core/testing/servers/tcp"
	"github.com/v2ray/v2ray-core/testing/servers/udp"
)

func TestDokodemoTCP(t *testing.T) {
	v2testing.Current(t)

	port := v2nettesting.PickPort()

	data2Send := "Data to be sent to remote."

	tcpServer := &tcp.Server{
		Port: port,
		MsgProcessor: func(data []byte) []byte {
			buffer := make([]byte, 0, 2048)
			buffer = append(buffer, []byte("Processed: ")...)
			buffer = append(buffer, data...)
			return buffer
		},
	}
	_, err := tcpServer.Start()
	assert.Error(err).IsNil()

	pointPort := v2nettesting.PickPort()
	config := &point.Config{
		Port: pointPort,
		InboundConfig: &point.ConnectionConfig{
			Protocol: "dokodemo-door",
			Settings: []byte(`{
        "address": "127.0.0.1",
        "port": ` + port.String() + `,
        "network": "tcp",
        "timeout": 0
      }`),
		},
		OutboundConfig: &point.ConnectionConfig{
			Protocol: "freedom",
			Settings: nil,
		},
	}

	point, err := point.NewPoint(config)
	assert.Error(err).IsNil()

	err = point.Start()
	assert.Error(err).IsNil()

	tcpClient, err := net.DialTCP("tcp", nil, &net.TCPAddr{
		IP:   []byte{127, 0, 0, 1},
		Port: int(pointPort),
		Zone: "",
	})
	assert.Error(err).IsNil()

	tcpClient.Write([]byte(data2Send))
	tcpClient.CloseWrite()

	response := make([]byte, 1024)
	nBytes, err := tcpClient.Read(response)
	assert.Error(err).IsNil()
	tcpClient.Close()

	assert.StringLiteral("Processed: " + data2Send).Equals(string(response[:nBytes]))
}

func TestDokodemoUDP(t *testing.T) {
	v2testing.Current(t)

	port := v2nettesting.PickPort()

	data2Send := "Data to be sent to remote."

	udpServer := &udp.Server{
		Port: port,
		MsgProcessor: func(data []byte) []byte {
			buffer := make([]byte, 0, 2048)
			buffer = append(buffer, []byte("Processed: ")...)
			buffer = append(buffer, data...)
			return buffer
		},
	}
	_, err := udpServer.Start()
	assert.Error(err).IsNil()

	pointPort := v2nettesting.PickPort()
	config := &point.Config{
		Port: pointPort,
		InboundConfig: &point.ConnectionConfig{
			Protocol: "dokodemo-door",
			Settings: []byte(`{
        "address": "127.0.0.1",
        "port": ` + port.String() + `,
        "network": "udp",
        "timeout": 0
      }`),
		},
		OutboundConfig: &point.ConnectionConfig{
			Protocol: "freedom",
			Settings: nil,
		},
	}

	point, err := point.NewPoint(config)
	assert.Error(err).IsNil()

	err = point.Start()
	assert.Error(err).IsNil()

	udpClient, err := net.DialUDP("udp", nil, &net.UDPAddr{
		IP:   []byte{127, 0, 0, 1},
		Port: int(pointPort),
		Zone: "",
	})
	assert.Error(err).IsNil()

	udpClient.Write([]byte(data2Send))

	response := make([]byte, 1024)
	nBytes, err := udpClient.Read(response)
	assert.Error(err).IsNil()
	udpClient.Close()

	assert.StringLiteral("Processed: " + data2Send).Equals(string(response[:nBytes]))
}
