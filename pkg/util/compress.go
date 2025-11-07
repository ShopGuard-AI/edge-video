package util

import (
	"bytes"
	"fmt"

	zstd "github.com/klauspost/compress/zstd"
)

type Compressor struct {
	encoder *zstd.Encoder
	level   int
}

func NewCompressor(level int) (*Compressor, error) {
	enc, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(level)))
	if err != nil {
		return nil, fmt.Errorf("zstd new writer: %w", err)
	}
	return &Compressor{encoder: enc, level: level}, nil
}

func (c *Compressor) Compress(data []byte) ([]byte, error) {
	var b bytes.Buffer
	w, err := zstd.NewWriter(&b)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(data); err != nil {
		w.Close()
		return nil, err
	}
	w.Close()
	return b.Bytes(), nil
}

func Decompress(data []byte) ([]byte, error) {
	r, err := zstd.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	var out bytes.Buffer
	_, err = out.ReadFrom(r)
	if err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
