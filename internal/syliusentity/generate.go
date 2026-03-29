package syliusentity

import (
	"fmt"
	"sort"
	"strings"
)

type phpTypeInfo struct {
	phpType string
	ormType string
}

var phpTypes = map[string]phpTypeInfo{
	"string":             {"string", ""},
	"text":               {"string", "text"},
	"int":                {"int", ""},
	"integer":            {"int", ""},
	"float":              {"float", ""},
	"bool":               {"bool", ""},
	"boolean":            {"bool", ""},
	"datetime":           {`\DateTime`, "datetime"},
	"datetime_immutable": {`\DateTimeImmutable`, "datetime_immutable"},
	"date":               {`\DateTimeImmutable`, "date_immutable"},
	"date_immutable":     {`\DateTimeImmutable`, "date_immutable"},
	"json":               {"array", "json"},
	"array":              {"array", "json"},
	"decimal":            {"string", "decimal"},
	"uuid":               {"string", "uuid"},
}

func camelToSnake(s string) string {
	var out []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			if i > 0 {
				out = append(out, '_')
			}
			out = append(out, c+32)
		} else {
			out = append(out, c)
		}
	}
	return string(out)
}

func ucFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func singularize(s string) string {
	if strings.HasSuffix(s, "ies") {
		return s[:len(s)-3] + "y"
	}
	if strings.HasSuffix(s, "ses") || strings.HasSuffix(s, "xes") ||
		strings.HasSuffix(s, "zes") || strings.HasSuffix(s, "ches") ||
		strings.HasSuffix(s, "shes") {
		return s[:len(s)-2]
	}
	if strings.HasSuffix(s, "us") || strings.HasSuffix(s, "ss") || strings.HasSuffix(s, "is") {
		return s
	}
	if strings.HasSuffix(s, "s") {
		return s[:len(s)-1]
	}
	return s
}

func classShortName(fqcn string) string {
	parts := strings.Split(fqcn, `\`)
	return parts[len(parts)-1]
}

func renderPHPValue(v any) string {
	switch val := v.(type) {
	case string:
		return "'" + strings.ReplaceAll(val, "'", `\'`) + "'"
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%g", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case nil:
		return "null"
	case []any:
		parts := make([]string, len(val))
		for i, item := range val {
			parts[i] = renderPHPValue(item)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	default:
		return fmt.Sprintf("%v", v)
	}
}

func renderConstraintAttr(c ConstraintInput) string {
	if len(c.Params) == 0 {
		return fmt.Sprintf("    #[Assert\\%s]\n", c.Name)
	}

	keys := make([]string, 0, len(c.Params))
	for k := range c.Params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s: %s", k, renderPHPValue(c.Params[k])))
	}
	return fmt.Sprintf("    #[Assert\\%s(%s)]\n", c.Name, strings.Join(parts, ", "))
}

func renderConstraintAttrs(constraints []ConstraintInput) string {
	var sb strings.Builder
	for _, c := range constraints {
		sb.WriteString(renderConstraintAttr(c))
	}
	return sb.String()
}

func generateFieldProperty(f FieldInput) string {
	ti, ok := phpTypes[strings.ToLower(f.Type)]
	if !ok {
		ti = phpTypeInfo{phpType: f.Type}
	}

	colAttrs := buildColumnAttrs(f.Name, ti.ormType, f.Nullable)

	var sb strings.Builder
	sb.WriteString(renderConstraintAttrs(f.Constraints))
	fmt.Fprintf(&sb, "    #[ORM\\Column(%s)]\n", colAttrs)
	fmt.Fprintf(&sb, "    private ?%s $%s = null;\n", ti.phpType, f.Name)
	return sb.String()
}

func buildColumnAttrs(fieldName, ormType string, nullable bool) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("name: '%s'", camelToSnake(fieldName)))
	if ormType != "" {
		parts = append(parts, fmt.Sprintf("type: '%s'", ormType))
	}
	if nullable {
		parts = append(parts, "nullable: true")
	}
	return strings.Join(parts, ", ")
}

func generateFieldGetterSetter(f FieldInput) string {
	ti, ok := phpTypes[strings.ToLower(f.Type)]
	if !ok {
		ti = phpTypeInfo{f.Type, ""}
	}

	phpType := ti.phpType
	methodName := ucFirst(f.Name)

	getterPrefix := "get"
	if phpType == "bool" {
		getterPrefix = "is"
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "    public function %s%s(): ?%s\n", getterPrefix, methodName, phpType)
	sb.WriteString("    {\n")
	fmt.Fprintf(&sb, "        return $this->%s;\n", f.Name)
	sb.WriteString("    }\n")
	sb.WriteString("\n")
	fmt.Fprintf(&sb, "    public function set%s(?%s $%s): void\n", methodName, phpType, f.Name)
	sb.WriteString("    {\n")
	fmt.Fprintf(&sb, "        $this->%s = $%s;\n", f.Name, f.Name)
	sb.WriteString("    }\n")
	return sb.String()
}

func generateAssocProperty(a AssociationInput) string {
	shortName := classShortName(a.TargetEntity)
	isCollection := a.Kind == "OneToMany" || a.Kind == "ManyToMany"

	var sb strings.Builder

	switch a.Kind {
	case "ManyToOne":
		attrs := buildManyToOneAttrs(a, shortName)
		for _, attr := range attrs {
			fmt.Fprintf(&sb, "    #[%s]\n", attr)
		}
		sb.WriteString(renderConstraintAttrs(a.Constraints))
		fmt.Fprintf(&sb, "    private ?%s $%s = null;\n", shortName, a.Name)

	case "OneToMany":
		attr := buildOneToManyAttr(a, shortName)
		fmt.Fprintf(&sb, "    #[%s]\n", attr)
		sb.WriteString(renderConstraintAttrs(a.Constraints))
		fmt.Fprintf(&sb, "    private Collection $%s;\n", a.Name)

	case "ManyToMany":
		attrs := buildManyToManyAttrs(a, shortName)
		for _, attr := range attrs {
			fmt.Fprintf(&sb, "    #[%s]\n", attr)
		}
		sb.WriteString(renderConstraintAttrs(a.Constraints))
		fmt.Fprintf(&sb, "    private Collection $%s;\n", a.Name)

	case "OneToOne":
		attrs := buildOneToOneAttrs(a, shortName)
		for _, attr := range attrs {
			fmt.Fprintf(&sb, "    #[%s]\n", attr)
		}
		sb.WriteString(renderConstraintAttrs(a.Constraints))
		if isCollection {
			fmt.Fprintf(&sb, "    private Collection $%s;\n", a.Name)
		} else {
			fmt.Fprintf(&sb, "    private ?%s $%s = null;\n", shortName, a.Name)
		}
	}

	return sb.String()
}

func buildManyToOneAttrs(a AssociationInput, shortName string) []string {
	var mtoArgs []string
	mtoArgs = append(mtoArgs, fmt.Sprintf("targetEntity: %s::class", shortName))
	if a.InversedBy != "" {
		mtoArgs = append(mtoArgs, fmt.Sprintf("inversedBy: '%s'", a.InversedBy))
	}

	var jcArgs []string
	if a.JoinColumnName != "" {
		jcArgs = append(jcArgs, fmt.Sprintf("name: '%s'", a.JoinColumnName))
	}
	if a.Nullable {
		jcArgs = append(jcArgs, "nullable: true")
	} else {
		jcArgs = append(jcArgs, "nullable: false")
	}
	if a.OnDelete != "" {
		jcArgs = append(jcArgs, fmt.Sprintf("onDelete: '%s'", a.OnDelete))
	}

	return []string{
		fmt.Sprintf("ORM\\ManyToOne(%s)", strings.Join(mtoArgs, ", ")),
		fmt.Sprintf("ORM\\JoinColumn(%s)", strings.Join(jcArgs, ", ")),
	}
}

func buildOneToManyAttr(a AssociationInput, shortName string) string {
	var args []string
	if a.MappedBy != "" {
		args = append(args, fmt.Sprintf("mappedBy: '%s'", a.MappedBy))
	}
	args = append(args, fmt.Sprintf("targetEntity: %s::class", shortName))
	if a.OrphanRemoval {
		args = append(args, "orphanRemoval: true")
	}
	return fmt.Sprintf("ORM\\OneToMany(%s)", strings.Join(args, ", "))
}

func buildManyToManyAttrs(a AssociationInput, shortName string) []string {
	var mmArgs []string
	mmArgs = append(mmArgs, fmt.Sprintf("targetEntity: %s::class", shortName))
	if a.MappedBy != "" {
		mmArgs = append(mmArgs, fmt.Sprintf("mappedBy: '%s'", a.MappedBy))
	} else if a.InversedBy != "" {
		mmArgs = append(mmArgs, fmt.Sprintf("inversedBy: '%s'", a.InversedBy))
	}

	result := []string{fmt.Sprintf("ORM\\ManyToMany(%s)", strings.Join(mmArgs, ", "))}

	if a.MappedBy == "" {
		jtArgs := ""
		if a.JoinTableName != "" {
			jtArgs = fmt.Sprintf("name: '%s'", a.JoinTableName)
		}
		if jtArgs != "" {
			result = append(result, fmt.Sprintf("ORM\\JoinTable(%s)", jtArgs))
		} else {
			result = append(result, "ORM\\JoinTable")
		}
	}

	return result
}

func buildOneToOneAttrs(a AssociationInput, shortName string) []string {
	var ooArgs []string
	ooArgs = append(ooArgs, fmt.Sprintf("targetEntity: %s::class", shortName))
	if a.MappedBy != "" {
		ooArgs = append(ooArgs, fmt.Sprintf("mappedBy: '%s'", a.MappedBy))
	} else if a.InversedBy != "" {
		ooArgs = append(ooArgs, fmt.Sprintf("inversedBy: '%s'", a.InversedBy))
	}

	result := []string{fmt.Sprintf("ORM\\OneToOne(%s)", strings.Join(ooArgs, ", "))}

	if a.MappedBy == "" {
		var jcArgs []string
		if a.JoinColumnName != "" {
			jcArgs = append(jcArgs, fmt.Sprintf("name: '%s'", a.JoinColumnName))
		}
		if a.Nullable {
			jcArgs = append(jcArgs, "nullable: true")
		} else {
			jcArgs = append(jcArgs, "nullable: false")
		}
		if a.OnDelete != "" {
			jcArgs = append(jcArgs, fmt.Sprintf("onDelete: '%s'", a.OnDelete))
		}
		result = append(result, fmt.Sprintf("ORM\\JoinColumn(%s)", strings.Join(jcArgs, ", ")))
	}

	return result
}

func generateAssocMethods(a AssociationInput) string {
	shortName := classShortName(a.TargetEntity)
	isCollection := a.Kind == "OneToMany" || a.Kind == "ManyToMany"

	if isCollection {
		return generateCollectionMethods(a.Name, a.SingularName, shortName)
	}
	return generateSingleAssocMethods(a.Name, shortName)
}

func generateCollectionMethods(fieldName, singularOverride, typeName string) string {
	singular := singularOverride
	if singular == "" {
		singular = singularize(fieldName)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "    public function get%s(): Collection\n", ucFirst(fieldName))
	sb.WriteString("    {\n")
	fmt.Fprintf(&sb, "        return $this->%s;\n", fieldName)
	sb.WriteString("    }\n")
	sb.WriteString("\n")
	fmt.Fprintf(&sb, "    public function add%s(%s $%s): void\n", ucFirst(singular), typeName, singular)
	sb.WriteString("    {\n")
	fmt.Fprintf(&sb, "        if (!$this->%s->contains($%s)) {\n", fieldName, singular)
	fmt.Fprintf(&sb, "            $this->%s->add($%s);\n", fieldName, singular)
	sb.WriteString("        }\n")
	sb.WriteString("    }\n")
	sb.WriteString("\n")
	fmt.Fprintf(&sb, "    public function remove%s(%s $%s): void\n", ucFirst(singular), typeName, singular)
	sb.WriteString("    {\n")
	fmt.Fprintf(&sb, "        $this->%s->removeElement($%s);\n", fieldName, singular)
	sb.WriteString("    }\n")
	return sb.String()
}

func generateSingleAssocMethods(fieldName, typeName string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "    public function get%s(): ?%s\n", ucFirst(fieldName), typeName)
	sb.WriteString("    {\n")
	fmt.Fprintf(&sb, "        return $this->%s;\n", fieldName)
	sb.WriteString("    }\n")
	sb.WriteString("\n")
	fmt.Fprintf(&sb, "    public function set%s(?%s $%s): void\n", ucFirst(fieldName), typeName, fieldName)
	sb.WriteString("    {\n")
	fmt.Fprintf(&sb, "        $this->%s = $%s;\n", fieldName, fieldName)
	sb.WriteString("    }\n")
	return sb.String()
}

func generateConstructorInit(fieldName string) string {
	return fmt.Sprintf("        $this->%s = new ArrayCollection();\n", fieldName)
}

func generateConstructor(collectionFields []string) string {
	var sb strings.Builder
	sb.WriteString("    public function __construct()\n")
	sb.WriteString("    {\n")
	for _, f := range collectionFields {
		fmt.Fprintf(&sb, "        $this->%s = new ArrayCollection();\n", f)
	}
	sb.WriteString("    }\n")
	return sb.String()
}

func inverseAssocProperty(a AssociationInput, thisEntityFQCN string) string {
	thisShort := classShortName(thisEntityFQCN)
	fieldName := a.InverseField
	if fieldName == "" {
		fieldName = a.Name
	}

	switch a.Kind {
	case "ManyToOne":
		inv := AssociationInput{
			Kind:         "OneToMany",
			Name:         fieldName,
			MappedBy:     a.Name,
			TargetEntity: thisEntityFQCN,
		}
		return generateAssocProperty(inv)

	case "OneToMany":
		inv := AssociationInput{
			Kind:         "ManyToOne",
			Name:         fieldName,
			TargetEntity: thisEntityFQCN,
			InversedBy:   a.Name,
			Nullable:     a.Nullable,
			OnDelete:     a.OnDelete,
		}
		return generateAssocProperty(inv)

	case "ManyToMany":
		inv := AssociationInput{
			Kind:         "ManyToMany",
			Name:         fieldName,
			MappedBy:     a.Name,
			TargetEntity: thisEntityFQCN,
		}
		return generateAssocProperty(inv)

	case "OneToOne":
		inv := AssociationInput{
			Kind:         "OneToOne",
			Name:         fieldName,
			MappedBy:     a.Name,
			TargetEntity: thisEntityFQCN,
		}
		return generateAssocProperty(inv)
	}

	_ = thisShort
	return ""
}

func inverseAssocMethods(a AssociationInput, thisEntityFQCN string) string {
	thisShort := classShortName(thisEntityFQCN)
	fieldName := a.InverseField
	if fieldName == "" {
		fieldName = a.Name
	}

	switch a.Kind {
	case "ManyToOne":
		return generateCollectionMethods(fieldName, "", thisShort)
	case "OneToMany":
		return generateSingleAssocMethods(fieldName, thisShort)
	case "ManyToMany":
		return generateCollectionMethods(fieldName, "", thisShort)
	case "OneToOne":
		return generateSingleAssocMethods(fieldName, thisShort)
	}
	return ""
}

func isCollectionKind(kind string) bool {
	return kind == "OneToMany" || kind == "ManyToMany"
}

func inverseIsCollection(kind string) bool {
	switch kind {
	case "ManyToOne":
		return true
	case "ManyToMany":
		return true
	}
	return false
}
