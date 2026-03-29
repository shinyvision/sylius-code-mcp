package syliusentity

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sylius-code-mcp/internal/phpparser"
)

type IndexParams struct {
	EntityClass string   `json:"entityClass"`
	Fields      []string `json:"fields"`
}

type IndexResult struct {
	EntityFile string
	IndexName  string
	Messages   []string
}

func EnsureIndex(projectRoot string, p IndexParams) (IndexResult, error) {
	if p.EntityClass == "" {
		return IndexResult{}, fmt.Errorf("entityClass is required")
	}
	if len(p.Fields) == 0 {
		return IndexResult{}, fmt.Errorf("at least one field is required")
	}

	relPath := entityRelPath(p.EntityClass)
	absPath := filepath.Join(projectRoot, relPath)

	data, err := os.ReadFile(absPath)
	if err != nil {
		return IndexResult{}, fmt.Errorf("reading entity %s: %w", relPath, err)
	}
	content := string(data)

	info, ok := phpparser.ParseClassInfo(content)
	if !ok {
		return IndexResult{}, fmt.Errorf("could not parse PHP class in %s", relPath)
	}

	tableName := info.TableName
	if tableName == "" {
		tableName = camelToSnake(classShortName(p.EntityClass))
	}

	snakeFields := make([]string, len(p.Fields))
	for i, f := range p.Fields {
		snakeFields[i] = camelToSnake(f)
	}
	indexName := tableName + "_" + strings.Join(snakeFields, "_") + "_idx"

	quotedFields := make([]string, len(p.Fields))
	for i, f := range p.Fields {
		quotedFields[i] = "'" + f + "'"
	}
	attrLine := fmt.Sprintf("#[ORM\\Index(name: '%s', fields: [%s])]",
		indexName, strings.Join(quotedFields, ", "))

	if info.ClassBodyStart == 0 {
		return IndexResult{}, fmt.Errorf("could not locate class declaration in %s", relPath)
	}

	classLine := findClassLine(content)
	if classLine < 0 {
		return IndexResult{}, fmt.Errorf("could not find class keyword in %s", relPath)
	}

	updated := insertBeforeLine(content, classLine, attrLine)

	if err := os.WriteFile(absPath, []byte(updated), 0644); err != nil {
		return IndexResult{}, fmt.Errorf("writing entity: %w", err)
	}

	msgs := []string{
		fmt.Sprintf("Added index %q to %s", indexName, relPath),
		fmt.Sprintf("Fields: %s", strings.Join(p.Fields, ", ")),
		fmt.Sprintf("Index name format: <table>_<fields>_idx = %q", indexName),
		"",
		"Remember to run: php bin/console doctrine:migrations:diff",
	}

	return IndexResult{
		EntityFile: relPath,
		IndexName:  indexName,
		Messages:   msgs,
	}, nil
}

func findClassLine(content string) int {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "class ") ||
			strings.HasPrefix(trimmed, "abstract class ") ||
			strings.HasPrefix(trimmed, "final class ") ||
			strings.HasPrefix(trimmed, "readonly class ") {
			return i
		}
	}
	return -1
}
