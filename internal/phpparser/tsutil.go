package phpparser

import (
	"context"

	phpforest "github.com/alexaandru/go-sitter-forest/php"
	sitter "github.com/alexaandru/go-tree-sitter-bare"
)

func parseTree(content []byte) (*sitter.Tree, error) {
	parser := sitter.NewParser()
	lang := sitter.NewLanguage(phpforest.GetLanguage())
	_ = parser.SetLanguage(lang)
	return parser.ParseString(context.Background(), nil, content)
}

func findMethodNode(root sitter.Node, methodName string, data []byte) sitter.Node {
	stack := []sitter.Node{root}
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		switch node.Type() {
		case "method_declaration", "function_definition", "function_declaration":
			if nameNode := node.ChildByFieldName("name"); !nameNode.IsNull() {
				if nameNode.Content(data) == methodName {
					return node
				}
			}
		}

		for i := uint32(0); i < node.NamedChildCount(); i++ {
			stack = append(stack, node.NamedChild(i))
		}
	}
	return sitter.Node{}
}
