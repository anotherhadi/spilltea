package copyas

import (
	"fmt"
	"strings"
)

type header struct{ key, value string }

type parsedRequest struct {
	method  string
	path    string
	host    string
	scheme  string
	headers []header
	body    string
}

func parseRaw(raw, scheme string) parsedRequest {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	pr := parsedRequest{scheme: scheme}
	if len(lines) == 0 {
		return pr
	}

	parts := strings.SplitN(lines[0], " ", 3)
	if len(parts) >= 1 {
		pr.method = strings.TrimSpace(parts[0])
	}
	if len(parts) >= 2 {
		pr.path = strings.TrimSpace(parts[1])
	}

	i := 1
	for i < len(lines) {
		line := strings.TrimRight(lines[i], "\r")
		if line == "" {
			i++
			break
		}
		if kv := strings.SplitN(line, ": ", 2); len(kv) == 2 {
			k := strings.TrimSpace(kv[0])
			v := strings.TrimSpace(kv[1])
			pr.headers = append(pr.headers, header{k, v})
			if strings.EqualFold(k, "host") {
				pr.host = v
			}
		}
		i++
	}

	if i < len(lines) {
		pr.body = strings.TrimRight(strings.Join(lines[i:], "\n"), "\n")
	}
	return pr
}

func (pr parsedRequest) fullURL() string {
	scheme := pr.scheme
	if scheme == "" {
		scheme = "https"
	}
	return scheme + "://" + pr.host + pr.path
}

func formatAs(id, raw, scheme string) string {
	pr := parseRaw(raw, scheme)
	switch id {
	case "raw":
		return raw
	case "curl":
		return toCurl(pr)
	case "python":
		return toPython(pr)
	case "go":
		return toGo(pr)
	case "ffuf":
		return toFFUF(pr)
	case "markdown":
		return toMarkdown(pr)
	}
	return raw
}

func toMarkdown(pr parsedRequest) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "### %s %s\n\n", pr.method, pr.fullURL())
	sb.WriteString("```\n")
	sb.WriteString(pr.method + " " + pr.path + " HTTP/1.1\n")
	for _, h := range pr.headers {
		sb.WriteString(fmt.Sprintf("%s: %s\n", h.key, h.value))
	}
	if pr.body != "" {
		sb.WriteString("\n" + pr.body)
	}
	sb.WriteString("\n```")
	return sb.String()
}

func toCurl(pr parsedRequest) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "curl -X %s '%s'", pr.method, pr.fullURL())
	for _, h := range pr.headers {
		if strings.EqualFold(h.key, "content-length") {
			continue
		}
		fmt.Fprintf(&sb, " \\\n  -H '%s: %s'", h.key, h.value)
	}
	if pr.body != "" {
		body := strings.ReplaceAll(pr.body, "'", "'\\''")
		fmt.Fprintf(&sb, " \\\n  --data '%s'", body)
	}
	return sb.String()
}

func toPython(pr parsedRequest) string {
	var sb strings.Builder
	sb.WriteString("import requests\n\n")
	fmt.Fprintf(&sb, "url = %q\n", pr.fullURL())

	sb.WriteString("headers = {\n")
	for _, h := range pr.headers {
		if strings.EqualFold(h.key, "content-length") {
			continue
		}
		fmt.Fprintf(&sb, "    %q: %q,\n", h.key, h.value)
	}
	sb.WriteString("}\n")

	method := strings.ToLower(pr.method)
	if pr.body != "" {
		fmt.Fprintf(&sb, "data = %q\n\n", pr.body)
		fmt.Fprintf(&sb, "response = requests.%s(url, headers=headers, data=data)\n", method)
	} else {
		fmt.Fprintf(&sb, "\nresponse = requests.%s(url, headers=headers)\n", method)
	}
	sb.WriteString("print(response.status_code)\n")
	sb.WriteString("print(response.text)\n")
	return sb.String()
}

func toGo(pr parsedRequest) string {
	var sb strings.Builder
	sb.WriteString("package main\n\nimport (\n")
	if pr.body != "" {
		sb.WriteString("\t\"fmt\"\n\t\"io\"\n\t\"net/http\"\n\t\"strings\"\n)\n\n")
	} else {
		sb.WriteString("\t\"fmt\"\n\t\"io\"\n\t\"net/http\"\n)\n\n")
	}
	sb.WriteString("func main() {\n")

	if pr.body != "" {
		fmt.Fprintf(&sb, "\tbody := strings.NewReader(%q)\n", pr.body)
		fmt.Fprintf(&sb, "\treq, err := http.NewRequest(%q, %q, body)\n", pr.method, pr.fullURL())
	} else {
		fmt.Fprintf(&sb, "\treq, err := http.NewRequest(%q, %q, nil)\n", pr.method, pr.fullURL())
	}
	sb.WriteString("\tif err != nil { panic(err) }\n")

	for _, h := range pr.headers {
		if strings.EqualFold(h.key, "host") || strings.EqualFold(h.key, "content-length") {
			continue
		}
		fmt.Fprintf(&sb, "\treq.Header.Set(%q, %q)\n", h.key, h.value)
	}

	sb.WriteString("\n\tclient := &http.Client{}\n")
	sb.WriteString("\tresp, err := client.Do(req)\n")
	sb.WriteString("\tif err != nil { panic(err) }\n")
	sb.WriteString("\tdefer resp.Body.Close()\n")
	sb.WriteString("\tbody2, _ := io.ReadAll(resp.Body)\n")
	sb.WriteString("\tfmt.Printf(\"Status: %d\\n\", resp.StatusCode)\n")
	sb.WriteString("\tfmt.Println(string(body2))\n")
	sb.WriteString("}\n")
	return sb.String()
}

func toFFUF(pr parsedRequest) string {
	// Place FUZZ in the path: replace query string or append ?FUZZ
	fuzzURL := pr.scheme + "://" + pr.host
	if idx := strings.Index(pr.path, "?"); idx != -1 {
		fuzzURL += pr.path[:idx] + "?FUZZ"
	} else {
		fuzzURL += pr.path + "?FUZZ"
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "ffuf -u '%s'", fuzzURL)
	sb.WriteString(" \\\n  -w wordlist.txt")
	fmt.Fprintf(&sb, " \\\n  -X %s", pr.method)
	for _, h := range pr.headers {
		if strings.EqualFold(h.key, "content-length") {
			continue
		}
		fmt.Fprintf(&sb, " \\\n  -H '%s: %s'", h.key, h.value)
	}
	if pr.body != "" {
		body := strings.ReplaceAll(pr.body, "'", "'\\''")
		fmt.Fprintf(&sb, " \\\n  -d '%s'", body)
	}
	return sb.String()
}
