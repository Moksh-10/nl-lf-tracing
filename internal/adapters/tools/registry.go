package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type Registry struct {
	Root   string
	Tavily string
	Todos  []map[string]any
}

func (r *Registry) Dispatch(ctx context.Context, name string, args map[string]any) (string, error) {
	switch name {
	case "Read":
		return r.read(args)
	case "Write":
		return r.write(args)
	case "Edit":
		return r.edit(args)
	case "Glob":
		return r.glob(args)
	case "Grep":
		return r.grep(args)
	case "Bash":
		return r.bash(ctx, args)
	case "WebSearch":
		return r.webSearch(ctx, args)
	case "WebFetch":
		return r.webFetch(ctx, args)
	case "TodoWrite":
		return r.todoWrite(args)
	default:
		return "", fmt.Errorf("unknown tool %q", name)
	}
}

func (r *Registry) resolve(path string) (string, error) {
	p := path
	if !filepath.IsAbs(p) {
		p = filepath.Join(r.Root, p)
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	root, err := filepath.Abs(r.Root)
	if err != nil {
		return "", err
	}
	if abs != root && !strings.HasPrefix(abs, root+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes workspace")
	}
	return abs, nil
}

func (r *Registry) read(args map[string]any) (string, error) {
	path, _ := args["path"].(string)
	offset := intArg(args, "offset", 1)
	limit := intArg(args, "limit", 0)
	target, err := r.resolve(path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(data), "\n")
	start := max(1, offset) - 1
	end := len(lines)
	if limit > 0 && start+limit < end {
		end = start + limit
	}
	var b strings.Builder
	for i := start; i < end && i < len(lines); i++ {
		fmt.Fprintf(&b, "%d|%s\n", i+1, lines[i])
	}
	return b.String(), nil
}

func (r *Registry) write(args map[string]any) (string, error) {
	path, _ := args["path"].(string)
	contents, _ := args["contents"].(string)
	target, err := r.resolve(path)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(target, []byte(contents), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("Wrote %d bytes", len(contents)), nil
}

func (r *Registry) edit(args map[string]any) (string, error) {
	path, _ := args["path"].(string)
	oldS, _ := args["old_string"].(string)
	newS, _ := args["new_string"].(string)
	replaceAll, _ := args["replace_all"].(bool)
	target, err := r.resolve(path)
	if err != nil {
		return "", err
	}
	text, err := os.ReadFile(target)
	if err != nil {
		return "", err
	}
	s := string(text)
	if !strings.Contains(s, oldS) {
		return "", fmt.Errorf("old_string not found")
	}
	count := 1
	if replaceAll {
		count = strings.Count(s, oldS)
		s = strings.ReplaceAll(s, oldS, newS)
	} else {
		s = strings.Replace(s, oldS, newS, 1)
	}
	if err := os.WriteFile(target, []byte(s), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("Replaced %d occurrence(s)", count), nil
}

func (r *Registry) glob(args map[string]any) (string, error) {
	pattern, _ := args["glob_pattern"].(string)
	dir, _ := args["target_directory"].(string)
	if dir == "" {
		dir = "."
	}
	base, err := r.resolve(dir)
	if err != nil {
		return "", err
	}
	matches, err := filepath.Glob(filepath.Join(base, pattern))
	if err != nil {
		return "", err
	}
	root, _ := filepath.Abs(r.Root)
	var rel []string
	for _, m := range matches {
		if st, err := os.Stat(m); err == nil && !st.IsDir() {
			rel = append(rel, strings.TrimPrefix(m, root+string(os.PathSeparator)))
		}
	}
	return strings.Join(rel, "\n"), nil
}

func (r *Registry) grep(args map[string]any) (string, error) {
	pattern, _ := args["pattern"].(string)
	path, _ := args["path"].(string)
	globPat, _ := args["glob"].(string)
	limit := intArg(args, "head_limit", 50)
	rx, err := regexp.Compile(pattern)
	if err != nil {
		return "", err
	}
	rootPath, err := r.resolve(pathValue(path))
	if err != nil {
		return "", err
	}
	root, _ := filepath.Abs(r.Root)
	var hits []string
	walk := func(fp string) error {
		st, err := os.Stat(fp)
		if err != nil || st.IsDir() {
			return nil
		}
		data, err := os.ReadFile(fp)
		if err != nil {
			return nil
		}
		for i, line := range strings.Split(string(data), "\n") {
			if rx.MatchString(line) {
				rel, _ := filepath.Rel(root, fp)
				hits = append(hits, fmt.Sprintf("%s:%d:%s", rel, i+1, line))
				if len(hits) >= limit {
					return io.EOF
				}
			}
		}
		return nil
	}
	if st, err := os.Stat(rootPath); err == nil && !st.IsDir() {
		_ = walk(rootPath)
	} else {
		_ = filepath.WalkDir(rootPath, func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if globPat != "" {
				ok, _ := filepath.Match(globPat, filepath.Base(p))
				if !ok {
					return nil
				}
			}
			if err := walk(p); err == io.EOF {
				return io.EOF
			}
			return nil
		})
	}
	if len(hits) == 0 {
		return "(no matches)", nil
	}
	return strings.Join(hits, "\n"), nil
}

func (r *Registry) bash(ctx context.Context, args map[string]any) (string, error) {
	cmdStr, _ := args["command"].(string)
	timeout := time.Duration(intArg(args, "timeout_seconds", 120)) * time.Second
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, "bash", "-lc", cmdStr)
	cmd.Dir = r.Root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	payload := map[string]any{
		"exit_code": 0,
		"stdout":    trimTail(stdout.String(), 8000),
		"stderr":    trimTail(stderr.String(), 4000),
	}
	if err != nil {
		payload["exit_code"] = 1
		payload["error"] = err.Error()
	}
	b, _ := json.MarshalIndent(payload, "", "  ")
	return string(b), nil
}

func (r *Registry) webSearch(ctx context.Context, args map[string]any) (string, error) {
	if r.Tavily == "" {
		return "", fmt.Errorf("TAVILY_API_KEY not set")
	}
	query, _ := args["query"].(string)
	maxResults := intArg(args, "max_results", 5)
	body, _ := json.Marshal(map[string]any{"api_key": r.Tavily, "query": query, "max_results": maxResults})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.tavily.com/search", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return trimTail(string(data), 12000), nil
}

func (r *Registry) webFetch(ctx context.Context, args map[string]any) (string, error) {
	if r.Tavily == "" {
		return "", fmt.Errorf("TAVILY_API_KEY not set")
	}
	url, _ := args["url"].(string)
	body, _ := json.Marshal(map[string]any{"api_key": r.Tavily, "urls": []string{url}})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.tavily.com/extract", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return trimTail(string(data), 12000), nil
}

func (r *Registry) todoWrite(args map[string]any) (string, error) {
	raw, ok := args["todos"].([]any)
	if !ok {
		return "", fmt.Errorf("todos must be an array")
	}
	r.Todos = nil
	for _, item := range raw {
		if m, ok := item.(map[string]any); ok {
			r.Todos = append(r.Todos, m)
		}
	}
	b, _ := json.MarshalIndent(map[string]any{"saved": len(r.Todos), "todos": r.Todos}, "", "  ")
	return string(b), nil
}

func intArg(args map[string]any, key string, def int) int {
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	default:
		return def
	}
}

func pathValue(p string) string {
	if p == "" {
		return "."
	}
	return p
}

func trimTail(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[len(s)-max:]
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
