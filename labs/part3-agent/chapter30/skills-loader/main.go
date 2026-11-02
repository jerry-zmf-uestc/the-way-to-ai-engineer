package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"io/fs"
	"os"
	"sort"
	"strings"
	"unicode/utf8"
)

var (
	ErrSkill  = errors.New("skills: invalid package")
	ErrDenied = errors.New("skills: activation or tool denied")
	ErrBudget = errors.New("skills: loading budget exceeded")
)

type Metadata struct {
	Name          string            `yaml:"name"`
	Description   string            `yaml:"description"`
	License       string            `yaml:"license"`
	Compatibility string            `yaml:"compatibility"`
	Metadata      map[string]string `yaml:"metadata"`
	AllowedTools  string            `yaml:"allowed-tools"`
}
type Entry struct{ Name, Description, Version, HeaderHash string }
type Loaded struct{ Name, Version, Text, Digest string }
type Loader struct {
	root            *os.Root
	catalog         map[string]Entry
	approved, tools map[string]bool
	loaded          map[string]Loaded
	resources       map[string]string
	resourceHashes  map[string]map[string]string
	Budget, Used    int
}

func hash(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func nameValid(name string) bool {
	if len(name) == 0 || len(name) > 64 || strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") || strings.Contains(name, "--") {
		return false
	}
	for _, c := range name {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
			return false
		}
	}
	return true
}
func header(data []byte) (Metadata, int, error) {
	if !utf8.Valid(data) || !bytes.HasPrefix(data, []byte("---\n")) {
		return Metadata{}, 0, ErrSkill
	}
	end := bytes.Index(data[4:], []byte("\n---\n"))
	if end < 0 {
		return Metadata{}, 0, ErrSkill
	}
	end += 4
	if end > 4096 {
		return Metadata{}, 0, ErrSkill
	}
	var meta Metadata
	dec := yaml.NewDecoder(bytes.NewReader(data[4:end]))
	dec.KnownFields(true)
	if err := dec.Decode(&meta); err != nil {
		return Metadata{}, 0, fmt.Errorf("%w: %w", ErrSkill, err)
	}
	// Preserve presence and scalar type; decoding into a string alone can coerce YAML numbers.
	var fields map[string]any
	if err := yaml.Unmarshal(data[4:end], &fields); err != nil {
		return Metadata{}, 0, ErrSkill
	}
	for _, name := range []string{"name", "description", "license", "compatibility", "allowed-tools"} {
		if value, present := fields[name]; present {
			if _, ok := value.(string); !ok {
				return Metadata{}, 0, ErrSkill
			}
		}
	}
	if value, present := fields["metadata"]; present {
		values, ok := value.(map[string]any)
		if !ok {
			return Metadata{}, 0, ErrSkill
		}
		for _, value := range values {
			if _, ok := value.(string); !ok {
				return Metadata{}, 0, ErrSkill
			}
		}
	}
	if field, present := fields["compatibility"]; present {
		value, ok := field.(string)
		if !ok || strings.TrimSpace(value) == "" || utf8.RuneCountInString(value) > 500 {
			return Metadata{}, 0, ErrSkill
		}
	}
	if !nameValid(meta.Name) || strings.TrimSpace(meta.Description) == "" || utf8.RuneCountInString(meta.Description) > 1024 || meta.Metadata["version"] == "" || len(meta.Metadata["version"]) > 128 {
		return Metadata{}, 0, ErrSkill
	}
	return meta, end + 5, nil
}
func New(root *os.Root, budget int, approved, tools map[string]bool) (*Loader, error) {
	if budget <= 0 || budget > 32<<10 {
		return nil, ErrBudget
	}
	loader := &Loader{root: root, Budget: budget, catalog: map[string]Entry{}, approved: map[string]bool{}, tools: map[string]bool{}, loaded: map[string]Loaded{}, resources: map[string]string{}, resourceHashes: map[string]map[string]string{}}
	for k, v := range approved {
		loader.approved[k] = v
	}
	for k, v := range tools {
		loader.tools[k] = v
	}
	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return nil, err
	}
	for _, dir := range entries {
		if !dir.IsDir() {
			continue
		}
		if !nameValid(dir.Name()) || len(loader.catalog) >= 8 {
			return nil, ErrSkill
		}
		skill, err := root.OpenRoot(dir.Name())
		if err != nil {
			return nil, err
		}
		f, err := skill.Open("SKILL.md")
		if err != nil {
			skill.Close()
			return nil, err
		}
		// 发现阶段只读取有界前部，不加载整个正文；4KiB元信息不足则拒绝。
		data, err := io.ReadAll(io.LimitReader(f, 4096))
		f.Close()
		skill.Close()
		if err != nil {
			return nil, err
		}
		meta, end, err := header(data)
		if err != nil || meta.Name != dir.Name() {
			return nil, ErrSkill
		}
		hashes := map[string]string{}
		for key, value := range meta.Metadata {
			if strings.HasPrefix(key, "sha256.") {
				path := strings.TrimPrefix(key, "sha256.")
				decoded, e := hex.DecodeString(value)
				if e != nil || len(decoded) != 32 || !fs.ValidPath(path) || !strings.HasPrefix(path, "references/") {
					return nil, ErrSkill
				}
				hashes[path] = hex.EncodeToString(decoded)
			}
		}
		loader.resourceHashes[meta.Name] = hashes
		loader.catalog[meta.Name] = Entry{Name: meta.Name, Description: meta.Description, Version: meta.Metadata["version"], HeaderHash: hash(data[:end])}
	}
	return loader, nil
}
func (l *Loader) Catalog() []Entry {
	out := make([]Entry, 0, len(l.catalog))
	for _, entry := range l.catalog {
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Load固定一次运行的快照；重复激活不重复扣预算，重新发现才切换版本。
func (l *Loader) Load(ctx context.Context, name string) (Loaded, error) {
	if err := ctx.Err(); err != nil {
		return Loaded{}, err
	}
	entry, ok := l.catalog[name]
	if !ok || !l.approved[name] {
		return Loaded{}, ErrDenied
	}
	if value, ok := l.loaded[name]; ok {
		return value, nil
	}
	skill, err := l.root.OpenRoot(name)
	if err != nil {
		return Loaded{}, err
	}
	defer skill.Close()
	f, err := skill.Open("SKILL.md")
	if err != nil {
		return Loaded{}, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (16<<10)+1))
	if err != nil {
		return Loaded{}, err
	}
	if len(data) > 16<<10 {
		return Loaded{}, ErrSkill
	}
	meta, end, err := header(data)
	if err != nil || meta.Name != entry.Name || meta.Metadata["version"] != entry.Version || hash(data[:end]) != entry.HeaderHash {
		return Loaded{}, ErrSkill
	}
	body := strings.TrimSpace(string(data[end:]))
	if body == "" {
		return Loaded{}, ErrSkill
	}
	if len(body) > l.Budget-l.Used {
		return Loaded{}, ErrBudget
	}
	if err := ctx.Err(); err != nil {
		return Loaded{}, err
	}
	value := Loaded{Name: name, Version: entry.Version, Text: body, Digest: hash([]byte(body))}
	l.loaded[name] = value
	l.Used += len(body)
	return value, nil
}

// Select仅做教学词重合匹配，不冒充语义路由；结果仍经过激活批准。
func (l *Loader) Select(query string) []string {
	out := []string{}
	words := strings.Fields(strings.ToLower(query))
	for name, entry := range l.catalog {
		if !l.approved[name] {
			continue
		}
		match := false
		for _, word := range words {
			for _, token := range strings.Fields(strings.ToLower(entry.Description)) {
				if word == strings.Trim(token, ".,;:") {
					match = true
				}
			}
		}
		if match {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}
func (l *Loader) ReadReference(ctx context.Context, name, path string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if _, ok := l.loaded[name]; !ok || !fs.ValidPath(path) || !strings.HasPrefix(path, "references/") {
		return "", ErrDenied
	}
	expected, registered := l.resourceHashes[name][path]
	if !registered {
		return "", ErrDenied
	}
	key := name + "\x00" + path
	if text, ok := l.resources[key]; ok {
		return text, nil
	}
	root, err := l.root.OpenRoot(name)
	if err != nil {
		return "", err
	}
	defer root.Close()
	f, err := root.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (8<<10)+1))
	if err != nil {
		return "", err
	}
	if len(data) > 8<<10 || !utf8.Valid(data) || hash(data) != expected {
		return "", ErrSkill
	}
	if len(data) > l.Budget-l.Used {
		return "", ErrBudget
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	text := string(data)
	l.resources[key] = text
	l.Used += len(data)
	return text, nil
}
func (l *Loader) AuthorizeTool(name string) error {
	if !l.tools[name] {
		return ErrDenied
	}
	return nil
}
func fixture(dir string) error {
	packages := map[string]struct{ description, body string }{
		"incident-diagnosis": {"Diagnose timeout failures using readonly evidence.", "1. Read the error description.\n2. Compare timeout and retry evidence.\n3. Cite sources; report missing facts.\nSee references/guide.md."},
		"report-format":      {"Format a report with source references.", "1. Group claims by topic.\n2. Add source references.\n3. State unresolved evidence."},
	}
	for name, pkg := range packages {
		if err := os.MkdirAll(dir+"/"+name+"/references", 0700); err != nil {
			return err
		}
		text := fmt.Sprintf("---\nname: %s\ndescription: %s\nmetadata:\n  version: v1\n  sha256.references/guide.md: %s\nallowed-tools: run-command\n---\n%s\n", name, pkg.description, hash([]byte("Use only approved readonly tools.\n")), pkg.body)
		if err := os.WriteFile(dir+"/"+name+"/SKILL.md", []byte(text), 0600); err != nil {
			return err
		}
		if err := os.WriteFile(dir+"/"+name+"/references/guide.md", []byte("Use only approved readonly tools.\n"), 0600); err != nil {
			return err
		}
	}
	return nil
}
func main() {
	export := flag.Bool("export-inputs", false, "export actual full and selected skill inputs for comparison")
	set := flag.String("comparison-set", "development-v1", "registered comparison task set")
	flag.Parse()
	dir, err := os.MkdirTemp("", "skills-demo-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	if err := fixture(dir); err != nil {
		panic(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		panic(err)
	}
	defer root.Close()
	if *export {
		if err := exportComparisons(context.Background(), root, os.Stdout, *set); err != nil {
			panic(err)
		}
		return
	}
	loader, err := New(root, 1024, map[string]bool{"incident-diagnosis": true}, map[string]bool{"read-doc": true})
	if err != nil {
		panic(err)
	}
	loaded, err := loader.Load(context.Background(), "incident-diagnosis")
	if err != nil {
		panic(err)
	}
	loader.Load(context.Background(), loaded.Name)
	fmt.Printf("catalog=%d loaded=%d used_bytes=%d extra_tool_denied=%t\n", len(loader.Catalog()), len(loader.loaded), loader.Used, errors.Is(loader.AuthorizeTool("run-command"), ErrDenied))
}
