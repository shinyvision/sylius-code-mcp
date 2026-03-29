package syliusmenu

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/sylius-code-mcp/internal/container"
	"github.com/sylius-code-mcp/internal/phpparser"
)

const menuBuilderServiceID = "sylius_admin.menu_builder.main"

type MenuItemsResult struct {
	Tree   []MenuNode
	Report string
}

type MenuNode struct {
	Key      string
	Children []MenuNode
	Source   string
}

func GetMenuItems(projectRoot string) (MenuItemsResult, error) {
	psr4, err := container.LoadPSR4(projectRoot)
	if err != nil {
		return MenuItemsResult{}, fmt.Errorf("loading PSR-4: %w", err)
	}

	cnt, containerErr := container.Load(projectRoot)
	if containerErr != nil {
		return MenuItemsResult{}, fmt.Errorf(
			"loading container XML: %w\n(run php bin/console cache:warmup first)", containerErr)
	}

	builderAnalysis, err := analyzeMenuBuilder(projectRoot, cnt, psr4)
	if err != nil {
		return MenuItemsResult{}, err
	}

	handlerAnalyses := analyzeMenuHandlers(projectRoot, cnt, psr4)

	result := BuildMenuItemsResult(builderAnalysis, handlerAnalyses)
	return result, nil
}

func analyzeMenuBuilder(projectRoot string, cnt *container.Container, psr4 container.PSR4Map) (phpparser.MenuAnalysis, error) {
	svc, ok := cnt.Services[menuBuilderServiceID]
	if !ok {
		return phpparser.MenuAnalysis{}, fmt.Errorf("service %q not found in container", menuBuilderServiceID)
	}
	filePath, ok := container.ResolveClass(svc.Class, psr4, projectRoot)
	if !ok {
		return phpparser.MenuAnalysis{}, fmt.Errorf("could not resolve class %q to a file", svc.Class)
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return phpparser.MenuAnalysis{}, fmt.Errorf("reading %s: %w", filePath, err)
	}
	return phpparser.AnalyzeMenuFile(string(data), filePath), nil
}

func analyzeMenuHandlers(projectRoot string, cnt *container.Container, psr4 container.PSR4Map) []phpparser.MenuAnalysis {
	handlers := FindMenuHandlers(projectRoot, cnt, psr4)
	var analyses []phpparser.MenuAnalysis
	for _, h := range handlers {
		data, err := os.ReadFile(h.FilePath)
		if err != nil {
			continue
		}
		a := phpparser.AnalyzeMenuFile(string(data), h.FilePath)
		if a.ClassName == "" {
			a.ClassName = h.Class
		}
		analyses = append(analyses, a)
	}
	return analyses
}

func BuildMenuItemsResult(base phpparser.MenuAnalysis, modifiers []phpparser.MenuAnalysis) MenuItemsResult {
	type nodeInfo struct {
		key      string
		source   string
		children []string
	}

	nodes := make(map[string]*nodeInfo)
	rootNode := &nodeInfo{key: "root", source: "root"}
	nodes[""] = rootNode

	getOrCreate := func(path []string) *nodeInfo {
		key := strings.Join(path, "/")
		if n, ok := nodes[key]; ok {
			return n
		}
		n := &nodeInfo{key: path[len(path)-1]}
		nodes[key] = n
		return n
	}

	applyOps := func(ops []phpparser.MenuOp, source string) {
		for _, op := range ops {
			if op.Kind != phpparser.OpAdd {
				continue
			}
			parentKey := strings.Join(op.ParentPath, "/")
			parentNode, ok := nodes[parentKey]
			if !ok {
				parentNode = getOrCreate(op.ParentPath)
			}

			childPathKey := strings.Join(append(op.ParentPath, op.ChildKey), "/")
			if _, exists := nodes[childPathKey]; !exists {
				childNode := &nodeInfo{key: op.ChildKey, source: source}
				nodes[childPathKey] = childNode
				parentNode.children = append(parentNode.children, op.ChildKey)
			}
		}
	}

	relPath := func(filePath string) string {
		idx := strings.LastIndex(filePath, "/src/")
		if idx >= 0 {
			return "src" + filePath[idx+4:]
		}
		return filePath
	}

	applyOps(base.Ops, base.ClassName)
	for _, m := range modifiers {
		src := m.ClassName
		if m.FilePath != "" {
			src += " (" + relPath(m.FilePath) + ")"
		}
		applyOps(m.Ops, src)
	}

	type removeEntry struct {
		parentPath []string
		childKey   string
		source     string
	}
	var removes []removeEntry
	for _, m := range modifiers {
		src := m.ClassName
		if m.FilePath != "" {
			src += " (" + relPath(m.FilePath) + ")"
		}
		for _, op := range m.Ops {
			if op.Kind == phpparser.OpRemove {
				removes = append(removes, removeEntry{op.ParentPath, op.ChildKey, src})
			}
		}
	}

	var buildNodes func(path []string) []MenuNode
	buildNodes = func(path []string) []MenuNode {
		key := strings.Join(path, "/")
		n, ok := nodes[key]
		if !ok {
			return nil
		}
		var result []MenuNode
		for _, childKey := range n.children {
			childPath := append(path, childKey)
			childKey_ := strings.Join(childPath, "/")
			childNode := nodes[childKey_]
			src := ""
			if childNode != nil {
				src = childNode.source
			}
			result = append(result, MenuNode{
				Key:      childKey,
				Children: buildNodes(childPath),
				Source:   src,
			})
		}
		return result
	}

	tree := buildNodes([]string{})

	var sb strings.Builder
	sb.WriteString("## Sylius admin menu structure\n\n")
	sb.WriteString("### Base menu items (from " + base.ClassName + ")\n\n")

	var writeTree func(nodes []MenuNode, indent string)
	writeTree = func(nodes []MenuNode, indent string) {
		for _, n := range nodes {
			src := ""
			if n.Source != "" && n.Source != base.ClassName {
				src = " [" + n.Source + "]"
			}
			sb.WriteString(fmt.Sprintf("%s- %s%s\n", indent, n.Key, src))
			writeTree(n.Children, indent+"  ")
		}
	}
	writeTree(tree, "")

	if len(removes) > 0 {
		sb.WriteString("\n### Items removed by subscribers\n\n")
		for _, r := range removes {
			parent := "root"
			if len(r.parentPath) > 0 {
				parent = strings.Join(r.parentPath, " > ")
			}
			sb.WriteString(fmt.Sprintf("- %s > **%s** (removed by %s)\n", parent, r.childKey, r.source))
		}
	}

	sb.WriteString("\n### Top-level menu keys (use as `parentKey` in sylius_admin_add_menu_item)\n\n")
	topKeys := collectTopLevelKeys(tree)
	sort.Strings(topKeys)
	for _, k := range topKeys {
		sb.WriteString(fmt.Sprintf("- `%s`\n", k))
	}

	return MenuItemsResult{Tree: tree, Report: sb.String()}
}

func collectTopLevelKeys(nodes []MenuNode) []string {
	keys := make([]string, 0, len(nodes))
	for _, n := range nodes {
		keys = append(keys, n.Key)
	}
	return keys
}
