package scope

import (
	"net/url"
	"testing"
)

func TestURL(t *testing.T) {
	s, err := New([]string{"*.example.com", "api.test:8443", "shop.test/api", "cdn.test/*/img"}, []string{"logout.example.com", "shop.test/api/health"})
	if err != nil {
		t.Fatal(err)
	}
	for u, want := range map[string]bool{
		"https://a.example.com/x":         true,
		"https://A.B.Example.COM/":        true,
		"https://example.com/":            false,
		"https://logout.example.com/":     false,
		"https://api.test:8443/":          true,
		"https://api.test/":               false,
		"http://shop.test/api/users":      true,
		"http://shop.test/apiv2":          true,
		"http://shop.test/other":          false,
		"http://shop.test/api/health?x=1": false,
		"https://cdn.test/v1/img/a.png":   true,
		"https://cdn.test/v1/css/a.css":   false,
	} {
		pu, _ := url.Parse(u)
		if got := s.URL(pu); got != want {
			t.Errorf("%s: %v, want %v", u, got, want)
		}
	}
	if !s.Host("shop.test", 443) || s.Host("logout.example.com", 443) || s.Host("other.test", 443) {
		t.Error("Host")
	}
}

func TestEmpty(t *testing.T) {
	s, _ := New(nil, []string{"ads.test"})
	pu, _ := url.Parse("https://anything.test/")
	if !s.URL(pu) || s.Host("ads.test", 443) {
		t.Fatal("an empty include takes everything but the excluded")
	}
	for _, bad := range []string{"/path", "h:99999", "h:x", "a.test/\xa9*"} {
		if _, err := New([]string{bad}, nil); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}
