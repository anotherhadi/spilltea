package sitemap

import (
	"testing"

	"github.com/anotherhadi/spilltea/internal/project"
)

func TestBuild(t *testing.T) {
	es := []project.Entry{
		{URL: "https://api.test/v1/users?x=1"},
		{URL: "https://api.test/v1/users/12"},
		{URL: "https://api.test/"},
		{URL: "http://b.test:8080/login"},
	}
	got := Build(es)
	want := []Node{
		{"api.test", "api.test", true, 3},
		{"api.test/v1∕users", "api.test/v1/users", true, 2},
		{"api.test/v1∕users/12", "api.test/v1/users/12", false, 1},
		{"b.test:8080", "b.test:8080", true, 1},
		{"b.test:8080/login", "b.test:8080/login", false, 1},
	}
	if len(got) != len(want) {
		t.Fatalf("%+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%d: %+v, want %+v", i, got[i], want[i])
		}
	}
	if !Under(Path(es[1].URL), "api.test/v1") || Under("api.test/v10", "api.test/v1") {
		t.Fatal("Under")
	}
}
