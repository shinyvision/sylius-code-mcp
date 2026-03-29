package syliusroute_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sylius-code-mcp/internal/syliusroute"
)

func newTestProject(t *testing.T) (root string, cleanup func()) {
	t.Helper()
	root = t.TempDir()
	routesDir := filepath.Join(root, "config", "routes", "_sylius")
	if err := os.MkdirAll(routesDir, 0755); err != nil {
		t.Fatalf("creating routes dir: %v", err)
	}
	adminYAML := "app_existing:\n    resource: \"_sylius/existing.yaml\"\n    prefix: '/%sylius_admin.path_name%'\n"
	adminPath := filepath.Join(root, "config", "routes", "sylius_admin.yaml")
	if err := os.WriteFile(adminPath, []byte(adminYAML), 0644); err != nil {
		t.Fatalf("writing sylius_admin.yaml: %v", err)
	}
	return root, func() {}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(data)
}

func mustGenerateRouteYAML(t *testing.T, p syliusroute.Params) string {
	t.Helper()
	out, err := syliusroute.GenerateRouteYAML(p)
	if err != nil {
		t.Fatalf("GenerateRouteYAML: %v", err)
	}
	return out
}

func mustGenerateImportYAML(t *testing.T, resourceName string) string {
	t.Helper()
	out, err := syliusroute.GenerateImportYAML(resourceName)
	if err != nil {
		t.Fatalf("GenerateImportYAML: %v", err)
	}
	return out
}

func TestGenerateRouteYAML_minimal(t *testing.T) {
	p := syliusroute.Params{
		ResourceName: "my_resource",
		Alias:        "app.my_resource",
	}
	got := mustGenerateRouteYAML(t, p)

	assertContains(t, got, "app_admin_my_resource:")
	assertContains(t, got, "alias: app.my_resource")
	assertContains(t, got, "section: admin")
	assertContains(t, got, `templates: "@SyliusAdmin\\shared\\crud"`)
	assertContains(t, got, "permission: true")
	assertContains(t, got, "type: sylius.resource")

	assertNotContains(t, got, "grid:")
	assertNotContains(t, got, "except:")
	assertNotContains(t, got, "only:")
	assertNotContains(t, got, "redirect:")
}

func TestGenerateRouteYAML_withAllOptions(t *testing.T) {
	p := syliusroute.Params{
		ResourceName: "ticket",
		Alias:        "app.ticket",
		Grid:         "app_ticket",
		Except:       []string{"create", "update", "delete", "bulk_delete"},
		Redirect:     "show",
	}
	got := mustGenerateRouteYAML(t, p)

	assertContains(t, got, "app_admin_ticket:")
	assertContains(t, got, "grid: app_ticket")
	assertContains(t, got, "except: ['create', 'update', 'delete', 'bulk_delete']")
	assertContains(t, got, "redirect: show")
	assertNotContains(t, got, "only:")
}

func TestGenerateRouteYAML_withOnly(t *testing.T) {
	p := syliusroute.Params{
		ResourceName: "warehouse",
		Alias:        "app.warehouse",
		Only:         []string{"index", "show"},
	}
	got := mustGenerateRouteYAML(t, p)

	assertContains(t, got, "only: ['index', 'show']")
	assertNotContains(t, got, "except:")
}

func TestGenerateRouteYAML_resourceBlockIsLiteralScalar(t *testing.T) {
	p := syliusroute.Params{ResourceName: "foo", Alias: "app.foo"}
	got := mustGenerateRouteYAML(t, p)

	assertContains(t, got, "    resource: |\n")
}

func TestGenerateRouteYAML_fieldOrder(t *testing.T) {
	p := syliusroute.Params{
		ResourceName: "order",
		Alias:        "app.order",
		Grid:         "app_order",
		Except:       []string{"delete"},
		Redirect:     "update",
	}
	got := mustGenerateRouteYAML(t, p)

	positions := map[string]int{
		"alias:":      strings.Index(got, "alias:"),
		"section:":    strings.Index(got, "section:"),
		"templates:":  strings.Index(got, "templates:"),
		"except:":     strings.Index(got, "except:"),
		"redirect:":   strings.Index(got, "redirect:"),
		"permission:": strings.Index(got, "permission:"),
		"grid:":       strings.Index(got, "grid:"),
	}

	order := []string{"alias:", "section:", "templates:", "except:", "redirect:", "permission:", "grid:"}
	for i := 1; i < len(order); i++ {
		if positions[order[i-1]] >= positions[order[i]] {
			t.Errorf("field %q (pos %d) should appear before %q (pos %d)",
				order[i-1], positions[order[i-1]], order[i], positions[order[i]])
		}
	}
}

func TestGenerateImportYAML(t *testing.T) {
	got := mustGenerateImportYAML(t, "my_resource")

	assertContains(t, got, "app_my_resource:")
	assertContains(t, got, `resource: "_sylius/my_resource.yaml"`)
	assertContains(t, got, "prefix: '/%sylius_admin.path_name%'")
}

func TestValidate_missingResourceName(t *testing.T) {
	p := syliusroute.Params{Alias: "app.foo"}
	if err := p.Validate(); err == nil {
		t.Error("expected error for missing resourceName")
	}
}

func TestValidate_missingAlias(t *testing.T) {
	p := syliusroute.Params{ResourceName: "foo"}
	if err := p.Validate(); err == nil {
		t.Error("expected error for missing alias")
	}
}

func TestValidate_invalidExcept(t *testing.T) {
	p := syliusroute.Params{ResourceName: "foo", Alias: "app.foo", Except: []string{"unknown"}}
	if err := p.Validate(); err == nil {
		t.Error("expected error for invalid except value")
	}
}

func TestValidate_invalidOnly(t *testing.T) {
	p := syliusroute.Params{ResourceName: "foo", Alias: "app.foo", Only: []string{"bad"}}
	if err := p.Validate(); err == nil {
		t.Error("expected error for invalid only value")
	}
}

func TestValidate_invalidRedirect(t *testing.T) {
	p := syliusroute.Params{ResourceName: "foo", Alias: "app.foo", Redirect: "nowhere"}
	if err := p.Validate(); err == nil {
		t.Error("expected error for invalid redirect")
	}
}

func TestValidate_exceptAndOnlyMutuallyExclusive(t *testing.T) {
	p := syliusroute.Params{
		ResourceName: "foo",
		Alias:        "app.foo",
		Except:       []string{"show"},
		Only:         []string{"index"},
	}
	if err := p.Validate(); err == nil {
		t.Error("expected error when both except and only are provided")
	}
}

func TestValidate_valid(t *testing.T) {
	p := syliusroute.Params{
		ResourceName: "my_resource",
		Alias:        "app.my_resource",
		Grid:         "app_my_resource",
		Except:       []string{"show", "delete"},
		Redirect:     "update",
	}
	if err := p.Validate(); err != nil {
		t.Errorf("unexpected validation error: %v", err)
	}
}

func TestEnsureRoute_createsNewFile(t *testing.T) {
	root, _ := newTestProject(t)

	p := syliusroute.Params{
		ResourceName: "my_resource",
		Alias:        "app.my_resource",
		Grid:         "app_my_resource",
		Redirect:     "update",
	}

	result, err := syliusroute.EnsureRoute(root, p)
	if err != nil {
		t.Fatalf("EnsureRoute failed: %v", err)
	}

	if !result.FileCreated {
		t.Error("expected FileCreated to be true")
	}
	if result.FileUpdated {
		t.Error("expected FileUpdated to be false")
	}

	routePath := filepath.Join(root, "config", "routes", "_sylius", "my_resource.yaml")
	content := readFile(t, routePath)

	assertContains(t, content, "app_admin_my_resource:")
	assertContains(t, content, "alias: app.my_resource")
	assertContains(t, content, "grid: app_my_resource")
	assertContains(t, content, "redirect: update")
}

func TestEnsureRoute_returnsAlreadyExistsMessage(t *testing.T) {
	root, _ := newTestProject(t)

	p := syliusroute.Params{ResourceName: "my_resource", Alias: "app.my_resource"}
	if _, err := syliusroute.EnsureRoute(root, p); err != nil {
		t.Fatalf("first EnsureRoute failed: %v", err)
	}

	result, err := syliusroute.EnsureRoute(root, p)
	if err != nil {
		t.Fatalf("second EnsureRoute failed: %v", err)
	}

	if result.FileCreated || result.FileUpdated {
		t.Error("expected no file changes on duplicate")
	}
	if !strings.Contains(result.Message, "already exists") {
		t.Errorf("expected 'already exists' in message, got: %q", result.Message)
	}
}

func TestEnsureRoute_appendsToExistingFile(t *testing.T) {
	root, _ := newTestProject(t)

	existing, err := syliusroute.GenerateRouteYAML(syliusroute.Params{
		ResourceName: "ticket",
		Alias:        "app.ticket",
	})
	if err != nil {
		t.Fatalf("generating existing route YAML: %v", err)
	}
	routePath := filepath.Join(root, "config", "routes", "_sylius", "multi.yaml")
	if err := os.WriteFile(routePath, []byte(existing), 0644); err != nil {
		t.Fatalf("writing existing route file: %v", err)
	}

	p := syliusroute.Params{ResourceName: "multi", Alias: "app.multi", Grid: "app_multi"}
	result, err := syliusroute.EnsureRoute(root, p)
	if err != nil {
		t.Fatalf("EnsureRoute append failed: %v", err)
	}

	if !result.FileUpdated {
		t.Error("expected FileUpdated to be true when appending")
	}
	if result.FileCreated {
		t.Error("expected FileCreated to be false when appending")
	}

	content := readFile(t, routePath)
	assertContains(t, content, "app_admin_ticket:")
	assertContains(t, content, "app_admin_multi:")

	parts := strings.SplitN(content, "app_admin_multi:", 2)
	separator := strings.TrimRight(parts[0], " \t")
	if !strings.HasSuffix(separator, "\n\n") {
		t.Errorf("expected exactly one blank line before new route, got trailing: %q", separator[max(0, len(separator)-4):])
	}
}

func TestEnsureRoute_addsImportToAdminYAML(t *testing.T) {
	root, _ := newTestProject(t)

	p := syliusroute.Params{ResourceName: "widget", Alias: "app.widget"}
	result, err := syliusroute.EnsureRoute(root, p)
	if err != nil {
		t.Fatalf("EnsureRoute failed: %v", err)
	}

	if !result.ImportAdded {
		t.Error("expected ImportAdded to be true")
	}

	adminPath := filepath.Join(root, "config", "routes", "sylius_admin.yaml")
	content := readFile(t, adminPath)
	assertContains(t, content, "app_widget:")
	assertContains(t, content, `resource: "_sylius/widget.yaml"`)
	assertContains(t, content, "prefix: '/%sylius_admin.path_name%'")
}

func TestEnsureRoute_doesNotDuplicateImport(t *testing.T) {
	root, _ := newTestProject(t)

	p := syliusroute.Params{ResourceName: "widget", Alias: "app.widget"}
	if _, err := syliusroute.EnsureRoute(root, p); err != nil {
		t.Fatalf("first EnsureRoute failed: %v", err)
	}

	result, err := syliusroute.EnsureRoute(root, p)
	if err != nil {
		t.Fatalf("second EnsureRoute failed: %v", err)
	}

	if result.ImportAdded {
		t.Error("expected ImportAdded to be false on duplicate")
	}

	adminPath := filepath.Join(root, "config", "routes", "sylius_admin.yaml")
	content := readFile(t, adminPath)
	if count := strings.Count(content, "app_widget:"); count != 1 {
		t.Errorf("expected exactly 1 import for app_widget, found %d", count)
	}
}

func TestEnsureRoute_importBlankLineSeparation(t *testing.T) {
	root, _ := newTestProject(t)

	p := syliusroute.Params{ResourceName: "gadget", Alias: "app.gadget"}
	if _, err := syliusroute.EnsureRoute(root, p); err != nil {
		t.Fatalf("EnsureRoute failed: %v", err)
	}

	adminPath := filepath.Join(root, "config", "routes", "sylius_admin.yaml")
	content := readFile(t, adminPath)

	parts := strings.SplitN(content, "app_gadget:", 2)
	separator := strings.TrimRight(parts[0], " \t")
	if !strings.HasSuffix(separator, "\n\n") {
		t.Errorf("expected exactly one blank line before new import, got: %q", separator[max(0, len(separator)-4):])
	}
}

func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("expected to find %q in:\n%s", substr, s)
	}
}

func assertNotContains(t *testing.T, s, substr string) {
	t.Helper()
	if strings.Contains(s, substr) {
		t.Errorf("expected NOT to find %q in:\n%s", substr, s)
	}
}
