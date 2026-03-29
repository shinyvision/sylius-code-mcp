package phpparser

import (
	"strings"

	sitter "github.com/alexaandru/go-tree-sitter-bare"
)

type ClassInfo struct {
	LastUseLine       int
	LastPropertyLine  int
	ClassBodyStart    int
	ClassBodyEnd      int
	HasConstructor    bool
	ConstructorBounds MethodBounds
	TableName         string
}

func ParseClassInfo(content string) (ClassInfo, bool) {
	data := []byte(content)
	tree, err := parseTree(data)
	if err != nil {
		return ClassInfo{}, false
	}
	defer tree.Close()

	var info ClassInfo
	info.LastUseLine = -1
	info.LastPropertyLine = -1

	root := tree.RootNode()
	hasClass := false

	stack := []sitter.Node{root}
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		switch node.Type() {
		case "namespace_use_declaration":
			if line := int(node.EndPoint().Row); line > info.LastUseLine {
				info.LastUseLine = line
			}
			continue

		case "property_declaration":
			if line := int(node.EndPoint().Row); line > info.LastPropertyLine {
				info.LastPropertyLine = line
			}
			continue

		case "class_declaration":
			hasClass = true
			body := node.ChildByFieldName("body")
			if !body.IsNull() {
				info.ClassBodyStart = int(body.StartPoint().Row)
				info.ClassBodyEnd = int(body.EndPoint().Row)
			}
			for i := uint32(0); i < node.ChildCount(); i++ {
				child := node.Child(i)
				if child.Type() == "attribute_list" {
					if t := extractAttributeArg(child, data, "Table", "name"); t != "" {
						info.TableName = t
					}
				}
			}

		case "method_declaration":
			nameNode := node.ChildByFieldName("name")
			if !nameNode.IsNull() && nameNode.Content(data) == "__construct" {
				info.HasConstructor = true
				info.ConstructorBounds = MethodBounds{
					StartLine: int(node.StartPoint().Row),
					EndLine:   int(node.EndPoint().Row),
				}
			}
		}

		for i := uint32(0); i < node.NamedChildCount(); i++ {
			stack = append(stack, node.NamedChild(i))
		}
	}

	if !hasClass {
		return ClassInfo{}, false
	}
	return info, true
}

func extractAttributeArg(attrList sitter.Node, data []byte, attrSuffix, argName string) string {
	stack := []sitter.Node{attrList}
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if node.Type() == "attribute" {
			nameNode := sitter.Node{}
			argsNode := sitter.Node{}
			for i := uint32(0); i < node.NamedChildCount(); i++ {
				child := node.NamedChild(i)
				switch child.Type() {
				case "qualified_name", "name":
					nameNode = child
				case "arguments":
					argsNode = child
				}
			}
			if nameNode.IsNull() || argsNode.IsNull() {
				continue
			}
			if !strings.HasSuffix(nameNode.Content(data), attrSuffix) {
				continue
			}
			for i := uint32(0); i < argsNode.NamedChildCount(); i++ {
				arg := argsNode.NamedChild(i)
				if arg.Type() != "argument" || arg.NamedChildCount() < 2 {
					continue
				}
				first := arg.NamedChild(0)
				if first.Type() != "name" || first.Content(data) != argName {
					continue
				}
				last := arg.NamedChild(arg.NamedChildCount() - 1)
				return extractStringValue(last, data)
			}
			continue
		}

		for i := uint32(0); i < node.NamedChildCount(); i++ {
			stack = append(stack, node.NamedChild(i))
		}
	}
	return ""
}

func HasUseStatement(content, fqcn string) bool {
	return strings.Contains(content, "use "+fqcn+";") ||
		strings.Contains(content, "use "+fqcn+" ")
}
