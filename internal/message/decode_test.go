package message

import (
	"bytes"
	"compress/gzip"
	"errors"
	"testing"
)

func gz(b []byte) []byte {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	w.Write(b)
	w.Close()
	return buf.Bytes()
}

func TestDecode(t *testing.T) {
	out, err := Decode("gzip", gz([]byte("hello")))
	if err != nil || string(out) != "hello" {
		t.Fatalf("got %q, %v", out, err)
	}
	if out, err := Decode("identity", []byte("x")); err != nil || string(out) != "x" {
		t.Fatalf("identity: %q, %v", out, err)
	}
	if _, err := Decode("compress", []byte("x")); err == nil {
		t.Fatal("unknown encoding accepted")
	}
}

// A gzip bomb is rejected instead of being fully decompressed.
func TestDecodeBomb(t *testing.T) {
	bomb := gz(make([]byte, MaxDecoded+1))
	if _, err := Decode("gzip", bomb); !errors.Is(err, ErrTooBig) {
		t.Fatalf("err = %v, want ErrTooBig", err)
	}
}
