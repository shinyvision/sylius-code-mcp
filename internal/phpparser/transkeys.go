package phpparser

import (
	"maps"
	"regexp"
	"strings"

	sitter "github.com/alexaandru/go-tree-sitter-bare"
)

type TransKeySource struct {
	Key  string
	File string
}

var twigTransSimpleRe = regexp.MustCompile(`['"]((app\.[a-zA-Z0-9_.]+))['"][ \t]*\|trans`)

func ExtractTwigTransKeys(content, filePath string) []TransKeySource {
	var result []TransKeySource
	seen := make(map[string]struct{})
	for _, m := range twigTransSimpleRe.FindAllStringSubmatch(content, -1) {
		key := m[1]
		if _, dup := seen[key]; !dup {
			seen[key] = struct{}{}
			result = append(result, TransKeySource{Key: key, File: filePath})
		}
	}
	return result
}

var menuLabelRe = regexp.MustCompile(`->setLabel\(['"]((app\.[a-zA-Z0-9_.]+))['"]\)`)

func ExtractMenuLabelKeys(content, filePath string) []TransKeySource {
	var result []TransKeySource
	seen := make(map[string]struct{})
	for _, m := range menuLabelRe.FindAllStringSubmatch(content, -1) {
		key := m[1]
		if _, dup := seen[key]; !dup {
			seen[key] = struct{}{}
			result = append(result, TransKeySource{Key: key, File: filePath})
		}
	}
	return result
}

var translatorTypeHints = map[string]bool{
	"TranslatorInterface":    true,
	"TranslatorBagInterface": true,
	"TranslatorBag":          true,
	"Translator":             true,
}

func ExtractPHPTransKeys(content, filePath string) []TransKeySource {
	data := []byte(content)
	tree, err := parseTree(data)
	if err != nil {
		return nil
	}
	defer tree.Close()

	root := tree.RootNode()

	translatorVars := collectTranslatorVars(root, data, content)
	if len(translatorVars) == 0 {
		return nil
	}

	seen := make(map[string]struct{})
	var result []TransKeySource

	walkAllMethods(root, func(body sitter.Node) {
		localVars := collectLocalTranslatorVars(body, data, translatorVars)

		allVars := make(map[string]bool, len(translatorVars)+len(localVars))
		maps.Copy(allVars, translatorVars)
		maps.Copy(allVars, localVars)

		collectTransCalls(body, data, allVars, func(key string) {
			if _, dup := seen[key]; !dup {
				seen[key] = struct{}{}
				result = append(result, TransKeySource{Key: key, File: filePath})
			}
		})
	})

	return result
}

func collectTranslatorVars(root sitter.Node, data []byte, content string) map[string]bool {
	vars := make(map[string]bool)

	if strings.Contains(content, "TranslatorAwareTrait") {
		vars["$this->translator"] = true
		vars["getTranslator"] = true
	}

	stack := []sitter.Node{root}
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		switch node.Type() {
		case "property_declaration":
			processPropertyDecl(node, data, vars)
			continue

		case "method_declaration":
			nameNode := node.ChildByFieldName("name")
			if !nameNode.IsNull() && nameNode.Content(data) == "__construct" {
				params := node.ChildByFieldName("parameters")
				if !params.IsNull() {
					processConstructorParams(params, data, vars)
				}
			}
		}

		for i := uint32(0); i < node.NamedChildCount(); i++ {
			stack = append(stack, node.NamedChild(i))
		}
	}
	return vars
}

func processPropertyDecl(node sitter.Node, data []byte, vars map[string]bool) {
	var typeNode, nameNode sitter.Node
	for i := uint32(0); i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		switch child.Type() {
		case "named_type", "optional_type", "primitive_type":
			typeNode = child
		case "property_element":
			for j := uint32(0); j < child.NamedChildCount(); j++ {
				gc := child.NamedChild(j)
				if gc.Type() == "variable_name" {
					nameNode = gc
				}
			}
		}
	}
	if typeNode.IsNull() || nameNode.IsNull() {
		return
	}
	typeStr := strings.TrimSpace(typeNode.Content(data))
	if idx := strings.LastIndex(typeStr, "\\"); idx >= 0 {
		typeStr = typeStr[idx+1:]
	}
	typeStr = strings.TrimPrefix(typeStr, "?")
	if !translatorTypeHints[typeStr] {
		return
	}
	propName := strings.TrimSpace(nameNode.Content(data))
	for j := uint32(0); j < nameNode.NamedChildCount(); j++ {
		gc := nameNode.NamedChild(j)
		if gc.Type() == "name" {
			propName = gc.Content(data)
			break
		}
	}
	propName = strings.TrimPrefix(propName, "$")
	vars["$this->"+propName] = true
}

func processConstructorParams(params sitter.Node, data []byte, vars map[string]bool) {
	for i := uint32(0); i < params.NamedChildCount(); i++ {
		param := params.NamedChild(i)
		typeNode := param.ChildByFieldName("type")
		nameNode := param.ChildByFieldName("name")
		if typeNode.IsNull() || nameNode.IsNull() {
			continue
		}
		typeStr := strings.TrimSpace(typeNode.Content(data))
		if idx := strings.LastIndex(typeStr, "\\"); idx >= 0 {
			typeStr = typeStr[idx+1:]
		}
		typeStr = strings.TrimPrefix(typeStr, "?")
		if !translatorTypeHints[typeStr] {
			continue
		}
		varName := strings.TrimSpace(nameNode.Content(data))
		vars[varName] = true
	}
}

func collectLocalTranslatorVars(body sitter.Node, data []byte, knownVars map[string]bool) map[string]bool {
	local := make(map[string]bool)
	stack := []sitter.Node{body}
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if node.Type() == "assignment_expression" {
			left := node.ChildByFieldName("left")
			right := node.ChildByFieldName("right")
			if !left.IsNull() && !right.IsNull() && left.Type() == "variable_name" {
				lhs := strings.TrimSpace(left.Content(data))
				rhs := strings.TrimSpace(right.Content(data))

				if knownVars[rhs] || local[rhs] {
					local[lhs] = true
					continue
				}
				if isGetTranslatorCall(right, data) {
					local[lhs] = true
					continue
				}
			}
		}

		for i := uint32(0); i < node.NamedChildCount(); i++ {
			stack = append(stack, node.NamedChild(i))
		}
	}
	return local
}

func isGetTranslatorCall(node sitter.Node, data []byte) bool {
	if node.IsNull() {
		return false
	}
	if node.Type() != "member_call_expression" && node.Type() != "nullsafe_member_call_expression" {
		return false
	}
	nameNode := node.ChildByFieldName("name")
	objectNode := node.ChildByFieldName("object")
	if nameNode.IsNull() || objectNode.IsNull() {
		return false
	}
	return nameNode.Content(data) == "getTranslator" &&
		strings.TrimSpace(objectNode.Content(data)) == "$this"
}

func collectTransCalls(node sitter.Node, data []byte, allVars map[string]bool, emit func(string)) {
	if node.IsNull() {
		return
	}

	if node.Type() == "member_call_expression" || node.Type() == "nullsafe_member_call_expression" {
		nameNode := node.ChildByFieldName("name")
		objectNode := node.ChildByFieldName("object")
		if !nameNode.IsNull() && !objectNode.IsNull() && nameNode.Content(data) == "trans" {
			receiver := receiverExpr(objectNode, data)
			if allVars[receiver] || (receiver == "getTranslator" && allVars["getTranslator"]) {
				key := firstStringArgFromCall(node, data)
				if key != "" && strings.HasPrefix(key, "app.") {
					emit(key)
				}
			}
		}
	}

	for i := uint32(0); i < node.NamedChildCount(); i++ {
		collectTransCalls(node.NamedChild(i), data, allVars, emit)
	}
}

func receiverExpr(node sitter.Node, data []byte) string {
	if node.IsNull() {
		return ""
	}
	switch node.Type() {
	case "variable_name":
		return strings.TrimSpace(node.Content(data))
	case "member_access_expression", "nullsafe_member_access_expression":
		obj := node.ChildByFieldName("object")
		name := node.ChildByFieldName("name")
		if obj.IsNull() || name.IsNull() {
			return ""
		}
		if strings.TrimSpace(obj.Content(data)) == "$this" {
			return "$this->" + strings.TrimSpace(name.Content(data))
		}
	case "member_call_expression", "nullsafe_member_call_expression":
		nameNode := node.ChildByFieldName("name")
		objNode := node.ChildByFieldName("object")
		if !nameNode.IsNull() && !objNode.IsNull() {
			if strings.TrimSpace(objNode.Content(data)) == "$this" {
				return nameNode.Content(data)
			}
		}
	}
	return ""
}

func firstStringArgFromCall(call sitter.Node, data []byte) string {
	argsNode := call.ChildByFieldName("arguments")
	if argsNode.IsNull() {
		return ""
	}
	for i := uint32(0); i < argsNode.NamedChildCount(); i++ {
		arg := argsNode.NamedChild(i)
		val := argExprNode(arg)
		if val.IsNull() {
			continue
		}
		s := extractStringValue(val, data)
		if s != "" {
			return s
		}
		return ""
	}
	return ""
}

func walkAllMethods(root sitter.Node, fn func(sitter.Node)) {
	stack := []sitter.Node{root}
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if node.Type() == "method_declaration" {
			body := node.ChildByFieldName("body")
			if !body.IsNull() {
				fn(body)
			}
			continue
		}

		for i := uint32(0); i < node.NamedChildCount(); i++ {
			stack = append(stack, node.NamedChild(i))
		}
	}
}
