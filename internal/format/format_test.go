package format

import "testing"

func TestBody(t *testing.T) {
	for _, c := range []struct {
		body, ct, want string
		ok             bool
	}{
		{`{"a":[1,2]}`, "", "{\n  \"a\": [\n    1,\n    2\n  ]\n}", true},
		{`{"a":`, "application/json", `{"a":`, false},
		{"plain text", "text/plain", "plain text", false},
		{"<p>x</p>", "application/javascript", "<p>x</p>", false},
		{
			`<!DOCTYPE html><html><head><title>Hi</title><meta charset="utf-8"></head>` +
				`<body><div class="a"><p>one</p><br><span></span><pre>  keep
  this</pre><!-- c --></div></body></html>`,
			"",
			`<!DOCTYPE html>
<html>
  <head>
    <title>Hi</title>
    <meta charset="utf-8">
  </head>
  <body>
    <div class="a">
      <p>one</p>
      <br>
      <span></span>
      <pre>  keep
  this</pre>
      <!-- c -->
    </div>
  </body>
</html>`,
			true,
		},
		{"<div>a<b>b</b>\n  c</div>", "text/html; charset=utf-8", "<div>\n  a\n  <b>b</b>\n  c\n</div>", true},
	} {
		got, ok := Body([]byte(c.body), c.ct)
		if string(got) != c.want || ok != c.ok {
			t.Errorf("%q: got %v\n%s", c.body, ok, got)
		}
	}
}
