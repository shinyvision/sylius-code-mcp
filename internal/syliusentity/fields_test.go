package syliusentity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readTestdata(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading testdata %s: %v", name, err)
	}
	return string(data)
}

func TestRenderConstraintAttr_noParams(t *testing.T) {
	c := ConstraintInput{Name: "NotBlank"}
	got := renderConstraintAttr(c)
	if got != "    #[Assert\\NotBlank]\n" {
		t.Errorf("unexpected output: %q", got)
	}
}

func TestRenderConstraintAttr_withParams(t *testing.T) {
	c := ConstraintInput{Name: "Length", Params: map[string]any{"max": float64(255)}}
	got := renderConstraintAttr(c)
	if got != "    #[Assert\\Length(max: 255)]\n" {
		t.Errorf("unexpected output: %q", got)
	}
}

func TestRenderConstraintAttr_stringParam(t *testing.T) {
	c := ConstraintInput{Name: "Regex", Params: map[string]any{"pattern": "/^[a-z]+$/"}}
	got := renderConstraintAttr(c)
	if got != "    #[Assert\\Regex(pattern: '/^[a-z]+$/')]\n" {
		t.Errorf("unexpected output: %q", got)
	}
}

func TestGenerateFieldProperty_withConstraints(t *testing.T) {
	f := FieldInput{
		Name: "email",
		Type: "string",
		Constraints: []ConstraintInput{
			{Name: "NotBlank"},
			{Name: "Email"},
		},
	}
	code := generateFieldProperty(f)
	if !strings.Contains(code, "#[Assert\\NotBlank]") {
		t.Errorf("missing NotBlank constraint: %s", code)
	}
	if !strings.Contains(code, "#[Assert\\Email]") {
		t.Errorf("missing Email constraint: %s", code)
	}
	notBlankPos := strings.Index(code, "Assert\\NotBlank")
	ormPos := strings.Index(code, "ORM\\Column")
	if notBlankPos > ormPos {
		t.Errorf("constraints should appear before ORM\\Column attr: %s", code)
	}
}

func TestApplyFieldsToContent_constraintAddsAssertImport(t *testing.T) {
	content := readTestdata(t, "Supplier.php")

	p := FieldsParams{
		EntityClass: `App\Warehouse\Entity\Supplier`,
		Fields: []FieldInput{
			{Name: "email", Type: "string", Constraints: []ConstraintInput{{Name: "NotBlank"}}},
		},
	}
	updated, _, err := applyFieldsToContent(content, p)
	if err != nil {
		t.Fatalf("applyFieldsToContent: %v", err)
	}
	if !strings.Contains(updated, "use Symfony\\Component\\Validator\\Constraints as Assert") {
		t.Errorf("missing Assert import in output:\n%s", updated)
	}
	if !strings.Contains(updated, "#[Assert\\NotBlank]") {
		t.Errorf("missing NotBlank constraint in output:\n%s", updated)
	}
}

func TestCamelToSnake(t *testing.T) {
	cases := []struct{ in, want string }{
		{"name", "name"},
		{"createdAt", "created_at"},
		{"firstName", "first_name"},
		{"MyField", "my_field"},
		{"HTMLParser", "h_t_m_l_parser"},
	}
	for _, c := range cases {
		if got := camelToSnake(c.in); got != c.want {
			t.Errorf("camelToSnake(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSingularize(t *testing.T) {
	cases := []struct{ in, want string }{
		{"products", "product"},
		{"categories", "category"},
		{"addresses", "address"},
		{"children", "children"},
		{"status", "status"},
	}
	for _, c := range cases {
		if got := singularize(c.in); got != c.want {
			t.Errorf("singularize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestGenerateFieldProperty_string(t *testing.T) {
	f := FieldInput{Name: "description", Type: "string", Nullable: true}
	code := generateFieldProperty(f)
	if !strings.Contains(code, "ORM\\Column(name: 'description', nullable: true)") {
		t.Errorf("unexpected column attr: %s", code)
	}
	if !strings.Contains(code, "private ?string $description = null") {
		t.Errorf("unexpected property decl: %s", code)
	}
}

func TestGenerateFieldProperty_datetimeImmutable(t *testing.T) {
	f := FieldInput{Name: "createdAt", Type: "datetime_immutable", Nullable: false}
	code := generateFieldProperty(f)
	if !strings.Contains(code, "name: 'created_at'") {
		t.Errorf("expected snake_case column name in: %s", code)
	}
	if !strings.Contains(code, "type: 'datetime_immutable'") {
		t.Errorf("expected ORM type in: %s", code)
	}
	if !strings.Contains(code, `\DateTimeImmutable`) {
		t.Errorf("expected DateTimeImmutable PHP type in: %s", code)
	}
	if strings.Contains(code, "nullable") {
		t.Errorf("non-nullable field should not have nullable attr: %s", code)
	}
}

func TestGenerateFieldProperty_nolength(t *testing.T) {
	f := FieldInput{Name: "title", Type: "string"}
	code := generateFieldProperty(f)
	if strings.Contains(code, "length") {
		t.Errorf("length should never be added to column attr: %s", code)
	}
}

func TestGenerateFieldGetterSetter_bool(t *testing.T) {
	f := FieldInput{Name: "active", Type: "bool"}
	code := generateFieldGetterSetter(f)
	if !strings.Contains(code, "public function isActive()") {
		t.Errorf("expected isXxx() getter for bool: %s", code)
	}
	if !strings.Contains(code, "public function setActive(?bool $active)") {
		t.Errorf("expected setter: %s", code)
	}
}

func TestGenerateAssocProperty_manyToOne(t *testing.T) {
	a := AssociationInput{
		Kind:         "ManyToOne",
		Name:         "supplier",
		TargetEntity: `App\Warehouse\Entity\Supplier`,
		Nullable:     false,
		OnDelete:     "CASCADE",
	}
	code := generateAssocProperty(a)
	if !strings.Contains(code, "ORM\\ManyToOne(targetEntity: Supplier::class)") {
		t.Errorf("missing ManyToOne attr: %s", code)
	}
	if !strings.Contains(code, "ORM\\JoinColumn(nullable: false, onDelete: 'CASCADE')") {
		t.Errorf("missing JoinColumn attr: %s", code)
	}
	if !strings.Contains(code, "private ?Supplier $supplier = null") {
		t.Errorf("missing property decl: %s", code)
	}
}

func TestGenerateAssocProperty_oneToMany(t *testing.T) {
	a := AssociationInput{
		Kind:         "OneToMany",
		Name:         "products",
		TargetEntity: `App\Warehouse\Entity\Product`,
		MappedBy:     "supplier",
	}
	code := generateAssocProperty(a)
	if !strings.Contains(code, "ORM\\OneToMany(mappedBy: 'supplier', targetEntity: Product::class)") {
		t.Errorf("missing OneToMany attr: %s", code)
	}
	if !strings.Contains(code, "private Collection $products") {
		t.Errorf("missing Collection property: %s", code)
	}
}

func TestGenerateAssocProperty_manyToMany(t *testing.T) {
	a := AssociationInput{
		Kind:          "ManyToMany",
		Name:          "categories",
		TargetEntity:  `App\Entity\Category`,
		JoinTableName: "supplier_category",
	}
	code := generateAssocProperty(a)
	if !strings.Contains(code, "ORM\\ManyToMany(targetEntity: Category::class)") {
		t.Errorf("missing ManyToMany attr: %s", code)
	}
	if !strings.Contains(code, "ORM\\JoinTable(name: 'supplier_category')") {
		t.Errorf("missing JoinTable attr: %s", code)
	}
}

func TestGenerateCollectionMethods(t *testing.T) {
	code := generateCollectionMethods("products", "", "Product")
	if !strings.Contains(code, "public function getProducts(): Collection") {
		t.Errorf("missing getter: %s", code)
	}
	if !strings.Contains(code, "public function addProduct(Product $product): void") {
		t.Errorf("missing adder: %s", code)
	}
	if !strings.Contains(code, "public function removeProduct(Product $product): void") {
		t.Errorf("missing remover: %s", code)
	}
}

func TestApplyFieldsToContent_scalarField(t *testing.T) {
	content := readTestdata(t, "Supplier.php")

	p := FieldsParams{
		EntityClass: `App\Warehouse\Entity\Supplier`,
		Fields: []FieldInput{
			{Name: "description", Type: "text", Nullable: true},
		},
	}
	updated, msgs, err := applyFieldsToContent(content, p)
	if err != nil {
		t.Fatalf("applyFieldsToContent: %v", err)
	}
	if len(msgs) == 0 {
		t.Error("expected at least one message")
	}

	if !strings.Contains(updated, "ORM\\Column(name: 'description', type: 'text', nullable: true)") {
		t.Errorf("missing column attr in output:\n%s", updated)
	}
	if !strings.Contains(updated, "private ?string $description = null") {
		t.Errorf("missing property decl in output:\n%s", updated)
	}
	if !strings.Contains(updated, "public function getDescription()") {
		t.Errorf("missing getter in output:\n%s", updated)
	}
	if !strings.Contains(updated, "public function setDescription(") {
		t.Errorf("missing setter in output:\n%s", updated)
	}
}

func TestApplyFieldsToContent_oneToManyCreatesConstructor(t *testing.T) {
	content := readTestdata(t, "Supplier.php")

	p := FieldsParams{
		EntityClass: `App\Warehouse\Entity\Supplier`,
		Associations: []AssociationInput{
			{
				Kind:         "OneToMany",
				Name:         "products",
				TargetEntity: `App\Warehouse\Entity\Product`,
				MappedBy:     "supplier",
			},
		},
	}
	updated, _, err := applyFieldsToContent(content, p)
	if err != nil {
		t.Fatalf("applyFieldsToContent: %v", err)
	}

	if !strings.Contains(updated, "new ArrayCollection()") {
		t.Errorf("missing ArrayCollection init in output:\n%s", updated)
	}
	if !strings.Contains(updated, "use Doctrine\\Common\\Collections\\ArrayCollection") {
		t.Errorf("missing ArrayCollection import in output:\n%s", updated)
	}
	if !strings.Contains(updated, "use Doctrine\\Common\\Collections\\Collection") {
		t.Errorf("missing Collection import in output:\n%s", updated)
	}
	if !strings.Contains(updated, "public function __construct()") {
		t.Errorf("missing constructor in output:\n%s", updated)
	}
}

func TestApplyFieldsToContent_oneToManyExistingConstructor(t *testing.T) {
	content := readTestdata(t, "SupplierWithCollections.php")

	p := FieldsParams{
		EntityClass: `App\Warehouse\Entity\Supplier`,
		Associations: []AssociationInput{
			{
				Kind:         "OneToMany",
				Name:         "orders",
				TargetEntity: `App\Entity\Order`,
				MappedBy:     "supplier",
			},
		},
	}
	updated, _, err := applyFieldsToContent(content, p)
	if err != nil {
		t.Fatalf("applyFieldsToContent: %v", err)
	}

	count := strings.Count(updated, "public function __construct()")
	if count != 1 {
		t.Errorf("expected 1 constructor, got %d in:\n%s", count, updated)
	}
	if !strings.Contains(updated, "$this->orders = new ArrayCollection()") {
		t.Errorf("missing orders init in:\n%s", updated)
	}
}

func TestApplyFieldsToContent_useStatementDedup(t *testing.T) {
	content := readTestdata(t, "Supplier.php")

	p := FieldsParams{
		EntityClass: `App\Warehouse\Entity\Supplier`,
		Fields:      []FieldInput{{Name: "code", Type: "string"}},
	}
	updated, _, err := applyFieldsToContent(content, p)
	if err != nil {
		t.Fatalf("applyFieldsToContent: %v", err)
	}

	count := strings.Count(updated, "use Doctrine\\ORM\\Mapping as ORM")
	if count != 1 {
		t.Errorf("expected exactly 1 ORM import, got %d in:\n%s", count, updated)
	}
}

func TestEnsureIndex_addsAttribute(t *testing.T) {
	dir := t.TempDir()

	entityDir := filepath.Join(dir, "src", "Warehouse", "Entity")
	_ = os.MkdirAll(entityDir, 0755)
	src := readTestdata(t, "Supplier.php")
	_ = os.WriteFile(filepath.Join(entityDir, "Supplier.php"), []byte(src), 0644)

	result, err := EnsureIndex(dir, IndexParams{
		EntityClass: `App\Warehouse\Entity\Supplier`,
		Fields:      []string{"name", "createdAt"},
	})
	if err != nil {
		t.Fatalf("EnsureIndex: %v", err)
	}

	if result.IndexName != "app_supplier_name_created_at_idx" {
		t.Errorf("unexpected index name: %q", result.IndexName)
	}

	updated, _ := os.ReadFile(filepath.Join(entityDir, "Supplier.php"))
	if !strings.Contains(string(updated), `#[ORM\Index(name: 'app_supplier_name_created_at_idx', fields: ['name', 'createdAt'])]`) {
		t.Errorf("missing index attribute in:\n%s", string(updated))
	}
}

func TestEnsureIndex_indexNameNoTableAnnotation(t *testing.T) {
	dir := t.TempDir()
	entityDir := filepath.Join(dir, "src", "Entity")
	_ = os.MkdirAll(entityDir, 0755)
	php := `<?php
namespace App\Entity;
use Doctrine\ORM\Mapping as ORM;
#[ORM\Entity]
class MyEntity {
    #[ORM\Id, ORM\GeneratedValue, ORM\Column]
    private ?int $id = null;
}
`
	_ = os.WriteFile(filepath.Join(entityDir, "MyEntity.php"), []byte(php), 0644)

	result, err := EnsureIndex(dir, IndexParams{
		EntityClass: `App\Entity\MyEntity`,
		Fields:      []string{"code"},
	})
	if err != nil {
		t.Fatalf("EnsureIndex: %v", err)
	}
	if result.IndexName != "my_entity_code_idx" {
		t.Errorf("unexpected index name: %q", result.IndexName)
	}
}
