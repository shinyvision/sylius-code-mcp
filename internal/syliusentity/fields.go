package syliusentity

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sylius-code-mcp/internal/phpparser"
)

type ConstraintInput struct {
	Name   string         `json:"name"`
	Params map[string]any `json:"params,omitempty"`
}

type FieldInput struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Nullable    bool              `json:"nullable"`
	Constraints []ConstraintInput `json:"constraints,omitempty"`
}

type AssociationInput struct {
	Name            string            `json:"name"`
	Kind            string            `json:"kind"`
	TargetEntity    string            `json:"targetEntity"`
	Nullable        bool              `json:"nullable,omitempty"`
	OnDelete        string            `json:"onDelete,omitempty"`
	MappedBy        string            `json:"mappedBy,omitempty"`
	InversedBy      string            `json:"inversedBy,omitempty"`
	OrphanRemoval   bool              `json:"orphanRemoval,omitempty"`
	JoinColumnName  string            `json:"joinColumnName,omitempty"`
	JoinTableName   string            `json:"joinTableName,omitempty"`
	GenerateInverse bool              `json:"generateInverse,omitempty"`
	InverseField    string            `json:"inverseField,omitempty"`
	SingularName    string            `json:"singularName,omitempty"`
	Constraints     []ConstraintInput `json:"constraints,omitempty"`
}

type FieldsParams struct {
	EntityClass  string             `json:"entityClass"`
	Fields       []FieldInput       `json:"fields,omitempty"`
	Associations []AssociationInput `json:"associations,omitempty"`
}

type FieldsResult struct {
	EntityFile string
	Messages   []string
}

func EnsureFields(projectRoot string, p FieldsParams) (FieldsResult, error) {
	if p.EntityClass == "" {
		return FieldsResult{}, fmt.Errorf("entityClass is required")
	}
	relPath := entityRelPath(p.EntityClass)
	absPath := filepath.Join(projectRoot, relPath)

	content, err := os.ReadFile(absPath)
	if err != nil {
		return FieldsResult{}, fmt.Errorf("reading entity %s: %w", relPath, err)
	}

	updated, msgs, err := applyFieldsToContent(string(content), p)
	if err != nil {
		return FieldsResult{}, err
	}

	if err := os.WriteFile(absPath, []byte(updated), 0644); err != nil {
		return FieldsResult{}, fmt.Errorf("writing entity %s: %w", relPath, err)
	}

	for _, a := range p.Associations {
		if !a.GenerateInverse {
			continue
		}
		targetRelPath := entityRelPath(a.TargetEntity)
		targetAbsPath := filepath.Join(projectRoot, targetRelPath)
		targetContent, err := os.ReadFile(targetAbsPath)
		if err != nil {
			msgs = append(msgs, fmt.Sprintf("WARNING: could not read target entity %s for inverse generation: %v", targetRelPath, err))
			continue
		}

		invFieldName := a.InverseField
		if invFieldName == "" {
			invFieldName = a.Name
		}

		invUpdated, invMsgs, err := applyInverseToContent(string(targetContent), a, p.EntityClass, invFieldName)
		if err != nil {
			msgs = append(msgs, fmt.Sprintf("WARNING: inverse generation failed for %s: %v", targetRelPath, err))
			continue
		}
		msgs = append(msgs, invMsgs...)

		if err := os.WriteFile(targetAbsPath, []byte(invUpdated), 0644); err != nil {
			msgs = append(msgs, fmt.Sprintf("WARNING: could not write target entity %s: %v", targetRelPath, err))
			continue
		}
		msgs = append(msgs, fmt.Sprintf("Modified inverse entity: %s", targetRelPath))
	}

	return FieldsResult{EntityFile: relPath, Messages: msgs}, nil
}

func applyFieldsToContent(content string, p FieldsParams) (string, []string, error) {
	var msgs []string
	req := ModifyRequest{}

	req.UseStatements = append(req.UseStatements, `Doctrine\ORM\Mapping as ORM`)

	for _, f := range p.Fields {
		req.Properties = append(req.Properties, generateFieldProperty(f))
		req.Methods = append(req.Methods, generateFieldGetterSetter(f))
		if len(f.Constraints) > 0 {
			req.UseStatements = append(req.UseStatements, `Symfony\Component\Validator\Constraints as Assert`)
		}
		msgs = append(msgs, fmt.Sprintf("Added field: %s (%s)", f.Name, f.Type))
	}

	info, _ := phpparser.ParseClassInfo(content)
	var collectionFields []string

	for _, a := range p.Associations {
		req.Properties = append(req.Properties, generateAssocProperty(a))
		req.Methods = append(req.Methods, generateAssocMethods(a))

		if len(a.Constraints) > 0 {
			req.UseStatements = append(req.UseStatements, `Symfony\Component\Validator\Constraints as Assert`)
		}

		if isCollectionKind(a.Kind) {
			collectionFields = append(collectionFields, a.Name)
			req.UseStatements = append(req.UseStatements,
				`Doctrine\Common\Collections\ArrayCollection`,
				`Doctrine\Common\Collections\Collection`,
			)
		}

		req.UseStatements = append(req.UseStatements, a.TargetEntity)
		msgs = append(msgs, fmt.Sprintf("Added association: %s %s → %s", a.Kind, a.Name, a.TargetEntity))
	}

	if len(collectionFields) > 0 {
		if info.HasConstructor {
			for _, f := range collectionFields {
				req.ConstructorInits = append(req.ConstructorInits, generateConstructorInit(f))
			}
		} else {
			req.Methods = append([]string{generateConstructor(collectionFields)}, req.Methods...)
		}
	}

	req.UseStatements = deduplicateStrings(req.UseStatements)

	updated, err := ApplyModifications(content, req)
	if err != nil {
		return "", nil, err
	}
	return updated, msgs, nil
}

func applyInverseToContent(content string, a AssociationInput, thisEntityFQCN, invFieldName string) (string, []string, error) {
	var msgs []string
	req := ModifyRequest{}

	req.UseStatements = append(req.UseStatements, `Doctrine\ORM\Mapping as ORM`)

	propCode := inverseAssocProperty(a, thisEntityFQCN)
	methodCode := inverseAssocMethods(a, thisEntityFQCN)

	req.Properties = append(req.Properties, propCode)
	req.Methods = append(req.Methods, methodCode)

	if inverseIsCollection(a.Kind) {
		req.UseStatements = append(req.UseStatements,
			`Doctrine\Common\Collections\ArrayCollection`,
			`Doctrine\Common\Collections\Collection`,
		)
	}
	req.UseStatements = append(req.UseStatements, thisEntityFQCN)
	req.UseStatements = deduplicateStrings(req.UseStatements)

	info, _ := phpparser.ParseClassInfo(content)
	if inverseIsCollection(a.Kind) {
		if info.HasConstructor {
			req.ConstructorInits = append(req.ConstructorInits, generateConstructorInit(invFieldName))
		} else {
			req.Methods = append([]string{generateConstructor([]string{invFieldName})}, req.Methods...)
		}
	}

	updated, err := ApplyModifications(content, req)
	if err != nil {
		return "", nil, err
	}
	msgs = append(msgs, fmt.Sprintf("Added inverse %s field %q to %s", a.Kind, invFieldName, thisEntityFQCN))
	return updated, msgs, nil
}

func deduplicateStrings(ss []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func entityRelPath(fqcn string) string {
	rel := strings.TrimPrefix(fqcn, `App\`)
	rel = strings.ReplaceAll(rel, `\`, "/")
	return "src/" + rel + ".php"
}

func buildFieldsResultMessages(r FieldsResult) []string {
	var lines []string
	lines = append(lines, fmt.Sprintf("Modified entity: %s", r.EntityFile))
	lines = append(lines, r.Messages...)
	lines = append(lines, "")
	lines = append(lines, "Remember to run: php bin/console doctrine:migrations:diff")
	return lines
}
