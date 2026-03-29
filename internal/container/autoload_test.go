package container_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sylius-code-mcp/internal/container"
)

func TestLoadPSR4_singlePath(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "composer.json"), `{
		"autoload": { "psr-4": { "App\\": "src/" } }
	}`)

	psr4, err := container.LoadPSR4(root)
	if err != nil {
		t.Fatalf("LoadPSR4: %v", err)
	}
	if got, ok := psr4["App\\"]; !ok || len(got) != 1 || got[0] != "src/" {
		t.Errorf("expected App\\ → [src/], got %v", psr4["App\\"])
	}
}

func TestLoadPSR4_arrayPath(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "composer.json"), `{
		"autoload": { "psr-4": { "App\\": ["src/", "lib/"] } }
	}`)

	psr4, err := container.LoadPSR4(root)
	if err != nil {
		t.Fatalf("LoadPSR4: %v", err)
	}
	dirs := psr4["App\\"]
	if len(dirs) != 2 {
		t.Errorf("expected 2 dirs, got %v", dirs)
	}
}

func TestLoadPSR4_missingFile(t *testing.T) {
	_, err := container.LoadPSR4(t.TempDir())
	if err == nil {
		t.Error("expected error for missing composer.json")
	}
}

func TestLoadPSR4_includesVendorMap(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "composer.json"),
		`{"autoload":{"psr-4":{"App\\":"src/"}}}`)

	vendorPSR4 := `<?php
$vendorDir = dirname(__DIR__);
$baseDir = dirname($vendorDir);
return array(
    'Sylius\\Bundle\\' => array($vendorDir . '/sylius/sylius/src/Sylius/Bundle'),
    'Knp\\Menu\\' => array($vendorDir . '/knplabs/knp-menu/src/Knp/Menu'),
);
`
	writeFile(t, filepath.Join(root, "vendor", "composer", "autoload_psr4.php"), vendorPSR4)

	psr4, err := container.LoadPSR4(root)
	if err != nil {
		t.Fatalf("LoadPSR4: %v", err)
	}

	if _, ok := psr4["App\\"]; !ok {
		t.Error("expected App\\ from composer.json")
	}

	syliusDirs, ok := psr4["Sylius\\Bundle\\"]
	if !ok {
		t.Fatal("expected Sylius\\Bundle\\ from vendor autoload_psr4.php")
	}
	if len(syliusDirs) == 0 {
		t.Fatal("expected at least one dir for Sylius\\Bundle\\")
	}
	if syliusDirs[0] != filepath.Join("vendor", "sylius", "sylius", "src", "Sylius", "Bundle") {
		t.Errorf("unexpected path: %q", syliusDirs[0])
	}
}

func TestLoadPSR4_vendorMissingIsNonFatal(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "composer.json"),
		`{"autoload":{"psr-4":{"App\\":"src/"}}}`)

	psr4, err := container.LoadPSR4(root)
	if err != nil {
		t.Fatalf("LoadPSR4 should not fail when vendor is absent: %v", err)
	}
	if _, ok := psr4["App\\"]; !ok {
		t.Error("expected App\\ from composer.json")
	}
}

func TestResolveClass_vendorClass(t *testing.T) {
	root := t.TempDir()

	dir := filepath.Join(root, "vendor", "sylius", "sylius", "src", "Sylius", "Bundle", "AdminBundle", "Menu")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "MainMenuBuilder.php"), "<?php")

	psr4 := container.PSR4Map{
		"Sylius\\Bundle\\": []string{filepath.Join("vendor", "sylius", "sylius", "src", "Sylius", "Bundle")},
	}

	path, ok := container.ResolveClass(`Sylius\Bundle\AdminBundle\Menu\MainMenuBuilder`, psr4, root)
	if !ok {
		t.Fatal("expected to resolve MainMenuBuilder")
	}
	expected := filepath.Join(dir, "MainMenuBuilder.php")
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}
}

func TestResolveClass(t *testing.T) {
	root := t.TempDir()

	entityDir := filepath.Join(root, "src", "Entity")
	if err := os.MkdirAll(entityDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(entityDir, "Product.php"), "<?php")

	psr4 := container.PSR4Map{
		"App\\": []string{"src/"},
	}

	path, ok := container.ResolveClass(`App\Entity\Product`, psr4, root)
	if !ok {
		t.Fatal("expected to resolve App\\Entity\\Product")
	}
	expected := filepath.Join(root, "src", "Entity", "Product.php")
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}
}

func TestResolveClass_notFound(t *testing.T) {
	psr4 := container.PSR4Map{"App\\": []string{"src/"}}
	_, ok := container.ResolveClass(`App\Missing\Class`, psr4, t.TempDir())
	if ok {
		t.Error("expected not found")
	}
}

func TestResolveClass_leadingBackslash(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "src")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "Foo.php"), "<?php")

	psr4 := container.PSR4Map{"App\\": []string{"src/"}}
	_, ok := container.ResolveClass(`\App\Foo`, psr4, root)
	if !ok {
		t.Error("expected to handle leading backslash")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}
