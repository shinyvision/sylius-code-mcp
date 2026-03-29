package syliusgrid

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fakeEntity(props ...string) string {
	var sb strings.Builder
	sb.WriteString("<?php\nclass Foo {\n")
	for _, p := range props {
		sb.WriteString("    private string $" + p + ";\n")
	}
	sb.WriteString("}\n")
	return sb.String()
}

func TestDeriveGridName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"app.supplier", "app_supplier"},
		{"app.my_resource", "app_my_resource"},
		{"noprefix", "noprefix"},
	}
	for _, c := range cases {
		if got := deriveGridName(c.in); got != c.want {
			t.Errorf("deriveGridName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestResourceKey(t *testing.T) {
	cases := []struct{ in, want string }{
		{"app.supplier", "supplier"},
		{"app.my_resource", "my_resource"},
		{"noprefix", "noprefix"},
	}
	for _, c := range cases {
		if got := resourceKey(c.in); got != c.want {
			t.Errorf("resourceKey(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestClassNameFromFQCN(t *testing.T) {
	cases := []struct{ in, want string }{
		{`App\Warehouse\Entity\Supplier`, "Supplier"},
		{`App\Entity\Order`, "Order"},
		{"Supplier", "Supplier"},
	}
	for _, c := range cases {
		if got := classNameFromFQCN(c.in); got != c.want {
			t.Errorf("classNameFromFQCN(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestEntityRelPath(t *testing.T) {
	cases := []struct{ in, want string }{
		{`App\Warehouse\Entity\Supplier`, "src/Warehouse/Entity/Supplier.php"},
		{`App\Entity\Order`, "src/Entity/Order.php"},
	}
	for _, c := range cases {
		if got := entityRelPath(c.in); got != c.want {
			t.Errorf("entityRelPath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestValidateFields_entityNotFound(t *testing.T) {
	fields := []FieldSpec{{Name: "name", Type: "string"}, {Name: "foo", Type: "string"}}
	valid, skipped := validateFields("/nonexistent/path.php", fields)
	if len(valid) != 2 {
		t.Errorf("expected 2 valid fields, got %d", len(valid))
	}
	if len(skipped) != 0 {
		t.Errorf("expected 0 skipped fields, got %d", len(skipped))
	}
}

func TestValidateFields_entityFound(t *testing.T) {
	dir := t.TempDir()
	entityPath := filepath.Join(dir, "Supplier.php")
	_ = os.WriteFile(entityPath, []byte(fakeEntity("name", "code")), 0644)

	fields := []FieldSpec{
		{Name: "name", Type: "string"},
		{Name: "code", Type: "string"},
		{Name: "missing", Type: "string"},
	}
	valid, skipped := validateFields(entityPath, fields)
	if len(valid) != 2 {
		t.Errorf("expected 2 valid fields, got %d", len(valid))
	}
	if len(skipped) != 1 || skipped[0] != "missing" {
		t.Errorf("expected [missing] skipped, got %v", skipped)
	}
}

func TestValidateFields_dotNotation(t *testing.T) {
	dir := t.TempDir()
	entityPath := filepath.Join(dir, "Supplier.php")
	_ = os.WriteFile(entityPath, []byte(fakeEntity("category")), 0644)

	fields := []FieldSpec{{Name: "category.name", Type: "string"}}
	valid, skipped := validateFields(entityPath, fields)
	if len(valid) != 1 {
		t.Errorf("expected 1 valid field, got %d", len(valid))
	}
	if len(skipped) != 0 {
		t.Errorf("expected 0 skipped, got %v", skipped)
	}
}

func TestGenerateGridYAML_basic(t *testing.T) {
	fields := []FieldSpec{
		{Name: "name", Type: "string", Label: "sylius.ui.name", Sortable: true},
	}
	filters := []FilterSpec{
		{Name: "name", Type: "string", Label: "sylius.ui.name"},
	}
	yaml := generateGridYAML("app_supplier", `App\Warehouse\Entity\Supplier`, fields, filters)

	wantContains := []string{
		"sylius_grid:",
		"app_supplier:",
		"class: App\\Warehouse\\Entity\\Supplier",
		"name:",
		"type: string",
		"label: sylius.ui.name",
		"sortable: true",
		"type: create",
		"type: update",
		"type: delete",
	}
	for _, want := range wantContains {
		if !strings.Contains(yaml, want) {
			t.Errorf("generated YAML missing %q\nGot:\n%s", want, yaml)
		}
	}
}

func TestGenerateGridYAML_twigField(t *testing.T) {
	fields := []FieldSpec{
		{Name: "createdAt", Type: "twig"},
	}
	yaml := generateGridYAML("app_order", `App\Entity\Order`, fields, nil)

	if !strings.Contains(yaml, `template: "Admin/Order/createdAt.html.twig"`) {
		t.Errorf("missing twig template option in:\n%s", yaml)
	}
}

func TestGenerateGridYAML_twigFieldCustomTemplate(t *testing.T) {
	fields := []FieldSpec{
		{Name: "createdAt", Type: "twig", TwigTemplate: "Grid/Field/datetime.html.twig"},
	}
	yaml := generateGridYAML("app_order", `App\Entity\Order`, fields, nil)

	if !strings.Contains(yaml, `template: "Grid/Field/datetime.html.twig"`) {
		t.Errorf("missing custom twig template in:\n%s", yaml)
	}
}

func TestEnsureGrid_createsFiles(t *testing.T) {
	dir := t.TempDir()

	entityDir := filepath.Join(dir, "src", "Warehouse", "Entity")
	_ = os.MkdirAll(entityDir, 0755)
	_ = os.WriteFile(
		filepath.Join(entityDir, "Supplier.php"),
		[]byte(fakeEntity("name", "code")),
		0644,
	)

	p := Params{
		ResourceAlias: "app.supplier",
		EntityClass:   `App\Warehouse\Entity\Supplier`,
		Fields: []FieldSpec{
			{Name: "name", Type: "string", Sortable: true},
			{Name: "code", Type: "twig"},
		},
		Filters: []FilterSpec{
			{Name: "name", Type: "string"},
		},
	}

	result, err := EnsureGrid(dir, p)
	if err != nil {
		t.Fatalf("EnsureGrid error: %v", err)
	}

	gridPath := filepath.Join(dir, result.GridPath)
	if _, err := os.Stat(gridPath); err != nil {
		t.Errorf("grid YAML not created: %v", err)
	}

	content, _ := os.ReadFile(gridPath)
	if !strings.Contains(string(content), "app_supplier:") {
		t.Errorf("grid name missing in YAML")
	}

	if len(result.TwigTemplates) != 1 {
		t.Errorf("expected 1 twig template created, got %d", len(result.TwigTemplates))
	}
	twigPath := filepath.Join(dir, result.TwigTemplates[0])
	if _, err := os.Stat(twigPath); err != nil {
		t.Errorf("twig template not created: %v", err)
	}
}

func TestEnsureGrid_skipsUnknownFields(t *testing.T) {
	dir := t.TempDir()

	entityDir := filepath.Join(dir, "src", "Warehouse", "Entity")
	_ = os.MkdirAll(entityDir, 0755)
	_ = os.WriteFile(
		filepath.Join(entityDir, "Supplier.php"),
		[]byte(fakeEntity("name")),
		0644,
	)

	p := Params{
		ResourceAlias: "app.supplier",
		EntityClass:   `App\Warehouse\Entity\Supplier`,
		Fields: []FieldSpec{
			{Name: "name", Type: "string"},
			{Name: "nonexistent", Type: "string"},
		},
	}

	result, err := EnsureGrid(dir, p)
	if err != nil {
		t.Fatalf("EnsureGrid error: %v", err)
	}

	if len(result.SkippedFields) != 1 || result.SkippedFields[0] != "nonexistent" {
		t.Errorf("expected [nonexistent] skipped, got %v", result.SkippedFields)
	}

	content, _ := os.ReadFile(filepath.Join(dir, result.GridPath))
	if strings.Contains(string(content), "nonexistent") {
		t.Errorf("skipped field 'nonexistent' appeared in generated YAML")
	}
}
