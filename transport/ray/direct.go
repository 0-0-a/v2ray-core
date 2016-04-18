package ray

import (
	"io"
	"sync"

	"github.com/v2ray/v2ray-core/common/alloc"
)

const (
	bufferSize = 128
)

// NewRay creates a new Ray for direct traffic transport.
func NewRay() Ray {
	return &directRay{
		Input:  NewStream(),
		Output: NewStream(),
	}
}

type directRay struct {
	Input  *Stream
	Output *Stream
}

func (this *directRay) OutboundInput() InputStream {
	return this.Input
}

func (this *directRay) OutboundOutput() OutputStream {
	return this.Output
}

func (this *directRay) InboundInput() OutputStream {
	return this.Input
}

func (this *directRay) InboundOutput() InputStream {
	return this.Output
}

type Stream struct {
	access sync.RWMutex
	closed bool
	buffer chan *alloc.Buffer
}

func NewStream() *Stream {
	return &Stream{
		buffer: make(chan *alloc.Buffer, bufferSize),
	}
}

func (this *Stream) Read() (*alloc.Buffer, error) {
	if this.buffer == nil {
		return nil, io.EOF
	}
	this.access.RLock()
	defer this.access.RUnlock()
	if this.buffer == nil {
		return nil, io.EOF
	}
	result, open := <-this.buffer
	if !open {
		return nil, io.EOF
	}
	return result, nil
}

func (this *Stream) Write(data *alloc.Buffer) error {
	if this.closed {
		return io.EOF
	}
	if this.buffer == nil {
		return io.EOF
	}
	this.access.RLock()
	defer this.access.RUnlock()
	if this.buffer == nil {
		return io.EOF
	}
	this.buffer <- data
	return nil
}

func (this *Stream) Close() {
	if this.closed {
		return
	}
	this.access.RLock()
	defer this.access.RUnlock()
	if this.closed {
		return
	}
	this.closed = true
	close(this.buffer)
}

func (this *Stream) Release() {
	if this.buffer == nil {
		return
	}
	this.Close()
	this.access.Lock()
	defer this.access.Unlock()
	if this.buffer == nil {
		return
	}
	for data := range this.buffer {
		data.Release()
	}
	this.buffer = nil
}
