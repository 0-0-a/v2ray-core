package buf

import (
	"io"

	"v2ray.com/core/common"
	"v2ray.com/core/common/errors"
	"v2ray.com/core/common/serial"
)

// ReadAllToMultiBuffer reads all content from the reader into a MultiBuffer, until EOF.
func ReadAllToMultiBuffer(reader io.Reader) (MultiBuffer, error) {
	mb := make(MultiBuffer, 0, 128)

	if _, err := mb.ReadFrom(reader); err != nil {
		ReleaseMulti(mb)
		return nil, err
	}

	return mb, nil
}

// ReadAllToBytes reads all content from the reader into a byte array, until EOF.
func ReadAllToBytes(reader io.Reader) ([]byte, error) {
	mb, err := ReadAllToMultiBuffer(reader)
	if err != nil {
		return nil, err
	}
	if mb.Len() == 0 {
		return nil, nil
	}
	b := make([]byte, mb.Len())
	common.Must2(mb.Read(b))
	ReleaseMulti(mb)
	return b, nil
}

// MultiBuffer is a list of Buffers. The order of Buffer matters.
type MultiBuffer []*Buffer

// MergeMulti merges content from src to dest, and returns the new address of dest and src
func MergeMulti(dest MultiBuffer, src MultiBuffer) (MultiBuffer, MultiBuffer) {
	dest = append(dest, src...)
	for idx := range src {
		src[idx] = nil
	}
	return dest, src[:0]
}

// ReleaseMulti release all content of the MultiBuffer, and returns an empty MultiBuffer.
func ReleaseMulti(mb MultiBuffer) MultiBuffer {
	for i := range mb {
		mb[i].Release()
		mb[i] = nil
	}
	return mb[:0]
}

// Copy copied the beginning part of the MultiBuffer into the given byte array.
func (mb MultiBuffer) Copy(b []byte) int {
	total := 0
	for _, bb := range mb {
		nBytes := copy(b[total:], bb.Bytes())
		total += nBytes
		if int32(nBytes) < bb.Len() {
			break
		}
	}
	return total
}

// ReadFrom implements io.ReaderFrom.
func (mb *MultiBuffer) ReadFrom(reader io.Reader) (int64, error) {
	totalBytes := int64(0)

	for {
		b := New()
		_, err := b.ReadFullFrom(reader, Size)
		if b.IsEmpty() {
			b.Release()
		} else {
			*mb = append(*mb, b)
		}
		totalBytes += int64(b.Len())
		if err != nil {
			if errors.Cause(err) == io.EOF || errors.Cause(err) == io.ErrUnexpectedEOF {
				return totalBytes, nil
			}
			return totalBytes, err
		}
	}
}

// Read implements io.Reader.
func (mb *MultiBuffer) Read(b []byte) (int, error) {
	if mb.IsEmpty() {
		return 0, io.EOF
	}
	endIndex := len(*mb)
	totalBytes := 0
	for i, bb := range *mb {
		nBytes, _ := bb.Read(b)
		totalBytes += nBytes
		b = b[nBytes:]
		if bb.IsEmpty() {
			bb.Release()
			(*mb)[i] = nil
		} else {
			endIndex = i
			break
		}
	}
	*mb = (*mb)[endIndex:]
	return totalBytes, nil
}

// WriteTo implements io.WriterTo.
func (mb *MultiBuffer) WriteTo(writer io.Writer) (int64, error) {
	defer func() {
		*mb = ReleaseMulti(*mb)
	}()

	totalBytes := int64(0)
	for _, b := range *mb {
		nBytes, err := writer.Write(b.Bytes())
		totalBytes += int64(nBytes)
		if err != nil {
			return totalBytes, err
		}
	}

	return totalBytes, nil
}

// Write implements io.Writer.
func (mb *MultiBuffer) Write(b []byte) (int, error) {
	totalBytes := len(b)

	n := len(*mb)
	if n > 0 && !(*mb)[n-1].IsFull() {
		nBytes, _ := (*mb)[n-1].Write(b)
		b = b[nBytes:]
	}

	for len(b) > 0 {
		bb := New()
		nBytes, _ := bb.Write(b)
		b = b[nBytes:]
		*mb = append(*mb, bb)
	}

	return totalBytes, nil
}

// WriteMultiBuffer implements Writer.
func (mb *MultiBuffer) WriteMultiBuffer(b MultiBuffer) error {
	*mb, _ = MergeMulti(*mb, b)
	return nil
}

// Len returns the total number of bytes in the MultiBuffer.
func (mb MultiBuffer) Len() int32 {
	if mb == nil {
		return 0
	}

	size := int32(0)
	for _, b := range mb {
		size += b.Len()
	}
	return size
}

// IsEmpty return true if the MultiBuffer has no content.
func (mb MultiBuffer) IsEmpty() bool {
	for _, b := range mb {
		if !b.IsEmpty() {
			return false
		}
	}
	return true
}

// String returns the content of the MultiBuffer in string.
func (mb MultiBuffer) String() string {
	v := make([]interface{}, len(mb))
	for i, b := range mb {
		v[i] = b
	}
	return serial.Concat(v...)
}

// SliceBySize splits the beginning of this MultiBuffer into another one, for at most size bytes.
func (mb *MultiBuffer) SliceBySize(size int32) MultiBuffer {
	slice := make(MultiBuffer, 0, 10)
	sliceSize := int32(0)
	endIndex := len(*mb)
	for i, b := range *mb {
		if b.Len()+sliceSize > size {
			endIndex = i
			break
		}
		sliceSize += b.Len()
		slice = append(slice, b)
		(*mb)[i] = nil
	}
	*mb = (*mb)[endIndex:]
	if endIndex == 0 && len(*mb) > 0 {
		b := New()
		common.Must2(b.ReadFullFrom((*mb)[0], size))
		return MultiBuffer{b}
	}
	return slice
}

// SplitFirst splits out the first Buffer in this MultiBuffer.
func (mb *MultiBuffer) SplitFirst() *Buffer {
	if len(*mb) == 0 {
		return nil
	}
	b := (*mb)[0]
	(*mb)[0] = nil
	*mb = (*mb)[1:]
	return b
}
