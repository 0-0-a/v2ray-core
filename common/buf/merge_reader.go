package buf

type MergingReader struct {
	reader        Reader
	timeoutReader TimeoutReader
	leftover      *Buffer
}

func NewMergingReader(reader Reader) Reader {
	return &MergingReader{
		reader:        reader,
		timeoutReader: reader.(TimeoutReader),
	}
}

func (r *MergingReader) Read() (*Buffer, error) {
	if r.leftover != nil {
		return r.leftover, nil
	}

	b, err := r.reader.Read()
	if err != nil {
		return nil, err
	}

	if b.IsFull() {
		return b, nil
	}

	if r.timeoutReader == nil {
		return b, nil
	}

	b2, err := r.timeoutReader.ReadTimeout(0)
	if err != nil {
		return b, nil
	}

	nBytes := b.Append(b2.Bytes())
	b2.SliceFrom(nBytes)
	if b2.IsEmpty() {
		b2.Release()
	} else {
		r.leftover = b2
	}

	return b, nil
}
