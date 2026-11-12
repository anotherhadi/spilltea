// Package sitemap shows the history as a tree: hosts, then path segments.
package sitemap

import (
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/anotherhadi/foam/draw"
	"github.com/anotherhadi/foam/theme"
	"github.com/anotherhadi/foam/tree"

	"github.com/anotherhadi/spilltea/internal/project"
)

// Node.Prefix is the real path, "api.test/v1/users"; Name is its place in
// the tree, where a chain of single-child folders is merged:
// "api.test/v1∕users".
type Node struct {
	Name   string
	Prefix string
	Dir    bool
	Count  int // entries at this node or below
}

// joined stands for the "/" inside a merged node.
const joined = "∕"

// Path keeps an encoded "/" inside a segment encoded.
func Path(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "?"
	}
	parts := []string{u.Host}
	for _, s := range strings.Split(strings.Trim(u.EscapedPath(), "/"), "/") {
		if s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, "/")
}

func Under(p, name string) bool {
	return p == name || strings.HasPrefix(p, name+"/")
}

// Build: a node with children is a folder; a folder holding a single folder
// is merged with it.
func Build(es []project.Entry) []Node {
	counts := map[string]int{}
	children := map[string][]string{}
	var roots []string
	for _, e := range es {
		p := Path(e.URL)
		for {
			counts[p]++
			parent := tree.Parent(p)
			if counts[p] == 1 {
				if parent == "" {
					roots = append(roots, p)
				} else {
					children[parent] = append(children[parent], p)
				}
			}
			if parent == "" {
				break
			}
			p = parent
		}
	}
	var out []Node
	var walk func(p, name string)
	walk = func(p, name string) {
		for {
			kids := children[p]
			if strings.Contains(name, "/") && len(kids) == 1 && len(children[kids[0]]) > 0 && counts[kids[0]] == counts[p] {
				p = kids[0]
				name += joined + base(p)
				continue
			}
			break
		}
		out = append(out, Node{Name: name, Prefix: p, Dir: len(children[p]) > 0, Count: counts[p]})
		for _, k := range children[p] {
			walk(k, name+"/"+base(k))
		}
	}
	for _, r := range roots {
		walk(r, r)
	}
	slices.SortFunc(out, compare)
	return out
}

// compare sorts segment by segment, folders first.
func compare(a, b Node) int {
	as, bs := strings.Split(a.Name, "/"), strings.Split(b.Name, "/")
	for i := 0; i < len(as) && i < len(bs); i++ {
		if as[i] == bs[i] {
			continue
		}
		aDir := i < len(as)-1 || a.Dir
		bDir := i < len(bs)-1 || b.Dir
		if aDir != bDir {
			if aDir {
				return -1
			}
			return 1
		}
		return strings.Compare(as[i], bs[i])
	}
	return len(as) - len(bs)
}

func base(name string) string {
	if i := strings.LastIndexByte(name, '/'); i >= 0 {
		return name[i+1:]
	}
	return name
}

func count(n int) string { return theme.Subtle().Render(" " + strconv.Itoa(n)) }

func New() *tree.Model[Node] {
	t := tree.New(func(n Node) (string, bool) { return n.Name, n.Dir })
	t.SetPlaceholder("filter paths")
	t.Empty = "nothing in the history yet"
	t.Folder = func(n Node) string {
		label := theme.Accent().Render(draw.Clean(strings.ReplaceAll(base(n.Name), joined, "/")))
		if !strings.Contains(n.Name, "/") {
			label = theme.Primary().Style().Bold(true).Render(draw.Clean(n.Name))
		}
		return label + count(n.Count)
	}
	t.Leaf = func(n Node, current bool) string {
		return theme.Fg().Render(draw.Clean(base(n.Name))) + count(n.Count)
	}
	return t
}
