package syliusconstraint

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sylius-code-mcp/internal/container"
	"github.com/sylius-code-mcp/internal/phpparser"
)

type ConstraintParam struct {
	Name         string
	PHPType      string
	Required     bool
	DefaultValue string
}

type ConstraintInfo struct {
	ShortName string
	FQCN      string
	IsBuiltin bool
	Params    []ConstraintParam
}

func ListConstraints(projectRoot string) ([]ConstraintInfo, error) {
	psr4, err := container.LoadPSR4(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("loading PSR-4 map: %w", err)
	}

	cont, contErr := container.Load(projectRoot)

	var infos []ConstraintInfo
	seen := make(map[string]bool)

	if contErr == nil {
		for _, svc := range cont.ServicesWithTag("validator.constraint_validator") {
			cls := svc.Class
			if cls == "" {
				cls = svc.ID
			}
			if !strings.HasSuffix(cls, "Validator") {
				continue
			}
			constraintFQCN := strings.TrimSuffix(cls, "Validator")
			if seen[constraintFQCN] {
				continue
			}
			seen[constraintFQCN] = true

			info := parseConstraintClass(constraintFQCN, psr4, projectRoot)
			if info == nil {
				continue
			}
			infos = append(infos, *info)
		}
	}

	symDir := filepath.Join(projectRoot, "vendor", "symfony", "validator", "Constraints")
	if entries, err := os.ReadDir(symDir); err == nil {
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".php") || strings.HasSuffix(name, "Validator.php") {
				continue
			}
			shortName := strings.TrimSuffix(name, ".php")
			fqcn := `Symfony\Component\Validator\Constraints\` + shortName
			if seen[fqcn] {
				continue
			}
			seen[fqcn] = true

			info := parseConstraintClass(fqcn, psr4, projectRoot)
			if info == nil {
				continue
			}
			infos = append(infos, *info)
		}
	}

	sort.Slice(infos, func(i, j int) bool {
		if infos[i].IsBuiltin != infos[j].IsBuiltin {
			return infos[i].IsBuiltin
		}
		return infos[i].ShortName < infos[j].ShortName
	})

	return infos, nil
}

func parseConstraintClass(fqcn string, psr4 container.PSR4Map, projectRoot string) *ConstraintInfo {
	filePath, ok := container.ResolveClass(fqcn, psr4, projectRoot)
	if !ok {
		return nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}
	content := string(data)

	if strings.Contains(content, "abstract class ") {
		return nil
	}

	parts := strings.Split(fqcn, `\`)
	shortName := parts[len(parts)-1]

	if strings.HasSuffix(shortName, "Validator") {
		return nil
	}

	if !strings.Contains(content, "class "+shortName) {
		return nil
	}

	rawParams := phpparser.ParseConstructorParams(content)
	if rawParams == nil {
		parentShort := phpparser.ParseClassExtends(content)
		if parentShort != "" {
			parentNS := strings.Join(parts[:len(parts)-1], `\`)
			parentFQCN := parentNS + `\` + parentShort
			if pFile, ok := container.ResolveClass(parentFQCN, psr4, projectRoot); ok {
				if pData, err := os.ReadFile(pFile); err == nil {
					rawParams = phpparser.ParseConstructorParams(string(pData))
				}
			}
		}
	}

	params := make([]ConstraintParam, len(rawParams))
	for i, p := range rawParams {
		params[i] = ConstraintParam{
			Name:         p.Name,
			PHPType:      p.PHPType,
			Required:     p.Required,
			DefaultValue: p.DefaultValue,
		}
	}

	isBuiltin := strings.HasPrefix(fqcn, `Symfony\`)

	return &ConstraintInfo{
		ShortName: shortName,
		FQCN:      fqcn,
		IsBuiltin: isBuiltin,
		Params:    params,
	}
}

func FormatForLLM(infos []ConstraintInfo) string {
	var sb strings.Builder

	sb.WriteString("Available validator constraints:\n")
	sb.WriteString("(Use short names for Symfony built-ins, FQCN for custom constraints)\n\n")

	builtinSep := false
	customSep := false

	for _, info := range infos {
		if info.IsBuiltin && !builtinSep {
			sb.WriteString("── Symfony built-in ──────────────────────────────────\n")
			builtinSep = true
		}
		if !info.IsBuiltin && !customSep {
			sb.WriteString("\n── Project-specific ──────────────────────────────────\n")
			customSep = true
		}

		usageName := info.ShortName
		if !info.IsBuiltin {
			usageName = info.FQCN
		}
		fmt.Fprintf(&sb, "\n%s\n", info.ShortName)
		if !info.IsBuiltin {
			fmt.Fprintf(&sb, "  FQCN: %s\n", info.FQCN)
		}

		if len(info.Params) == 0 {
			sb.WriteString("  params: (none)\n")
		} else {
			sb.WriteString("  params:\n")
			for _, p := range info.Params {
				req := "optional"
				if p.Required {
					req = "REQUIRED"
				}
				fmt.Fprintf(&sb, "    %-20s %s  %s\n", p.Name, req, p.PHPType)
			}
		}

		fmt.Fprintf(&sb, "  example: {\"name\": %q", usageName)
		var exampleParams []string
		for _, p := range info.Params {
			if p.Required {
				exampleParams = append(exampleParams, fmt.Sprintf("%q: ...", p.Name))
			}
		}
		if len(exampleParams) > 0 {
			fmt.Fprintf(&sb, ", \"params\": {%s}", strings.Join(exampleParams, ", "))
		}
		sb.WriteString("}\n")
	}

	sb.WriteString("\n──────────────────────────────────────────────────────\n")
	sb.WriteString("Use in sylius_add_entity_fields:\n")
	sb.WriteString(`  "constraints": [{"name": "NotBlank"}, {"name": "Length", "params": {"max": 255}}]`)
	sb.WriteString("\n")

	return sb.String()
}
