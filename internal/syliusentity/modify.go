package syliusentity

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sylius-code-mcp/internal/phpparser"
)

type insertion struct {
	afterLine int
	code      string
}

func applyInsertions(content string, ins []insertion) string {
	sort.Slice(ins, func(i, j int) bool {
		return ins[i].afterLine > ins[j].afterLine
	})

	lines := strings.Split(content, "\n")
	for _, in := range ins {
		codeLines := strings.Split(strings.TrimRight(in.code, "\n"), "\n")
		idx := min(in.afterLine+1, len(lines))
		newLines := make([]string, 0, len(lines)+len(codeLines))
		newLines = append(newLines, lines[:idx]...)
		newLines = append(newLines, codeLines...)
		newLines = append(newLines, lines[idx:]...)
		lines = newLines
	}
	return strings.Join(lines, "\n")
}

func insertBeforeLine(content string, line int, code string) string {
	return applyInsertions(content, []insertion{{afterLine: line - 1, code: code}})
}

type ModifyRequest struct {
	UseStatements    []string
	Properties       []string
	Methods          []string
	ConstructorInits []string
}

func ApplyModifications(content string, req ModifyRequest) (string, error) {
	info, ok := phpparser.ParseClassInfo(content)
	if !ok {
		return "", fmt.Errorf("could not parse PHP class")
	}

	var newUses []string
	for _, u := range req.UseStatements {
		if !phpparser.HasUseStatement(content, u) {
			newUses = append(newUses, u)
		}
	}

	var ins []insertion

	if len(newUses) > 0 {
		var useBlock strings.Builder
		for _, u := range newUses {
			fmt.Fprintf(&useBlock, "use %s;\n", u)
		}
		useLine := max(info.LastUseLine, 0)
		ins = append(ins, insertion{afterLine: useLine, code: strings.TrimRight(useBlock.String(), "\n")})
	}

	if len(req.Properties) > 0 {
		propBlock := "\n" + strings.Join(req.Properties, "\n")
		propBlock = strings.TrimRight(propBlock, "\n")
		propLine := info.LastPropertyLine
		if propLine < 0 {
			propLine = info.ClassBodyStart
		}
		ins = append(ins, insertion{afterLine: propLine, code: propBlock})
	}

	if len(req.ConstructorInits) > 0 {
		initBlock := strings.Join(req.ConstructorInits, "")
		initBlock = strings.TrimRight(initBlock, "\n")
		if info.HasConstructor {
			_ = initBlock
		}
	}

	if len(req.Methods) > 0 {
		methodBlock := "\n" + strings.Join(req.Methods, "\n")
		methodBlock = strings.TrimRight(methodBlock, "\n")
		ins = append(ins, insertion{afterLine: info.ClassBodyEnd - 1, code: methodBlock})
	}

	content = applyInsertions(content, ins)

	if len(req.ConstructorInits) > 0 {
		initBlock := strings.Join(req.ConstructorInits, "")
		initBlock = strings.TrimRight(initBlock, "\n")
		if info.HasConstructor {
			var err error
			content, err = phpparser.InsertBeforeMethodEnd(content, "__construct", initBlock)
			if err != nil {
				return "", fmt.Errorf("inserting constructor init: %w", err)
			}
		}
	}

	return content, nil
}
