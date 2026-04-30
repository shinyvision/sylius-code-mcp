package syliustwighooks

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/sylius-code-mcp/internal/yamlutil"
	"gopkg.in/yaml.v3"
)

type HookableSpec struct {
	Name     string
	Template string
	Priority *int
	Enabled  *bool
}

type AddHookParams struct {
	CategoryName string
	HookName     string
	Hookable     HookableSpec
}

type AddHookResult struct {
	FilePath        string
	FileCreated     bool
	HookCreated     bool
	HookableCreated bool
	HookableUpdated bool
	Message         string
}

var (
	categoryNameRe = regexp.MustCompile(`^[a-z0-9_]+$`)
	hookNameRe     = regexp.MustCompile(`^[A-Za-z0-9_]+(\.[A-Za-z0-9_#]+)*$`)
	hookableNameRe = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
)

func AddHook(projectRoot string, p AddHookParams) (AddHookResult, error) {
	if err := validateAddHookParams(p); err != nil {
		return AddHookResult{}, err
	}

	relPath := filepath.ToSlash(filepath.Join("config/packages/twig_hooks", p.CategoryName+".yaml"))
	absPath := filepath.Join(projectRoot, relPath)

	docNode, fileCreated, err := loadOrInitDoc(absPath)
	if err != nil {
		return AddHookResult{}, err
	}

	rootMap := docNode.Content[0]
	twigHooksNode := getOrCreateMapping(rootMap, "sylius_twig_hooks")
	if twigHooksNode == nil {
		return AddHookResult{}, fmt.Errorf("sylius_twig_hooks is not a mapping in %s", relPath)
	}
	hooksNode := getOrCreateMapping(twigHooksNode, "hooks")
	if hooksNode == nil {
		return AddHookResult{}, fmt.Errorf("sylius_twig_hooks.hooks is not a mapping in %s", relPath)
	}

	hookValueNode, hookCreated := findOrCreateHookEntry(hooksNode, p.HookName)
	if hookValueNode == nil {
		return AddHookResult{}, fmt.Errorf("hook %q is not a mapping", p.HookName)
	}

	hookableSpecNode := buildHookableNode(p.Hookable)

	hookableCreated := true
	hookableUpdated := false
	for i := 0; i+1 < len(hookValueNode.Content); i += 2 {
		if hookValueNode.Content[i].Value == p.Hookable.Name {
			hookValueNode.Content[i+1] = hookableSpecNode
			hookableCreated = false
			hookableUpdated = true
			break
		}
	}
	if hookableCreated {
		yamlutil.AddField(hookValueNode, yamlutil.Scalar(p.Hookable.Name), hookableSpecNode)
	}

	sortMappingByKey(hooksNode)
	normalizeHookKeys(hooksNode)

	if err := writeYAMLFile(absPath, docNode); err != nil {
		return AddHookResult{}, err
	}

	return AddHookResult{
		FilePath:        relPath,
		FileCreated:     fileCreated,
		HookCreated:     hookCreated,
		HookableCreated: hookableCreated,
		HookableUpdated: hookableUpdated,
		Message:         buildAddHookMessage(p, relPath, fileCreated, hookCreated, hookableUpdated),
	}, nil
}

func validateAddHookParams(p AddHookParams) error {
	if p.CategoryName == "" || !categoryNameRe.MatchString(p.CategoryName) {
		return fmt.Errorf("categoryName must be a non-empty snake_case identifier (got %q)", p.CategoryName)
	}
	if p.HookName == "" || !hookNameRe.MatchString(p.HookName) {
		return fmt.Errorf("hookName must be a dot-separated identifier (got %q)", p.HookName)
	}
	if p.Hookable.Name == "" || !hookableNameRe.MatchString(p.Hookable.Name) {
		return fmt.Errorf("hookable.name must be a non-empty identifier (got %q)", p.Hookable.Name)
	}
	if strings.TrimSpace(p.Hookable.Template) == "" {
		return errors.New("hookable.template is required")
	}
	return nil
}

func loadOrInitDoc(absPath string) (*yaml.Node, bool, error) {
	data, err := os.ReadFile(absPath)
	switch {
	case err == nil:
		doc := &yaml.Node{}
		if err := yaml.Unmarshal(data, doc); err != nil {
			return nil, false, fmt.Errorf("parsing %s: %w", absPath, err)
		}
		if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
			doc = newEmptyDoc()
		} else if doc.Content[0].Kind != yaml.MappingNode {
			return nil, false, fmt.Errorf("root of %s is not a mapping", absPath)
		}
		return doc, false, nil
	case errors.Is(err, fs.ErrNotExist):
		return newEmptyDoc(), true, nil
	default:
		return nil, false, fmt.Errorf("reading %s: %w", absPath, err)
	}
}

func newEmptyDoc() *yaml.Node {
	return &yaml.Node{
		Kind:    yaml.DocumentNode,
		Content: []*yaml.Node{yamlutil.MappingNode()},
	}
}

func getOrCreateMapping(parent *yaml.Node, key string) *yaml.Node {
	if parent == nil || parent.Kind != yaml.MappingNode {
		return nil
	}
	if existing := yamlutil.FindInMapping(parent, key); existing != nil {
		if existing.Kind != yaml.MappingNode {
			return nil
		}
		return existing
	}
	created := yamlutil.MappingNode()
	yamlutil.AddField(parent, yamlutil.Scalar(key), created)
	return created
}

func findOrCreateHookEntry(hooksNode *yaml.Node, hookName string) (*yaml.Node, bool) {
	if existing := yamlutil.FindInMapping(hooksNode, hookName); existing != nil {
		if existing.Kind != yaml.MappingNode {
			return nil, false
		}
		return existing, false
	}
	value := yamlutil.MappingNode()
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Style: yaml.SingleQuotedStyle, Value: hookName}
	yamlutil.AddField(hooksNode, keyNode, value)
	return value, true
}

func buildHookableNode(h HookableSpec) *yaml.Node {
	node := yamlutil.MappingNode()
	yamlutil.AddField(node, yamlutil.Scalar("template"), yamlutil.SingleQuotedScalar(h.Template))
	if h.Priority != nil {
		yamlutil.AddField(node, yamlutil.Scalar("priority"), yamlutil.Scalar(strconv.Itoa(*h.Priority)))
	}
	if h.Enabled != nil {
		yamlutil.AddField(node, yamlutil.Scalar("enabled"), yamlutil.BoolNode(*h.Enabled))
	}
	return node
}

func sortMappingByKey(m *yaml.Node) {
	if m == nil || m.Kind != yaml.MappingNode {
		return
	}
	type pair struct{ k, v *yaml.Node }
	pairs := make([]pair, 0, len(m.Content)/2)
	for i := 0; i+1 < len(m.Content); i += 2 {
		pairs = append(pairs, pair{m.Content[i], m.Content[i+1]})
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		return pairs[i].k.Value < pairs[j].k.Value
	})
	m.Content = m.Content[:0]
	for _, p := range pairs {
		m.Content = append(m.Content, p.k, p.v)
	}
}

func normalizeHookKeys(hooksNode *yaml.Node) {
	for i := 0; i+1 < len(hooksNode.Content); i += 2 {
		k := hooksNode.Content[i]
		k.Style = yaml.SingleQuotedStyle
		k.Tag = ""
	}
}

func writeYAMLFile(absPath string, docNode *yaml.Node) error {
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(4)
	if err := enc.Encode(docNode); err != nil {
		return fmt.Errorf("marshalling yaml: %w", err)
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("closing yaml encoder: %w", err)
	}
	if err := os.WriteFile(absPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", absPath, err)
	}
	return nil
}

func buildAddHookMessage(p AddHookParams, relPath string, fileCreated, hookCreated, hookableUpdated bool) string {
	var sb strings.Builder
	switch {
	case fileCreated:
		fmt.Fprintf(&sb, "Created `%s`.\n", relPath)
	case hookCreated:
		fmt.Fprintf(&sb, "Added new hook `%s` in `%s`.\n", p.HookName, relPath)
	case hookableUpdated:
		fmt.Fprintf(&sb, "Replaced existing hookable `%s` under `%s` in `%s`.\n", p.Hookable.Name, p.HookName, relPath)
	default:
		fmt.Fprintf(&sb, "Added hookable `%s` under `%s` in `%s`.\n", p.Hookable.Name, p.HookName, relPath)
	}
	sb.WriteString("Hooks in this file are kept sorted lexicographically so similar hook names group together.\n")
	return sb.String()
}
