package message

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
)

// MaxDecoded caps decompressed bodies: a small gzip bomb from a hostile
// server must not fill the memory.
const MaxDecoded = 64 << 20

var ErrTooBig = fmt.Errorf("decoded body over %d MB", MaxDecoded>>20)

// Decode decompresses a body by its Content-Encoding (gzip, br, deflate,
// zstd), up to MaxDecoded.
func Decode(enc string, body []byte) ([]byte, error) {
	if len(body) == 0 {
		return body, nil
	}
	var r io.Reader
	switch strings.ToLower(strings.TrimSpace(enc)) {
	case "", "identity":
		return body, nil
	case "gzip", "x-gzip":
		zr, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		defer zr.Close()
		r = zr
	case "br":
		r = brotli.NewReader(bytes.NewReader(body))
	case "deflate":
		fr := flate.NewReader(bytes.NewReader(body))
		defer fr.Close()
		r = fr
	case "zstd":
		zr, err := zstd.NewReader(bytes.NewReader(body), zstd.WithDecoderMaxMemory(MaxDecoded))
		if err != nil {
			return nil, err
		}
		defer zr.Close()
		r = zr
	default:
		return nil, errors.New("unsupported Content-Encoding " + enc)
	}
	out, err := io.ReadAll(io.LimitReader(r, MaxDecoded+1))
	if err != nil {
		return nil, err
	}
	if len(out) > MaxDecoded {
		return nil, ErrTooBig
	}
	return out, nil
}
