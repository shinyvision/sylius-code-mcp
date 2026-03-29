package syliustranslations

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func loadYAMLDoc(path string) (*yaml.Node, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &yaml.Node{
			Kind:    yaml.DocumentNode,
			Content: []*yaml.Node{{Kind: yaml.MappingNode}},
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	if len(doc.Content) == 0 {
		doc.Content = []*yaml.Node{{Kind: yaml.MappingNode}}
	}
	return &doc, nil
}

func saveYAMLDoc(path string, doc *yaml.Node) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		_ = err
	}
	data, err := yaml.Marshal(doc)
	if err != nil {
		return fmt.Errorf("serialising %s: %w", path, err)
	}
	data = collapseExplicitKeys(data)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

func collapseExplicitKeys(data []byte) []byte {
	lines := strings.Split(string(data), "\n")
	out := make([]string, 0, len(lines))
	i := 0
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimLeft(line, " \t")
		if !strings.HasPrefix(trimmed, "? ") {
			out = append(out, line)
			i++
			continue
		}
		indent := line[:len(line)-len(trimmed)]
		keyPart := trimmed[2:]

		if i+1 >= len(lines) {
			out = append(out, line)
			i++
			continue
		}
		next := lines[i+1]
		valPrefix := indent + ": "
		valPrefixBare := indent + ":"
		var valuePart string
		if strings.HasPrefix(next, valPrefix) {
			valuePart = next[len(valPrefix):]
		} else if next == valPrefixBare {
			valuePart = ""
		} else {
			out = append(out, line)
			i++
			continue
		}

		if valuePart == "" {
			out = append(out, indent+keyPart+": \"\"")
		} else {
			out = append(out, indent+keyPart+": "+valuePart)
		}
		i += 2
	}
	return []byte(strings.Join(out, "\n"))
}

func keyExistsInDoc(doc *yaml.Node, dotKey string) bool {
	if len(doc.Content) == 0 {
		return false
	}
	segments := strings.Split(dotKey, ".")
	_, ok := lookupInMapping(doc.Content[0], segments)
	return ok
}

func lookupInMapping(mapping *yaml.Node, segments []string) (string, bool) {
	if mapping == nil || mapping.Kind != yaml.MappingNode || len(segments) == 0 {
		return "", false
	}

	seg := segments[0]
	rest := segments[1:]

	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value != seg {
			continue
		}
		val := mapping.Content[i+1]
		if len(rest) == 0 {
			if val.Kind == yaml.ScalarNode {
				return val.Value, true
			}
			return "", false
		}
		if val.Kind == yaml.MappingNode {
			return lookupInMapping(val, rest)
		}
		return "", false
	}

	flatKey := strings.Join(segments, ".")
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == flatKey {
			val := mapping.Content[i+1]
			if val.Kind == yaml.ScalarNode {
				return val.Value, true
			}
			return "", false
		}
	}

	return "", false
}

func insertInDoc(doc *yaml.Node, dotKey, value string, overwrite bool) bool {
	if len(doc.Content) == 0 {
		doc.Content = []*yaml.Node{{Kind: yaml.MappingNode}}
	}
	segments := strings.Split(dotKey, ".")
	return insertInMapping(doc.Content[0], segments, value, overwrite)
}

func insertInMapping(mapping *yaml.Node, segments []string, value string, overwrite bool) bool {
	if mapping == nil || mapping.Kind != yaml.MappingNode || len(segments) == 0 {
		return false
	}

	seg := segments[0]
	rest := segments[1:]

	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value != seg {
			continue
		}
		val := mapping.Content[i+1]

		if len(rest) == 0 {
			if !overwrite {
				return false
			}
			mapping.Content[i+1] = scalarValueNode(value)
			return true
		}

		if val.Kind == yaml.MappingNode {
			return insertInMapping(val, rest, value, overwrite)
		}

		flatKey := seg + "." + strings.Join(rest, ".")
		return addFlatKey(mapping, flatKey, value, overwrite)
	}

	if len(rest) > 0 {
		prefix := seg + "."
		for i := 0; i+1 < len(mapping.Content); i += 2 {
			if strings.HasPrefix(mapping.Content[i].Value, prefix) {
				flatKey := seg + "." + strings.Join(rest, ".")
				return addFlatKey(mapping, flatKey, value, overwrite)
			}
		}
	}

	if len(rest) == 0 {
		mapping.Content = append(mapping.Content,
			scalarKeyNode(seg),
			scalarValueNode(value))
		return true
	}

	sub := &yaml.Node{Kind: yaml.MappingNode}
	insertInMapping(sub, rest, value, overwrite)
	mapping.Content = append(mapping.Content, scalarKeyNode(seg), sub)
	return true
}

func addFlatKey(mapping *yaml.Node, flatKey, value string, overwrite bool) bool {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == flatKey {
			if !overwrite {
				return false
			}
			mapping.Content[i+1] = scalarValueNode(value)
			return true
		}
	}
	mapping.Content = append(mapping.Content,
		scalarKeyNode(flatKey),
		scalarValueNode(value))
	return true
}

func deleteFromDoc(doc *yaml.Node, dotKey string) bool {
	if len(doc.Content) == 0 {
		return false
	}
	segments := strings.Split(dotKey, ".")
	return deleteFromMapping(doc.Content[0], segments)
}

func deleteFromMapping(mapping *yaml.Node, segments []string) bool {
	if mapping == nil || mapping.Kind != yaml.MappingNode || len(segments) == 0 {
		return false
	}

	seg := segments[0]
	rest := segments[1:]

	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value != seg {
			continue
		}
		val := mapping.Content[i+1]

		if len(rest) == 0 {
			mapping.Content = append(mapping.Content[:i], mapping.Content[i+2:]...)
			return true
		}

		if val.Kind == yaml.MappingNode {
			return deleteFromMapping(val, rest)
		}

		break
	}

	if len(rest) > 0 {
		flatKey := strings.Join(segments, ".")
		for i := 0; i+1 < len(mapping.Content); i += 2 {
			if mapping.Content[i].Value == flatKey {
				mapping.Content = append(mapping.Content[:i], mapping.Content[i+2:]...)
				return true
			}
		}
	}

	return false
}

func scalarKeyNode(s string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Value: s}
}

func scalarValueNode(s string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Value: s, Tag: "!!str"}
}

func readFlatMap(data []byte) (map[string]string, error) {
	if len(data) == 0 {
		return make(map[string]string), nil
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	flat := make(map[string]string)
	if len(doc.Content) > 0 {
		walkYAML(doc.Content[0], "", flat)
	}
	return flat, nil
}

func walkYAML(node *yaml.Node, prefix string, out map[string]string) {
	if node == nil {
		return
	}
	switch node.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			k := node.Content[i].Value
			key := k
			if prefix != "" {
				key = prefix + "." + k
			}
			walkYAML(node.Content[i+1], key, out)
		}
	case yaml.ScalarNode:
		if prefix != "" {
			out[prefix] = node.Value
		}
	}
}
