package phpparser

import (
	"strings"

	sitter "github.com/alexaandru/go-tree-sitter-bare"
)

type EntityProperty struct {
	Name    string
	PHPType string
	ORMType string
}

func ParseEntityProperties(content string) []EntityProperty {
	data := []byte(content)
	if !hasPhpTag(data) {
		data = append([]byte("<?php\n"), data...)
	}
	tree, err := parseTree(data)
	if err != nil {
		return nil
	}
	defer tree.Close()
	return collectEntityProperties(tree.RootNode(), data)
}

func collectEntityProperties(root sitter.Node, data []byte) []EntityProperty {
	var props []EntityProperty
	stack := []sitter.Node{root}
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if node.Type() == "property_declaration" {
			if p, ok := propertyFromDeclaration(node, data); ok {
				props = append(props, p)
			}
			continue
		}

		for i := uint32(0); i < node.NamedChildCount(); i++ {
			stack = append(stack, node.NamedChild(i))
		}
	}
	return props
}

func propertyFromDeclaration(node sitter.Node, data []byte) (EntityProperty, bool) {
	var prop EntityProperty

	for i := uint32(0); i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		switch child.Type() {
		case "attribute_list":
			prop.ORMType = extractORMColumnType(child, data)
		case "optional_type", "primitive_type", "named_type",
			"union_type", "intersection_type":
			prop.PHPType = strings.TrimSpace(child.Content(data))
		case "property_element":
			if name := extractPropertyElementName(child, data); name != "" {
				prop.Name = name
			}
		}
	}

	if prop.Name == "" {
		return EntityProperty{}, false
	}
	return prop, true
}

func extractPropertyElementName(propElem sitter.Node, data []byte) string {
	for i := uint32(0); i < propElem.NamedChildCount(); i++ {
		child := propElem.NamedChild(i)
		if child.Type() != "variable_name" {
			continue
		}
		for j := uint32(0); j < child.NamedChildCount(); j++ {
			gc := child.NamedChild(j)
			if gc.Type() == "name" {
				return gc.Content(data)
			}
		}
		return strings.TrimPrefix(strings.TrimSpace(child.Content(data)), "$")
	}
	return ""
}

func extractORMColumnType(attrList sitter.Node, data []byte) string {
	stack := []sitter.Node{attrList}
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if node.Type() == "attribute" {
			if t := maybeColumnType(node, data); t != "" {
				return t
			}
			continue
		}

		for i := uint32(0); i < node.NamedChildCount(); i++ {
			stack = append(stack, node.NamedChild(i))
		}
	}
	return ""
}

func maybeColumnType(attrNode sitter.Node, data []byte) string {
	var isColumn bool
	var argsNode sitter.Node

	for i := uint32(0); i < attrNode.NamedChildCount(); i++ {
		child := attrNode.NamedChild(i)
		switch child.Type() {
		case "qualified_name", "name":
			if strings.HasSuffix(child.Content(data), "Column") {
				isColumn = true
			}
		case "arguments":
			argsNode = child
		}
	}

	if !isColumn || argsNode.IsNull() {
		return ""
	}

	for i := uint32(0); i < argsNode.NamedChildCount(); i++ {
		arg := argsNode.NamedChild(i)
		if arg.Type() != "argument" || arg.NamedChildCount() < 2 {
			continue
		}
		first := arg.NamedChild(0)
		if first.Type() != "name" || first.Content(data) != "type" {
			continue
		}
		last := arg.NamedChild(arg.NamedChildCount() - 1)
		return extractStringValue(last, data)
	}
	return ""
}
