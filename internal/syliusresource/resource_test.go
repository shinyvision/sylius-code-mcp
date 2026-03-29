package syliusresource_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sylius-code-mcp/internal/syliusresource"
)

const minimalDoctrineYAML = `doctrine:
    dbal:
        url: '%env(resolve:DATABASE_URL)%'
    orm:
        auto_generate_proxy_classes: true
        entity_managers:
            default:
                mappings:
                    App:
                        is_bundle: false
                        type: attribute
                        dir: '%kernel.project_dir%/src/Entity'
                        prefix: 'App\Entity'
                        alias: App
`

const minimalSyliusResourceYAML = `sylius_resource:
    resources:
        sylius.product:
            classes:
                model: Sylius\Component\Product\Model\Product
`

func newTestProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	dirs := []string{
		filepath.Join(root, "config", "packages"),
		filepath.Join(root, "config", "routes", "_sylius"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("creating dir %s: %v", d, err)
		}
	}

	writeTestFile(t, filepath.Join(root, "config", "packages", "doctrine.yaml"), minimalDoctrineYAML)
	writeTestFile(t, filepath.Join(root, "config", "packages", "sylius_resource.yaml"), minimalSyliusResourceYAML)

	adminYAML := "app_existing:\n    resource: \"_sylius/existing.yaml\"\n    prefix: '/%sylius_admin.path_name%'\n"
	writeTestFile(t, filepath.Join(root, "config", "routes", "sylius_admin.yaml"), adminYAML)

	return root
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(data)
}

func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("expected to find %q in:\n%s", substr, s)
	}
}

func TestToPascalCase(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"my_resource", "MyResource"},
		{"payment_term", "PaymentTerm"},
		{"ticket_type_field", "TicketTypeField"},
		{"order", "Order"},
		{"product_variant_price", "ProductVariantPrice"},
	}
	for _, tc := range cases {
		got := syliusresource.ToPascalCase(tc.input)
		if got != tc.want {
			t.Errorf("ToPascalCase(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestDeriveMeta(t *testing.T) {
	p := syliusresource.Params{
		ResourceAlias: "app.my_resource",
		Namespace:     `App\MyResource`,
	}
	m := syliusresource.DeriveMeta(p)

	if m.AliasPrefix != "app" {
		t.Errorf("AliasPrefix = %q, want %q", m.AliasPrefix, "app")
	}
	if m.ResourceKey != "my_resource" {
		t.Errorf("ResourceKey = %q, want %q", m.ResourceKey, "my_resource")
	}
	if m.ClassName != "MyResource" {
		t.Errorf("ClassName = %q, want %q", m.ClassName, "MyResource")
	}
	if m.TableName != "app_my_resource" {
		t.Errorf("TableName = %q, want %q", m.TableName, "app_my_resource")
	}
	if m.EntityNamespace != `App\MyResource\Entity` {
		t.Errorf("EntityNamespace = %q, want %q", m.EntityNamespace, `App\MyResource\Entity`)
	}
	if m.RepoNamespace != `App\MyResource\Repository` {
		t.Errorf("RepoNamespace = %q, want %q", m.RepoNamespace, `App\MyResource\Repository`)
	}
	if m.EntityClass != "MyResource" {
		t.Errorf("EntityClass = %q, want %q", m.EntityClass, "MyResource")
	}
	if m.RepoClass != "MyResourceRepository" {
		t.Errorf("RepoClass = %q, want %q", m.RepoClass, "MyResourceRepository")
	}
	if m.EntityFQCN != `App\MyResource\Entity\MyResource` {
		t.Errorf("EntityFQCN = %q, want %q", m.EntityFQCN, `App\MyResource\Entity\MyResource`)
	}
	if m.RepoFQCN != `App\MyResource\Repository\MyResourceRepository` {
		t.Errorf("RepoFQCN = %q, want %q", m.RepoFQCN, `App\MyResource\Repository\MyResourceRepository`)
	}
	if m.EntityFilePath != "src/MyResource/Entity/MyResource.php" {
		t.Errorf("EntityFilePath = %q, want %q", m.EntityFilePath, "src/MyResource/Entity/MyResource.php")
	}
	if m.RepoFilePath != "src/MyResource/Repository/MyResourceRepository.php" {
		t.Errorf("RepoFilePath = %q, want %q", m.RepoFilePath, "src/MyResource/Repository/MyResourceRepository.php")
	}
	if m.DoctrineDir != "%kernel.project_dir%/src/MyResource/Entity" {
		t.Errorf("DoctrineDir = %q, want %q", m.DoctrineDir, "%kernel.project_dir%/src/MyResource/Entity")
	}
	if m.DoctrinePrefix != `App\MyResource\Entity` {
		t.Errorf("DoctrinePrefix = %q, want %q", m.DoctrinePrefix, `App\MyResource\Entity`)
	}
	if m.DoctrineAlias != `App\MyResource` {
		t.Errorf("DoctrineAlias = %q, want %q", m.DoctrineAlias, `App\MyResource`)
	}
	if m.GridName != "app_my_resource" {
		t.Errorf("GridName = %q, want %q", m.GridName, "app_my_resource")
	}
}

func TestEntityFileContent(t *testing.T) {
	root := newTestProject(t)
	p := syliusresource.Params{
		ResourceAlias: "app.my_resource",
		Namespace:     `App\MyResource`,
	}
	result, err := syliusresource.EnsureResource(root, p)
	if err != nil {
		t.Fatalf("EnsureResource: %v", err)
	}
	if !result.EntityCreated {
		t.Fatal("expected EntityCreated to be true")
	}

	meta := syliusresource.DeriveMeta(p)
	content := readTestFile(t, filepath.Join(root, meta.EntityFilePath))

	assertContains(t, content, "declare(strict_types=1);")
	assertContains(t, content, `namespace App\MyResource\Entity;`)
	assertContains(t, content, "use Doctrine\\ORM\\Mapping as ORM;")
	assertContains(t, content, "use Sylius\\Component\\Resource\\Model\\ResourceInterface;")
	assertContains(t, content, `use App\MyResource\Repository\MyResourceRepository;`)
	assertContains(t, content, "#[ORM\\Entity(repositoryClass: MyResourceRepository::class)]")
	assertContains(t, content, "#[ORM\\Table(name: 'app_my_resource')]")
	assertContains(t, content, "class MyResource implements ResourceInterface")
	assertContains(t, content, "private ?int $id = null;")
	assertContains(t, content, "public function getId(): ?int")
}

func TestRepositoryFileContent(t *testing.T) {
	root := newTestProject(t)
	p := syliusresource.Params{
		ResourceAlias: "app.my_resource",
		Namespace:     `App\MyResource`,
	}
	result, err := syliusresource.EnsureResource(root, p)
	if err != nil {
		t.Fatalf("EnsureResource: %v", err)
	}
	if !result.RepoCreated {
		t.Fatal("expected RepoCreated to be true")
	}

	meta := syliusresource.DeriveMeta(p)
	content := readTestFile(t, filepath.Join(root, meta.RepoFilePath))

	assertContains(t, content, "declare(strict_types=1);")
	assertContains(t, content, `namespace App\MyResource\Repository;`)
	assertContains(t, content, `use App\MyResource\Entity\MyResource;`)
	assertContains(t, content, "use Doctrine\\ORM\\EntityManager;")
	assertContains(t, content, "use Sylius\\Bundle\\ResourceBundle\\Doctrine\\ORM\\EntityRepository;")
	assertContains(t, content, "@method MyResource|null find(")
	assertContains(t, content, "@method MyResource|null findOneBy(")
	assertContains(t, content, "@method MyResource[] findAll()")
	assertContains(t, content, "class MyResourceRepository extends EntityRepository")
	assertContains(t, content, "public function __construct(EntityManager $entityManager, ClassMetadata $class)")
	assertContains(t, content, "parent::__construct($entityManager, $class);")
}

func TestEnsureResource_createsAll(t *testing.T) {
	root := newTestProject(t)
	p := syliusresource.Params{
		ResourceAlias: "app.my_resource",
		Namespace:     `App\MyResource`,
	}

	result, err := syliusresource.EnsureResource(root, p)
	if err != nil {
		t.Fatalf("EnsureResource: %v", err)
	}

	if !result.EntityCreated {
		t.Error("expected EntityCreated")
	}
	if !result.RepoCreated {
		t.Error("expected RepoCreated")
	}
	if !result.DoctrineAdded {
		t.Error("expected DoctrineAdded")
	}
	if !result.SyliusResAdded {
		t.Error("expected SyliusResAdded")
	}

	meta := syliusresource.DeriveMeta(p)

	if _, err := os.Stat(filepath.Join(root, meta.EntityFilePath)); err != nil {
		t.Errorf("entity file not found: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, meta.RepoFilePath)); err != nil {
		t.Errorf("repo file not found: %v", err)
	}

	docContent := readTestFile(t, filepath.Join(root, "config", "packages", "doctrine.yaml"))
	assertContains(t, docContent, `App\MyResource`)

	srContent := readTestFile(t, filepath.Join(root, "config", "packages", "sylius_resource.yaml"))
	assertContains(t, srContent, "app.my_resource")

	routePath := filepath.Join(root, "config", "routes", "_sylius", "my_resource.yaml")
	if _, err := os.Stat(routePath); err != nil {
		t.Errorf("route file not found: %v", err)
	}
}

func TestEnsureResource_idempotent(t *testing.T) {
	root := newTestProject(t)
	p := syliusresource.Params{
		ResourceAlias: "app.my_resource",
		Namespace:     `App\MyResource`,
	}

	if _, err := syliusresource.EnsureResource(root, p); err != nil {
		t.Fatalf("first EnsureResource: %v", err)
	}

	result, err := syliusresource.EnsureResource(root, p)
	if err != nil {
		t.Fatalf("second EnsureResource: %v", err)
	}

	if result.EntityCreated {
		t.Error("expected EntityCreated=false on second call")
	}
	if result.RepoCreated {
		t.Error("expected RepoCreated=false on second call")
	}
	if result.DoctrineAdded {
		t.Error("expected DoctrineAdded=false on second call")
	}
	if result.SyliusResAdded {
		t.Error("expected SyliusResAdded=false on second call")
	}
}

func TestEnsureDoctrineMapping_skipsExisting(t *testing.T) {
	root := newTestProject(t)

	existingDoctrineYAML := `doctrine:
    dbal:
        url: '%env(resolve:DATABASE_URL)%'
    orm:
        auto_generate_proxy_classes: true
        entity_managers:
            default:
                mappings:
                    App:
                        is_bundle: false
                        type: attribute
                        dir: '%kernel.project_dir%/src/Entity'
                        prefix: 'App\Entity'
                        alias: App
                    App\MyResource:
                        is_bundle: false
                        type: attribute
                        dir: '%kernel.project_dir%/src/MyResource/Entity'
                        prefix: 'App\MyResource\Entity'
                        alias: App\MyResource
`
	writeTestFile(t, filepath.Join(root, "config", "packages", "doctrine.yaml"), existingDoctrineYAML)

	p := syliusresource.Params{
		ResourceAlias: "app.my_resource",
		Namespace:     `App\MyResource`,
	}
	result, err := syliusresource.EnsureResource(root, p)
	if err != nil {
		t.Fatalf("EnsureResource: %v", err)
	}

	if result.DoctrineAdded {
		t.Error("expected DoctrineAdded=false when mapping already exists")
	}
}

func TestEnsureSyliusResource_skipsExisting(t *testing.T) {
	root := newTestProject(t)

	existingSyliusYAML := `sylius_resource:
    resources:
        sylius.product:
            classes:
                model: Sylius\Component\Product\Model\Product
        app.my_resource:
            classes:
                model: App\MyResource\Entity\MyResource
`
	writeTestFile(t, filepath.Join(root, "config", "packages", "sylius_resource.yaml"), existingSyliusYAML)

	p := syliusresource.Params{
		ResourceAlias: "app.my_resource",
		Namespace:     `App\MyResource`,
	}
	result, err := syliusresource.EnsureResource(root, p)
	if err != nil {
		t.Fatalf("EnsureResource: %v", err)
	}

	if result.SyliusResAdded {
		t.Error("expected SyliusResAdded=false when resource already registered")
	}
}

func TestEnsureResource_entityAlreadyExists(t *testing.T) {
	root := newTestProject(t)
	p := syliusresource.Params{
		ResourceAlias: "app.my_resource",
		Namespace:     `App\MyResource`,
	}
	meta := syliusresource.DeriveMeta(p)

	entityPath := filepath.Join(root, meta.EntityFilePath)
	if err := os.MkdirAll(filepath.Dir(entityPath), 0755); err != nil {
		t.Fatalf("creating entity dir: %v", err)
	}
	customContent := "<?php // custom content\n"
	writeTestFile(t, entityPath, customContent)

	result, err := syliusresource.EnsureResource(root, p)
	if err != nil {
		t.Fatalf("EnsureResource: %v", err)
	}

	if result.EntityCreated {
		t.Error("expected EntityCreated=false when entity already exists")
	}

	content := readTestFile(t, entityPath)
	if content != customContent {
		t.Errorf("entity file was overwritten; got %q", content)
	}
}

func TestValidate_missingDot(t *testing.T) {
	p := syliusresource.Params{ResourceAlias: "appmyresource", Namespace: `App\MyResource`}
	if err := syliusresource.Validate(p); err == nil {
		t.Error("expected error for alias without dot")
	}
}

func TestValidate_badNamespace(t *testing.T) {
	p := syliusresource.Params{ResourceAlias: "app.my_resource", Namespace: "MyResource"}
	if err := syliusresource.Validate(p); err == nil {
		t.Error("expected error for namespace not starting with App\\")
	}
}

func TestValidate_valid(t *testing.T) {
	p := syliusresource.Params{ResourceAlias: "app.my_resource", Namespace: `App\MyResource`}
	if err := syliusresource.Validate(p); err != nil {
		t.Errorf("unexpected validation error: %v", err)
	}
}
