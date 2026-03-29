package yamlutil

import (
	"gopkg.in/yaml.v3"
)

func MappingNode(pairs ...*yaml.Node) *yaml.Node {
	if len(pairs)%2 != 0 {
		panic("MappingNode: odd number of arguments")
	}
	return &yaml.Node{Kind: yaml.MappingNode, Content: pairs}
}

func AddField(m, key, value *yaml.Node) {
	m.Content = append(m.Content, key, value)
}

func Scalar(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Value: value}
}

func DoubleQuotedScalar(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Style: yaml.DoubleQuotedStyle, Value: value}
}

func SingleQuotedScalar(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Style: yaml.SingleQuotedStyle, Value: value}
}

func LiteralScalar(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Style: yaml.LiteralStyle, Value: value}
}

func BoolNode(v bool) *yaml.Node {
	value := "false"
	if v {
		value = "true"
	}
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: value}
}

func FlowSequenceNode(values []string) *yaml.Node {
	n := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}
	for _, v := range values {
		n.Content = append(n.Content, SingleQuotedScalar(v))
	}
	return n
}

func MarshalNode(n *yaml.Node) (string, error) {
	out, err := yaml.Marshal(n)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func FindInMapping(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func NavigatePath(root *yaml.Node, path ...string) *yaml.Node {
	if root == nil {
		return nil
	}
	cur := root
	if cur.Kind == yaml.DocumentNode {
		if len(cur.Content) == 0 {
			return nil
		}
		cur = cur.Content[0]
	}
	for _, key := range path {
		cur = FindInMapping(cur, key)
		if cur == nil {
			return nil
		}
	}
	return cur
}
