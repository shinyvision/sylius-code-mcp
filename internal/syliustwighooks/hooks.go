package syliustwighooks

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sylius-code-mcp/internal/container"
)

const (
	servicePrefix = "sylius_twig_hooks.hook."
	hookableSep   = ".hookable."
	MaxHooks      = 250
)

type Node struct {
	Name     string
	Children map[string]*Node
}

func newNode(name string) *Node {
	return &Node{Name: name, Children: map[string]*Node{}}
}

func GetTwigHooks(projectRoot, prefix string) (string, error) {
	cnt, err := container.Load(projectRoot)
	if err != nil {
		return "", fmt.Errorf("loading container XML: %w", err)
	}

	root := BuildHookTreeFromIDs(serviceIDs(cnt))
	return RenderSubtree(root, prefix), nil
}

func serviceIDs(cnt *container.Container) []string {
	ids := make([]string, 0, len(cnt.Services))
	for id := range cnt.Services {
		ids = append(ids, id)
	}
	return ids
}

func BuildHookTreeFromIDs(ids []string) *Node {
	root := newNode("")
	for _, id := range ids {
		if !strings.HasPrefix(id, servicePrefix) {
			continue
		}
		rest := id[len(servicePrefix):]
		idx := strings.Index(rest, hookableSep)
		if idx < 0 {
			continue
		}
		parent := rest[:idx]
		leaf := rest[idx+len(hookableSep):]
		if parent == "" || leaf == "" {
			continue
		}
		parentNode := ensureNode(root, parent)
		if _, ok := parentNode.Children[leaf]; !ok {
			parentNode.Children[leaf] = newNode(leaf)
		}
	}
	return root
}

func ensureNode(root *Node, path string) *Node {
	node := root
	if path == "" {
		return node
	}
	for _, seg := range strings.Split(path, ".") {
		child, ok := node.Children[seg]
		if !ok {
			child = newNode(seg)
			node.Children[seg] = child
		}
		node = child
	}
	return node
}

func FindNode(root *Node, prefix string) *Node {
	if prefix == "" {
		return root
	}
	node := root
	for _, seg := range strings.Split(prefix, ".") {
		child, ok := node.Children[seg]
		if !ok {
			return nil
		}
		node = child
	}
	return node
}

func RenderSubtree(root *Node, prefix string) string {
	node := FindNode(root, prefix)
	if node == nil {
		return renderNotFound(root, prefix)
	}

	count := countNodes(node)
	if count > MaxHooks {
		return renderTooMany(node, prefix, count)
	}

	if len(node.Children) == 0 {
		if prefix == "" {
			return "No Sylius Twig hooks found in the compiled container.\n"
		}
		return fmt.Sprintf("Hook `%s` exists but has no hookable children.\n", prefix)
	}

	var sb strings.Builder
	if prefix == "" {
		sb.WriteString("# Sylius Twig hooks\n\n")
	} else {
		sb.WriteString(fmt.Sprintf("# Sylius Twig hooks under `%s`\n\n", prefix))
	}
	writeTree(&sb, node, "")
	return sb.String()
}

func renderNotFound(root *Node, prefix string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("No hooks found for prefix `%s`.\n\n", prefix))
	sb.WriteString("Top-level hook namespaces:\n")
	for _, name := range childrenSorted(root) {
		sb.WriteString("- " + name + "\n")
	}
	return sb.String()
}

func renderTooMany(node *Node, prefix string, count int) string {
	var sb strings.Builder
	if prefix == "" {
		sb.WriteString(fmt.Sprintf("Too many hooks to display (%d > %d). Supply a more specific `prefix`.\n\n", count, MaxHooks))
	} else {
		sb.WriteString(fmt.Sprintf("Too many hooks under `%s` (%d > %d). Supply a more specific `prefix`.\n\n", prefix, count, MaxHooks))
	}
	sb.WriteString("Direct children you can drill into:\n")
	names := childrenSorted(node)
	for _, name := range names {
		example := name
		if prefix != "" {
			example = prefix + "." + name
		}
		sb.WriteString("- `" + example + "`\n")
	}
	if len(names) > 0 {
		example := names[0]
		if prefix != "" {
			example = prefix + "." + example
		}
		sb.WriteString(fmt.Sprintf("\nExample: call again with `prefix: \"%s\"`.\n", example))
	}
	return sb.String()
}

func writeTree(sb *strings.Builder, node *Node, indent string) {
	for _, name := range childrenSorted(node) {
		sb.WriteString(indent + "- " + name + "\n")
		writeTree(sb, node.Children[name], indent+"  ")
	}
}

func childrenSorted(node *Node) []string {
	names := make([]string, 0, len(node.Children))
	for k := range node.Children {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

func countNodes(node *Node) int {
	c := len(node.Children)
	for _, child := range node.Children {
		c += countNodes(child)
	}
	return c
}
