package phpparser

import (
	"strings"

	sitter "github.com/alexaandru/go-tree-sitter-bare"
)

type ConstructorParam struct {
	Name         string
	PHPType      string
	Required     bool
	DefaultValue string
}

var constraintBoilerplate = map[string]bool{
	"options": true,
	"groups":  true,
	"payload": true,
}

var typeAnnotationNodeTypes = map[string]bool{
	"optional_type":     true,
	"primitive_type":    true,
	"named_type":        true,
	"union_type":        true,
	"intersection_type": true,
}

func ParseConstructorParams(content string) []ConstructorParam {
	data := []byte(content)
	tree, err := parseTree(data)
	if err != nil {
		return nil
	}
	defer tree.Close()

	constructNode := findMethodNode(tree.RootNode(), "__construct", data)
	if constructNode.IsNull() {
		return nil
	}

	formalParams := constructNode.ChildByFieldName("parameters")
	if formalParams.IsNull() {
		for i := uint32(0); i < constructNode.ChildCount(); i++ {
			child := constructNode.Child(i)
			if child.Type() == "formal_parameters" {
				formalParams = child
				break
			}
		}
	}
	if formalParams.IsNull() {
		return nil
	}

	var params []ConstructorParam
	for i := uint32(0); i < formalParams.NamedChildCount(); i++ {
		paramNode := formalParams.NamedChild(i)
		switch paramNode.Type() {
		case "simple_parameter", "property_promoted_parameter":
			if p, ok := extractParam(paramNode, data); ok {
				params = append(params, p)
			}
		}
	}
	return params
}

func ParseClassExtends(content string) string {
	data := []byte(content)
	tree, err := parseTree(data)
	if err != nil {
		return ""
	}
	defer tree.Close()

	stack := []sitter.Node{tree.RootNode()}
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if node.Type() == "class_declaration" {
			base := node.ChildByFieldName("base_clause")
			if !base.IsNull() {
				for i := uint32(0); i < base.NamedChildCount(); i++ {
					child := base.NamedChild(i)
					if child.Type() == "qualified_name" || child.Type() == "name" {
						name := child.Content(data)
						parts := strings.Split(name, `\`)
						return parts[len(parts)-1]
					}
				}
			}
			return ""
		}

		for i := uint32(0); i < node.NamedChildCount(); i++ {
			stack = append(stack, node.NamedChild(i))
		}
	}
	return ""
}

func extractParam(node sitter.Node, data []byte) (ConstructorParam, bool) {
	var p ConstructorParam
	foundName := false

	for i := uint32(0); i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		typ := child.Type()

		if typeAnnotationNodeTypes[typ] && !foundName {
			p.PHPType = strings.TrimSpace(child.Content(data))
			continue
		}

		if typ == "variable_name" {
			p.Name = variableName(child, data)
			foundName = true
			continue
		}

		if !foundName {
			continue
		}

		if p.DefaultValue == "" && typ != "attribute_list" {
			p.DefaultValue = strings.TrimSpace(child.Content(data))
		}
	}

	if p.Name == "" {
		return ConstructorParam{}, false
	}

	if constraintBoilerplate[p.Name] {
		return ConstructorParam{}, false
	}

	if strings.Contains(p.PHPType, "callable") {
		return ConstructorParam{}, false
	}

	p.Required = p.DefaultValue == ""
	return p, true
}

func variableName(node sitter.Node, data []byte) string {
	for j := uint32(0); j < node.NamedChildCount(); j++ {
		gc := node.NamedChild(j)
		if gc.Type() == "name" {
			return gc.Content(data)
		}
	}
	return strings.TrimPrefix(strings.TrimSpace(node.Content(data)), "$")
}
