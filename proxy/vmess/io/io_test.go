package io_test

import (
	"bytes"
	"crypto/rand"
	"io"
	"testing"

	"v2ray.com/core/common/alloc"
	"v2ray.com/core/common/errors"
	v2io "v2ray.com/core/common/io"
	"v2ray.com/core/common/serial"
	. "v2ray.com/core/proxy/vmess/io"
	"v2ray.com/core/testing/assert"
)

func TestAuthenticate(t *testing.T) {
	assert := assert.On(t)

	buffer := alloc.NewBuffer()
	buffer.AppendBytes(1, 2, 3, 4)
	Authenticate(buffer)
	assert.Bytes(buffer.Bytes()).Equals([]byte{0, 8, 87, 52, 168, 125, 1, 2, 3, 4})

	b2, err := NewAuthChunkReader(buffer).Read()
	assert.Error(err).IsNil()
	assert.Bytes(b2.Bytes()).Equals([]byte{1, 2, 3, 4})
}

func TestSingleIO(t *testing.T) {
	assert := assert.On(t)

	content := bytes.NewBuffer(make([]byte, 0, 1024*1024))

	writer := NewAuthChunkWriter(v2io.NewAdaptiveWriter(content))
	b := alloc.NewBuffer()
	b.AppendFunc(serial.WriteString("abcd"))
	writer.Write(b)
	writer.Write(alloc.NewBuffer())
	writer.Release()

	reader := NewAuthChunkReader(content)
	buffer, err := reader.Read()
	assert.Error(err).IsNil()
	assert.String(buffer.String()).Equals("abcd")
}

func TestLargeIO(t *testing.T) {
	assert := assert.On(t)

	content := make([]byte, 1024*1024)
	rand.Read(content)

	chunckContent := bytes.NewBuffer(make([]byte, 0, len(content)*2))
	writer := NewAuthChunkWriter(v2io.NewAdaptiveWriter(chunckContent))
	writeSize := 0
	for {
		chunkSize := 7 * 1024
		if chunkSize+writeSize > len(content) {
			chunkSize = len(content) - writeSize
		}
		b := alloc.NewBuffer()
		b.Append(content[writeSize : writeSize+chunkSize])
		writer.Write(b)
		b.Release()

		writeSize += chunkSize
		if writeSize == len(content) {
			break
		}
	}
	writer.Write(alloc.NewBuffer())
	writer.Release()

	actualContent := make([]byte, 0, len(content))
	reader := NewAuthChunkReader(chunckContent)
	for {
		buffer, err := reader.Read()
		if errors.Cause(err) == io.EOF {
			break
		}
		assert.Error(err).IsNil()
		actualContent = append(actualContent, buffer.Bytes()...)
	}

	assert.Int(len(actualContent)).Equals(len(content))
	assert.Bytes(actualContent).Equals(content)
}
