package phpparser

import (
	"strings"

	sitter "github.com/alexaandru/go-tree-sitter-bare"
)

type OpKind string

const (
	OpAdd    OpKind = "add"
	OpRemove OpKind = "remove"
)

type MenuOp struct {
	Kind       OpKind
	ParentPath []string
	ChildKey   string
	MethodName string
}

type MenuAnalysis struct {
	FilePath  string
	ClassName string
	Ops       []MenuOp
}

type scope map[string][]string

type astParamDef struct {
	typeHint string
	name     string
}

type astMethodDef struct {
	name      string
	paramDefs []astParamDef
	params    sitter.Node
	body      sitter.Node
}

func AnalyzeMenuFile(content, filePath string) MenuAnalysis {
	data := []byte(content)
	tree, err := parseTree(data)
	if err != nil {
		return MenuAnalysis{FilePath: filePath}
	}
	defer tree.Close()

	root := tree.RootNode()
	className := findClassName(root, data)
	methods := collectMethods(root, data)

	registry := make(map[string]astMethodDef, len(methods))
	for _, m := range methods {
		registry[m.name] = m
	}

	var ops []MenuOp
	for _, m := range methods {
		if isEntryPoint(m, data) {
			s := buildInitialScopeFromParams(m.params, data)
			ops = append(ops, analyzeNode(m.body, m.name, s, registry, data, 0)...)
		}
	}

	return MenuAnalysis{
		FilePath:  filePath,
		ClassName: className,
		Ops:       deduplicateOps(ops),
	}
}

func findClassName(root sitter.Node, data []byte) string {
	for i := uint32(0); i < root.NamedChildCount(); i++ {
		node := root.NamedChild(i)
		if node.Type() == "class_declaration" {
			if nameNode := node.ChildByFieldName("name"); !nameNode.IsNull() {
				return strings.TrimSpace(nameNode.Content(data))
			}
		}
		if node.Type() == "namespace_definition" || node.Type() == "declaration_list" {
			for j := uint32(0); j < node.NamedChildCount(); j++ {
				child := node.NamedChild(j)
				if child.Type() == "class_declaration" {
					if nameNode := child.ChildByFieldName("name"); !nameNode.IsNull() {
						return strings.TrimSpace(nameNode.Content(data))
					}
				}
			}
		}
	}
	return ""
}

func collectMethods(root sitter.Node, data []byte) []astMethodDef {
	var methods []astMethodDef
	walkForMethods(root, data, &methods)
	return methods
}

func walkForMethods(node sitter.Node, data []byte, methods *[]astMethodDef) {
	if node.IsNull() {
		return
	}
	if node.Type() == "method_declaration" {
		if m, ok := extractMethodDef(node, data); ok {
			*methods = append(*methods, m)
		}
		return
	}
	for i := uint32(0); i < node.NamedChildCount(); i++ {
		walkForMethods(node.NamedChild(i), data, methods)
	}
}

func extractMethodDef(node sitter.Node, data []byte) (astMethodDef, bool) {
	nameNode := node.ChildByFieldName("name")
	if nameNode.IsNull() {
		return astMethodDef{}, false
	}
	name := strings.TrimSpace(nameNode.Content(data))

	body := node.ChildByFieldName("body")
	if body.IsNull() {
		return astMethodDef{}, false
	}

	params := node.ChildByFieldName("parameters")
	paramDefs := extractParamDefs(params, data)

	return astMethodDef{
		name:      name,
		paramDefs: paramDefs,
		params:    params,
		body:      body,
	}, true
}

func extractParamDefs(params sitter.Node, data []byte) []astParamDef {
	if params.IsNull() {
		return nil
	}
	var result []astParamDef
	for i := uint32(0); i < params.NamedChildCount(); i++ {
		param := params.NamedChild(i)
		nameNode := param.ChildByFieldName("name")
		if nameNode.IsNull() {
			continue
		}
		name := strings.TrimSpace(nameNode.Content(data))

		typeHint := ""
		if typeNode := param.ChildByFieldName("type"); !typeNode.IsNull() {
			full := strings.TrimSpace(typeNode.Content(data))
			if idx := strings.LastIndex(full, "\\"); idx >= 0 {
				typeHint = full[idx+1:]
			} else {
				typeHint = full
			}
		}

		result = append(result, astParamDef{typeHint: typeHint, name: name})
	}
	return result
}

func isEntryPoint(m astMethodDef, data []byte) bool {
	return containsRootCall(m.body, data) || hasItemInterfaceParam(m.params, data)
}

func containsRootCall(node sitter.Node, data []byte) bool {
	if node.IsNull() {
		return false
	}
	if node.Type() == "member_call_expression" || node.Type() == "nullsafe_member_call_expression" {
		if nameNode := node.ChildByFieldName("name"); !nameNode.IsNull() {
			name := strings.TrimSpace(nameNode.Content(data))
			switch name {
			case "getMenu":
				return true
			case "createItem":
				if firstStringArg(node, data) == "root" {
					return true
				}
			}
		}
	}
	for i := uint32(0); i < node.NamedChildCount(); i++ {
		if containsRootCall(node.NamedChild(i), data) {
			return true
		}
	}
	return false
}

func hasItemInterfaceParam(params sitter.Node, data []byte) bool {
	if params.IsNull() {
		return false
	}
	for i := uint32(0); i < params.NamedChildCount(); i++ {
		param := params.NamedChild(i)
		if typeNode := param.ChildByFieldName("type"); !typeNode.IsNull() {
			if strings.Contains(typeNode.Content(data), "ItemInterface") {
				return true
			}
		}
	}
	return false
}

func buildInitialScopeFromParams(params sitter.Node, data []byte) scope {
	s := make(scope)
	if params.IsNull() {
		return s
	}
	for i := uint32(0); i < params.NamedChildCount(); i++ {
		param := params.NamedChild(i)
		typeNode := param.ChildByFieldName("type")
		nameNode := param.ChildByFieldName("name")
		if typeNode.IsNull() || nameNode.IsNull() {
			continue
		}
		if strings.Contains(typeNode.Content(data), "ItemInterface") {
			varN := strings.TrimSpace(nameNode.Content(data))
			s[varN] = []string{}
		}
	}
	return s
}

func analyzeNode(node sitter.Node, methodName string, s scope, registry map[string]astMethodDef, data []byte, depth int) []MenuOp {
	if node.IsNull() || depth > 5 {
		return nil
	}

	var ops []MenuOp

	switch node.Type() {
	case "expression_statement":
		ops = append(ops, processExprStmt(node, methodName, s, registry, data, depth)...)
		return ops
	case "comment":
		return nil
	default:
		for i := uint32(0); i < node.NamedChildCount(); i++ {
			ops = append(ops, analyzeNode(node.NamedChild(i), methodName, s, registry, data, depth)...)
		}
	}
	return ops
}

func processExprStmt(stmt sitter.Node, methodName string, s scope, registry map[string]astMethodDef, data []byte, depth int) []MenuOp {
	if stmt.NamedChildCount() == 0 {
		return nil
	}
	expr := stmt.NamedChild(0)
	if expr.IsNull() {
		return nil
	}

	switch expr.Type() {
	case "assignment_expression":
		return processAssignment(expr, methodName, s, registry, data, depth)
	case "member_call_expression", "nullsafe_member_call_expression":
		ops, _ := processMemberCall(expr, "", methodName, s, registry, data, depth)
		return ops
	}
	return nil
}

func processAssignment(expr sitter.Node, methodName string, s scope, registry map[string]astMethodDef, data []byte, depth int) []MenuOp {
	left := expr.ChildByFieldName("left")
	right := expr.ChildByFieldName("right")
	if left.IsNull() || right.IsNull() {
		return nil
	}

	resultVar := ""
	if left.Type() == "variable_name" {
		resultVar = strings.TrimSpace(left.Content(data))
	}

	ops, _ := processMemberCall(right, resultVar, methodName, s, registry, data, depth)
	return ops
}

func processMemberCall(expr sitter.Node, resultVar string, methodName string, s scope, registry map[string]astMethodDef, data []byte, depth int) ([]MenuOp, []string) {
	if expr.IsNull() {
		return nil, nil
	}

	typ := expr.Type()
	if typ != "member_call_expression" && typ != "nullsafe_member_call_expression" {
		return nil, nil
	}

	nameNode := expr.ChildByFieldName("name")
	if nameNode.IsNull() {
		return nil, nil
	}
	callName := strings.TrimSpace(nameNode.Content(data))
	object := expr.ChildByFieldName("object")

	switch callName {
	case "getMenu":
		if resultVar != "" {
			s[resultVar] = []string{}
		}
		return nil, []string{}

	case "createItem":
		if firstStringArg(expr, data) == "root" {
			if resultVar != "" {
				s[resultVar] = []string{}
			}
			return nil, []string{}
		}

	case "addChild":
		key := firstStringArg(expr, data)
		if key == "" {
			return nil, nil
		}
		parentVar := objectVarName(object, data)
		if parentVar == "" {
			return nil, nil
		}
		parentPath, ok := s[parentVar]
		if !ok {
			return nil, nil
		}
		op := MenuOp{
			Kind:       OpAdd,
			ParentPath: clonePath(parentPath),
			ChildKey:   key,
			MethodName: methodName,
		}
		newPath := extendPath(parentPath, key)
		if resultVar != "" {
			s[resultVar] = newPath
		}
		return []MenuOp{op}, newPath

	case "getChild":
		key := firstStringArg(expr, data)
		if key == "" {
			return nil, nil
		}
		parentVar := objectVarName(object, data)
		if parentVar == "" {
			return nil, nil
		}
		parentPath, ok := s[parentVar]
		if !ok {
			return nil, nil
		}
		newPath := extendPath(parentPath, key)
		if resultVar != "" {
			s[resultVar] = newPath
		}
		return nil, newPath

	case "removeChild":
		key := firstStringArg(expr, data)
		if key == "" {
			return nil, nil
		}
		parentVar := objectVarName(object, data)
		if parentVar == "" {
			return nil, nil
		}
		parentPath, ok := s[parentVar]
		if !ok {
			return nil, nil
		}
		op := MenuOp{
			Kind:       OpRemove,
			ParentPath: clonePath(parentPath),
			ChildKey:   key,
			MethodName: methodName,
		}
		return []MenuOp{op}, nil
	}

	if !object.IsNull() && object.Type() == "variable_name" &&
		strings.TrimSpace(object.Content(data)) == "$this" && depth < 5 {
		arg := firstVarArg(expr, data)
		if arg != "" {
			argPath, ok := s[arg]
			if ok {
				if callee, ok := registry[callName]; ok {
					calleeScope := buildCalleeScope(callee, arg, argPath)
					subOps := analyzeNode(callee.body, callee.name, calleeScope, registry, data, depth+1)
					return subOps, nil
				}
			}
		}
	}

	if !object.IsNull() {
		return processMemberCall(object, resultVar, methodName, s, registry, data, depth)
	}

	return nil, nil
}

func buildCalleeScope(callee astMethodDef, arg string, argPath []string) scope {
	s := make(scope)
	for _, p := range callee.paramDefs {
		if strings.Contains(p.typeHint, "ItemInterface") || p.name == arg {
			s[p.name] = clonePath(argPath)
			return s
		}
	}
	if len(callee.paramDefs) > 0 {
		s[callee.paramDefs[0].name] = clonePath(argPath)
	}
	return s
}

func objectVarName(object sitter.Node, data []byte) string {
	if object.IsNull() || object.Type() != "variable_name" {
		return ""
	}
	return strings.TrimSpace(object.Content(data))
}

func firstStringArg(call sitter.Node, data []byte) string {
	argsNode := call.ChildByFieldName("arguments")
	if argsNode.IsNull() {
		return ""
	}
	for i := uint32(0); i < argsNode.NamedChildCount(); i++ {
		arg := argsNode.NamedChild(i)
		valNode := argExprNode(arg)
		if valNode.IsNull() {
			continue
		}
		val := extractStringValue(valNode, data)
		if val != "" {
			return val
		}
		return ""
	}
	return ""
}

func firstVarArg(call sitter.Node, data []byte) string {
	argsNode := call.ChildByFieldName("arguments")
	if argsNode.IsNull() {
		return ""
	}
	for i := uint32(0); i < argsNode.NamedChildCount(); i++ {
		arg := argsNode.NamedChild(i)
		valNode := argExprNode(arg)
		if !valNode.IsNull() && valNode.Type() == "variable_name" {
			return strings.TrimSpace(valNode.Content(data))
		}
	}
	return ""
}

func argExprNode(arg sitter.Node) sitter.Node {
	if arg.IsNull() {
		return sitter.Node{}
	}
	if arg.Type() == "argument" {
		if arg.NamedChildCount() > 0 {
			return arg.NamedChild(arg.NamedChildCount() - 1)
		}
		return sitter.Node{}
	}
	return arg
}

func clonePath(p []string) []string {
	if len(p) == 0 {
		return []string{}
	}
	c := make([]string, len(p))
	copy(c, p)
	return c
}

func extendPath(p []string, key string) []string {
	c := make([]string, len(p)+1)
	copy(c, p)
	c[len(p)] = key
	return c
}

func deduplicateOps(ops []MenuOp) []MenuOp {
	type key struct {
		kind     OpKind
		parent   string
		childKey string
		method   string
	}
	seen := make(map[key]struct{})
	result := ops[:0:0]
	for _, op := range ops {
		k := key{op.Kind, strings.Join(op.ParentPath, "/"), op.ChildKey, op.MethodName}
		if _, exists := seen[k]; exists {
			continue
		}
		seen[k] = struct{}{}
		result = append(result, op)
	}
	return result
}
