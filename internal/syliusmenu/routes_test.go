package syliusmenu_test

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/sylius-code-mcp/internal/syliusmenu"
)

func writeSyliusRoute(t *testing.T, root, filename, content string) {
	t.Helper()
	dir := filepath.Join(root, "config", "routes", "_sylius")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func writeCacheRoutes(t *testing.T, root, content string) {
	t.Helper()
	dir := filepath.Join(root, "var", "cache", "dev")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "url_generating_routes.php"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func assertRoutes(t *testing.T, got, want []string) {
	t.Helper()
	sort.Strings(want)
	if len(got) != len(want) {
		t.Errorf("route count: got %d %v, want %d %v", len(got), got, len(want), want)
		return
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("route[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestDiscoverResourceRoutes_fromCache(t *testing.T) {
	root := t.TempDir()
	writeCacheRoutes(t, root, `<?php
return [
    'app_admin_chat_agent_index'      => [[], [], [], [], [], [], []],
    'app_admin_chat_agent_create'     => [[], [], [], [], [], [], []],
    'app_admin_chat_agent_update'     => [[], [], [], [], [], [], []],
    'app_admin_chat_agent_bulk_delete'=> [[], [], [], [], [], [], []],
    'app_admin_chat_agent_delete'     => [[], [], [], [], [], [], []],
    'app_admin_other_index'           => [[], [], [], [], [], [], []],
];`)

	got := syliusmenu.DiscoverResourceRoutes(root, "app_admin_chat_agent_index")
	assertRoutes(t, got, []string{
		"app_admin_chat_agent_bulk_delete",
		"app_admin_chat_agent_create",
		"app_admin_chat_agent_delete",
		"app_admin_chat_agent_index",
		"app_admin_chat_agent_update",
	})
}

func TestDiscoverResourceRoutes_cachePreferredOverYAML(t *testing.T) {
	root := t.TempDir()
	writeSyliusRoute(t, root, "my_resource.yaml", `
app_admin_my_resource:
    resource: |
        alias: app.my_resource
        only: ['index']
    type: sylius.resource
`)
	writeCacheRoutes(t, root, `<?php
return [
    'app_admin_my_resource_index'  => [[], [], [], [], [], [], []],
    'app_admin_my_resource_create' => [[], [], [], [], [], [], []],
];`)

	got := syliusmenu.DiscoverResourceRoutes(root, "app_admin_my_resource_index")
	assertRoutes(t, got, []string{
		"app_admin_my_resource_create",
		"app_admin_my_resource_index",
	})
}

func TestDiscoverResourceRoutes_yamlNoRestrictions(t *testing.T) {
	root := t.TempDir()
	writeSyliusRoute(t, root, "my_resource.yaml", `
app_admin_my_resource:
    resource: |
        alias: app.my_resource
        section: admin
        grid: app_my_resource
    type: sylius.resource
`)

	got := syliusmenu.DiscoverResourceRoutes(root, "app_admin_my_resource_index")
	assertRoutes(t, got, []string{
		"app_admin_my_resource_create",
		"app_admin_my_resource_index",
		"app_admin_my_resource_show",
		"app_admin_my_resource_update",
	})
}

func TestDiscoverResourceRoutes_yamlWithOnly(t *testing.T) {
	root := t.TempDir()
	writeSyliusRoute(t, root, "order_invoice.yaml", `
app_admin_order_invoice:
    resource: |
        alias: app.order_invoice
        section: admin
        only: ['index']
        grid: app_admin_order_invoice
    type: sylius.resource
`)

	got := syliusmenu.DiscoverResourceRoutes(root, "app_admin_order_invoice_index")
	assertRoutes(t, got, []string{
		"app_admin_order_invoice_index",
	})
}

func TestDiscoverResourceRoutes_yamlWithExcept(t *testing.T) {
	root := t.TempDir()
	writeSyliusRoute(t, root, "assembly_assembler.yaml", `
app_admin_assembly_assembler:
    resource: |
        alias: app.assembly_assembler
        section: admin
        except: ['show']
    type: sylius.resource
`)

	got := syliusmenu.DiscoverResourceRoutes(root, "app_admin_assembly_assembler_index")
	assertRoutes(t, got, []string{
		"app_admin_assembly_assembler_create",
		"app_admin_assembly_assembler_index",
		"app_admin_assembly_assembler_update",
	})
}

func TestDiscoverResourceRoutes_yamlWithCustomRoutes(t *testing.T) {
	root := t.TempDir()
	writeSyliusRoute(t, root, "assembly_assembler.yaml", `
app_admin_assembly_assembler:
    resource: |
        alias: app.assembly_assembler
        section: admin
        except: ['show']
    type: sylius.resource

app_admin_assembly_assembler_assembled_show:
    path: '/assembly/assembler/assembled/show/{id}'
    defaults:
        _controller: app.controller.assembly_assembler::showAction

app_admin_assembly_assembler_assembled_update:
    path: '/assembly/assembler/change-assembled/{id}'
    defaults:
        _controller: app.controller.assembly_assembler::updateAction
`)

	got := syliusmenu.DiscoverResourceRoutes(root, "app_admin_assembly_assembler_index")
	assertRoutes(t, got, []string{
		"app_admin_assembly_assembler_assembled_show",
		"app_admin_assembly_assembler_assembled_update",
		"app_admin_assembly_assembler_create",
		"app_admin_assembly_assembler_index",
		"app_admin_assembly_assembler_update",
	})
}

func TestDiscoverResourceRoutes_noFilesUsesDefaults(t *testing.T) {
	root := t.TempDir()

	got := syliusmenu.DiscoverResourceRoutes(root, "app_admin_my_resource_index")
	assertRoutes(t, got, []string{
		"app_admin_my_resource_create",
		"app_admin_my_resource_index",
		"app_admin_my_resource_show",
		"app_admin_my_resource_update",
	})
}

func TestDiscoverResourceRoutes_nonAppAdminRoute(t *testing.T) {
	root := t.TempDir()
	got := syliusmenu.DiscoverResourceRoutes(root, "sylius_admin_order_index")
	if len(got) != 0 {
		t.Errorf("expected nil for non-app_admin_ route, got %v", got)
	}
}

func TestDiscoverResourceRoutes_multiSegmentResourceName(t *testing.T) {
	root := t.TempDir()
	writeSyliusRoute(t, root, "assembly.yaml", `
app_admin_assembly:
    resource: |
        alias: app.assembly
    type: sylius.resource
`)
	writeSyliusRoute(t, root, "assembly_assembler.yaml", `
app_admin_assembly_assembler:
    resource: |
        alias: app.assembly_assembler
        except: ['show']
    type: sylius.resource
`)

	got := syliusmenu.DiscoverResourceRoutes(root, "app_admin_assembly_assembler_index")
	for _, r := range got {
		if r == "app_admin_assembly_assembler_show" {
			t.Error("show route should be excluded per except: ['show']")
		}
	}
	found := false
	for _, r := range got {
		if r == "app_admin_assembly_assembler_index" {
			found = true
		}
	}
	if !found {
		t.Error("expected app_admin_assembly_assembler_index in results")
	}
}
