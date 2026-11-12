package findings

import (
	"net/http"
	"strings"
	"testing"

	"github.com/anotherhadi/spilltea/internal/message"
)

func TestSecrets(t *testing.T) {
	s, err := NewSecrets()
	if err != nil {
		t.Fatal(err)
	}
	token := "xox" + "b-1234567890123-1234567890123-abcdefghijABCDEFGHIJklmn"
	body := []byte("// app\nconst cfg={slack:\"" + token + "\",x:1};")
	hits := s.Detect(nil, &message.Response{Status: 200, Body: body})
	if len(hits) == 0 {
		t.Fatal("no hit")
	}
	if !strings.HasPrefix(hits[0].Key, hits[0].Title+":") || !strings.Contains(hits[0].Detail, token+"\nline 2: ") {
		t.Fatalf("%+v", hits[0])
	}
	if again := s.Detect(nil, &message.Response{Body: body}); len(again) != len(hits) || again[0].Key != hits[0].Key {
		t.Fatalf("cached: %+v", again)
	}
	if hits := s.Detect(nil, &message.Response{Body: []byte("nothing to see")}); len(hits) != 0 {
		t.Fatalf("%+v", hits)
	}
	for name, resp := range map[string]*message.Response{
		"image": {Header: http.Header{"Content-Type": {"image/svg+xml"}}, Body: body},
		"font":  {Header: http.Header{"Content-Type": {"Font/woff2; x=1"}}, Body: body},
		"NUL":   {Header: http.Header{"Content-Type": {"application/octet-stream"}}, Body: append([]byte("PK\x03\x04\x00"), body...)},
	} {
		if hits := s.Detect(nil, resp); len(hits) != 0 {
			t.Errorf("%s is scanned: %+v", name, hits)
		}
	}
	late := append(append([]byte(nil), body...), strings.Repeat(" ", 8000)+"\x00"...)
	for name, resp := range map[string]*message.Response{
		"octet-stream": {Header: http.Header{"Content-Type": {"application/octet-stream"}}, Body: append([]byte("//\n"), body...)},
		"late NUL":     {Body: late},
	} {
		if hits := s.Detect(nil, resp); len(hits) == 0 {
			t.Errorf("%s is not scanned", name)
		}
	}
}
