package util


import (
"bytes"
"fmt"


zstdpkg "github.com/klauspost/compress/zstd"
)


// Compressor wraps a zstd encoder for reuse.
type Compressor struct {
encoder *zstdpkg.Encoder
level int
}


// NewCompressor cria um novo compressor. Level 1..22 (zstd library maps levels internally).
func NewCompressor(level int) (*Compressor, error) {
enc, err := zstdpkg.NewWriter(nil, zstdpkg.WithEncoderLevel(zstdpkg.EncoderLevelFromZstd(level)))
if err != nil {
return nil, fmt.Errorf("zstd new writer: %w", err)
}
return &Compressor{encoder: enc, level: level}, nil
}


func (c *Compressor) Compress(data []byte) ([]byte, error) {
var b bytes.Buffer
w, err := zstdpkg.NewWriter(&b)
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
	r, err := zstdpkg.NewReader(bytes.NewReader(data))
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