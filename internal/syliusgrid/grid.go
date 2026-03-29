package syliusgrid

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sylius-code-mcp/internal/phpparser"
)

type FieldSpec struct {
	Name         string
	Type         string
	Label        string
	Sortable     bool
	TwigTemplate string
}

type FilterSpec struct {
	Name  string
	Type  string
	Label string
}

type Params struct {
	ResourceAlias string
	EntityClass   string
	Fields        []FieldSpec
	Filters       []FilterSpec
}

type Result struct {
	GridPath      string
	TwigTemplates []string
	SkippedFields []string
	Messages      []string
}

func EnsureGrid(projectRoot string, p Params) (Result, error) {
	if p.ResourceAlias == "" {
		return Result{}, fmt.Errorf("resourceAlias is required")
	}
	if p.EntityClass == "" {
		return Result{}, fmt.Errorf("entityClass is required")
	}

	gridName := deriveGridName(p.ResourceAlias)
	className := classNameFromFQCN(p.EntityClass)
	entityFilePath := filepath.Join(projectRoot, entityRelPath(p.EntityClass))

	validFields, skipped := validateFields(entityFilePath, p.Fields)

	gridYAML := generateGridYAML(gridName, p.EntityClass, validFields, p.Filters)
	gridRelPath := "config/packages/sylius_grid/" + resourceKey(p.ResourceAlias) + ".yaml"
	gridAbsPath := filepath.Join(projectRoot, gridRelPath)

	if err := os.MkdirAll(filepath.Dir(gridAbsPath), 0755); err != nil {
		return Result{}, fmt.Errorf("creating grid directory: %w", err)
	}
	existed := fileExists(gridAbsPath)
	if err := os.WriteFile(gridAbsPath, []byte(gridYAML), 0644); err != nil {
		return Result{}, fmt.Errorf("writing grid YAML: %w", err)
	}

	var twigPaths []string
	for _, f := range validFields {
		if f.Type != "twig" || f.TwigTemplate != "" {
			continue
		}
		templateRel := "Admin/" + className + "/" + f.Name + ".html.twig"
		templateAbs := filepath.Join(projectRoot, "templates", templateRel)
		if fileExists(templateAbs) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(templateAbs), 0755); err != nil {
			return Result{}, fmt.Errorf("creating twig dir: %w", err)
		}
		if err := os.WriteFile(templateAbs, []byte(generateTwigTemplate()), 0644); err != nil {
			return Result{}, fmt.Errorf("writing twig template: %w", err)
		}
		twigPaths = append(twigPaths, "templates/"+templateRel)
	}

	result := Result{
		GridPath:      gridRelPath,
		TwigTemplates: twigPaths,
		SkippedFields: skipped,
	}
	result.Messages = buildMessages(result, p, validFields, gridName, existed)
	return result, nil
}

func deriveGridName(alias string) string {
	return strings.ReplaceAll(alias, ".", "_")
}

func resourceKey(alias string) string {
	_, after, found := strings.Cut(alias, ".")
	if !found {
		return alias
	}
	return after
}

func classNameFromFQCN(fqcn string) string {
	parts := strings.Split(fqcn, `\`)
	return parts[len(parts)-1]
}

func entityRelPath(fqcn string) string {
	rel := strings.TrimPrefix(fqcn, `App\`)
	rel = strings.ReplaceAll(rel, `\`, "/")
	return "src/" + rel + ".php"
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func validateFields(entityFilePath string, fields []FieldSpec) (valid []FieldSpec, skipped []string) {
	data, err := os.ReadFile(entityFilePath)
	if err != nil {
		return fields, nil
	}

	props := phpparser.ParseEntityProperties(string(data))
	propSet := make(map[string]bool, len(props))
	for _, p := range props {
		propSet[p.Name] = true
	}

	for _, f := range fields {
		segment := strings.SplitN(f.Name, ".", 2)[0]
		if !propSet[segment] {
			skipped = append(skipped, f.Name)
			continue
		}
		valid = append(valid, f)
	}
	return
}

func generateGridYAML(gridName, entityClass string, fields []FieldSpec, filters []FilterSpec) string {
	var sb strings.Builder

	sb.WriteString("sylius_grid:\n")
	sb.WriteString("    grids:\n")
	fmt.Fprintf(&sb, "        %s:\n", gridName)
	sb.WriteString("            driver:\n")
	sb.WriteString("                name: doctrine/orm\n")
	sb.WriteString("                options:\n")
	fmt.Fprintf(&sb, "                    class: %s\n", entityClass)

	if len(fields) > 0 {
		sb.WriteString("            fields:\n")
		for _, f := range fields {
			writeFieldYAML(&sb, f, entityClass)
		}
	}

	if len(filters) > 0 {
		sb.WriteString("            filters:\n")
		for _, f := range filters {
			writeFilterYAML(&sb, f)
		}
	}

	sb.WriteString("            actions:\n")
	sb.WriteString("                main:\n")
	sb.WriteString("                    create:\n")
	sb.WriteString("                        type: create\n")
	sb.WriteString("                item:\n")
	sb.WriteString("                    update:\n")
	sb.WriteString("                        type: update\n")
	sb.WriteString("                    delete:\n")
	sb.WriteString("                        type: delete\n")

	return sb.String()
}

func writeFieldYAML(sb *strings.Builder, f FieldSpec, entityClass string) {
	label := f.Label
	if label == "" {
		label = "app.ui." + f.Name
	}

	fmt.Fprintf(sb, "                %s:\n", f.Name)
	fmt.Fprintf(sb, "                    type: %s\n", f.Type)
	fmt.Fprintf(sb, "                    label: %s\n", label)
	if f.Sortable {
		fmt.Fprintf(sb, "                    sortable: true\n")
	}
	if f.Type == "twig" {
		templatePath := f.TwigTemplate
		if templatePath == "" {
			className := classNameFromFQCN(entityClass)
			templatePath = "Admin/" + className + "/" + f.Name + ".html.twig"
		}
		fmt.Fprintf(sb, "                    options:\n")
		fmt.Fprintf(sb, "                        template: \"%s\"\n", templatePath)
	}
}

func writeFilterYAML(sb *strings.Builder, f FilterSpec) {
	label := f.Label
	if label == "" {
		label = "app.ui." + f.Name
	}

	fmt.Fprintf(sb, "                %s:\n", f.Name)
	fmt.Fprintf(sb, "                    type: %s\n", f.Type)
	fmt.Fprintf(sb, "                    label: %s\n", label)
}

func generateTwigTemplate() string {
	return "{{ data }}\n"
}

func buildMessages(r Result, p Params, validFields []FieldSpec, gridName string, existed bool) []string {
	var lines []string

	action := "Created"
	if existed {
		action = "Updated"
	}
	lines = append(lines, fmt.Sprintf("%s grid %q at %s.", action, gridName, r.GridPath))
	lines = append(lines, fmt.Sprintf("Entity class: %s", p.EntityClass))

	if len(validFields) > 0 {
		parts := make([]string, len(validFields))
		for i, f := range validFields {
			parts[i] = f.Name + " (" + f.Type + ")"
		}
		lines = append(lines, "Fields: "+strings.Join(parts, ", "))
	}

	if len(p.Filters) > 0 {
		parts := make([]string, len(p.Filters))
		for i, f := range p.Filters {
			parts[i] = f.Name + " (" + f.Type + ")"
		}
		lines = append(lines, "Filters: "+strings.Join(parts, ", "))
	}

	if len(r.SkippedFields) > 0 {
		lines = append(lines, fmt.Sprintf(
			"WARNING: Skipped %d field(s) not found in entity: %s",
			len(r.SkippedFields), strings.Join(r.SkippedFields, ", "),
		))
	}

	if len(r.TwigTemplates) > 0 {
		lines = append(lines, "Auto-created Twig templates:")
		for _, t := range r.TwigTemplates {
			lines = append(lines, "  - "+t)
		}
		lines = append(lines, "These templates receive the field value as {{ data }}. Customize as needed.")
	}

	lines = append(lines, "")
	lines = append(lines, "Standard CRUD actions included: create (main), update + delete (item).")
	lines = append(lines, "Ensure this grid name is referenced in your Sylius resource route configuration.")

	return lines
}
