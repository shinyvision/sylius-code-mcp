package syliusmenu_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sylius-code-mcp/internal/container"
	"github.com/sylius-code-mcp/internal/phpparser"
	"github.com/sylius-code-mcp/internal/syliusmenu"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(data)
}

func newMinimalProject(t *testing.T, withSubscriber bool) string {
	t.Helper()
	root := t.TempDir()

	writeFile(t, filepath.Join(root, "composer.json"),
		`{"autoload":{"psr-4":{"App\\":"src/"}}}`)

	if withSubscriber {
		writeFile(t,
			filepath.Join(root, "src", "EventListener", "Menu", "MenuSubscriber.php"),
			phpparser.NewMenuSubscriberContent(),
		)
	}
	return root
}

func newContainerWithSubscriber(projectRoot string) *container.Container {
	return &container.Container{
		WorkspaceRoot: projectRoot,
		Services: map[string]container.Service{
			`App\EventListener\Menu\MenuSubscriber`: {
				ID:    `App\EventListener\Menu\MenuSubscriber`,
				Class: `App\EventListener\Menu\MenuSubscriber`,
				Tags: []container.ServiceTag{
					{Name: "kernel.event_subscriber", Attrs: map[string]string{}},
				},
			},
		},
		Aliases:     make(map[string]string),
		TwigBundles: make(map[string][]string),
	}
}

func newContainerWithEventListener(projectRoot string) *container.Container {
	return &container.Container{
		WorkspaceRoot: projectRoot,
		Services: map[string]container.Service{
			"app.menu_listener": {
				ID:    "app.menu_listener",
				Class: `App\EventListener\Menu\MenuSubscriber`,
				Tags: []container.ServiceTag{
					{
						Name: "kernel.event_listener",
						Attrs: map[string]string{
							"event":  "sylius.menu.admin.main",
							"method": "addAdminMenuItems",
						},
					},
				},
			},
		},
		Aliases:     make(map[string]string),
		TwigBundles: make(map[string][]string),
	}
}

func TestFindMenuHandlers_eventSubscriber(t *testing.T) {
	root := newMinimalProject(t, true)
	cnt := newContainerWithSubscriber(root)
	psr4 := container.PSR4Map{"App\\": []string{"src/"}}

	handlers := syliusmenu.FindMenuHandlers(root, cnt, psr4)

	if len(handlers) != 1 {
		t.Fatalf("expected 1 handler, got %d: %v", len(handlers), handlers)
	}
	if handlers[0].MethodName != "addAdminMenuItems" {
		t.Errorf("unexpected method name: %s", handlers[0].MethodName)
	}
	if !strings.HasSuffix(handlers[0].FilePath, "MenuSubscriber.php") {
		t.Errorf("unexpected file path: %s", handlers[0].FilePath)
	}
}

func TestFindMenuHandlers_eventListener(t *testing.T) {
	root := newMinimalProject(t, true)
	cnt := newContainerWithEventListener(root)
	psr4 := container.PSR4Map{"App\\": []string{"src/"}}

	handlers := syliusmenu.FindMenuHandlers(root, cnt, psr4)

	if len(handlers) != 1 {
		t.Fatalf("expected 1 handler, got %d", len(handlers))
	}
	if handlers[0].MethodName != "addAdminMenuItems" {
		t.Errorf("unexpected method: %s", handlers[0].MethodName)
	}
}

func TestFindMenuHandlers_nilContainer(t *testing.T) {
	root := newMinimalProject(t, true)
	psr4 := container.PSR4Map{"App\\": []string{"src/"}}

	handlers := syliusmenu.FindMenuHandlers(root, nil, psr4)
	if len(handlers) != 0 {
		t.Error("nil container should return empty handlers")
	}
}

func TestFindMenuHandlers_subscriberNotListeningToMenuEvent(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "composer.json"),
		`{"autoload":{"psr-4":{"App\\":"src/"}}}`)

	writeFile(t, filepath.Join(root, "src", "EventListener", "OtherSubscriber.php"), `<?php
class OtherSubscriber implements EventSubscriberInterface {
    public static function getSubscribedEvents(): array {
        return ['kernel.request' => 'onRequest'];
    }
    public function onRequest() {}
}`)

	cnt := &container.Container{
		WorkspaceRoot: root,
		Services: map[string]container.Service{
			`App\EventListener\OtherSubscriber`: {
				ID: `App\EventListener\OtherSubscriber`, Class: `App\EventListener\OtherSubscriber`,
				Tags: []container.ServiceTag{{Name: "kernel.event_subscriber", Attrs: map[string]string{}}},
			},
		},
		Aliases:     make(map[string]string),
		TwigBundles: make(map[string][]string),
	}
	psr4 := container.PSR4Map{"App\\": []string{"src/"}}

	handlers := syliusmenu.FindMenuHandlers(root, cnt, psr4)
	if len(handlers) != 0 {
		t.Error("subscriber not listening to menu event should not be returned")
	}
}

func TestAddMenuItem_createsSubscriberWhenNoneExists(t *testing.T) {
	root := newMinimalProject(t, false)
	result, err := syliusmenu.AddMenuItem(root, syliusmenu.Params{
		ItemKey: "my_resource",
		Label:   "My Resource",
		Route:   "app_admin_my_resource_index",
		Icon:    "tabler:box",
	})
	if err != nil {
		t.Fatalf("AddMenuItem: %v", err)
	}
	if !result.FileCreated {
		t.Error("expected FileCreated=true when subscriber is created from scratch")
	}

	content := readFile(t, result.FilePath)
	if !strings.Contains(content, "addChild('my_resource'") {
		t.Error("expected addChild for new item")
	}
	if !strings.Contains(content, "'app_admin_my_resource_index'") {
		t.Error("expected route in generated code")
	}
	if !strings.Contains(content, "setLabel('My Resource')") {
		t.Error("expected label")
	}
}

func TestAddMenuItem_insertsIntoExistingSubscriber(t *testing.T) {
	root := newMinimalProject(t, true)
	subscriberPath := filepath.Join(root, "src", "EventListener", "Menu", "MenuSubscriber.php")

	xmlContent := buildContainerXML(`App\EventListener\Menu\MenuSubscriber`, "kernel.event_subscriber", "", "")
	writeContainerXML(t, root, xmlContent)

	result, err := syliusmenu.AddMenuItem(root, syliusmenu.Params{
		ParentKey: "sales",
		ItemKey:   "orders_v2",
		Label:     "Orders V2",
		Route:     "app_admin_orders_v2_index",
	})
	if err != nil {
		t.Fatalf("AddMenuItem: %v", err)
	}
	if result.FileCreated {
		t.Error("expected FileCreated=false when subscriber already exists")
	}
	if result.FilePath != subscriberPath {
		t.Errorf("expected file path %s, got %s", subscriberPath, result.FilePath)
	}

	content := readFile(t, subscriberPath)
	if !strings.Contains(content, "getChild('sales')") {
		t.Error("expected getChild for parent key")
	}
	if !strings.Contains(content, "addChild('orders_v2'") {
		t.Error("expected addChild for item key")
	}
}

func TestAddMenuItem_missingItemKey(t *testing.T) {
	_, err := syliusmenu.AddMenuItem(t.TempDir(), syliusmenu.Params{
		Label: "Label", Route: "route",
	})
	if err == nil {
		t.Error("expected error for missing itemKey")
	}
}

func TestAddMenuItem_missingLabel(t *testing.T) {
	_, err := syliusmenu.AddMenuItem(t.TempDir(), syliusmenu.Params{
		ItemKey: "key", Route: "route",
	})
	if err == nil {
		t.Error("expected error for missing label")
	}
}

func TestAddMenuItem_missingRoute(t *testing.T) {
	_, err := syliusmenu.AddMenuItem(t.TempDir(), syliusmenu.Params{
		ItemKey: "key", Label: "Label",
	})
	if err == nil {
		t.Error("expected error for missing route")
	}
}

func TestAddMenuItem_canCallTwice(t *testing.T) {
	root := newMinimalProject(t, false)

	params := syliusmenu.Params{
		ItemKey: "item_one",
		Label:   "Item One",
		Route:   "app_admin_item_one_index",
	}

	if _, err := syliusmenu.AddMenuItem(root, params); err != nil {
		t.Fatalf("first AddMenuItem: %v", err)
	}
	params.ItemKey = "item_two"
	params.Label = "Item Two"
	params.Route = "app_admin_item_two_index"
	if _, err := syliusmenu.AddMenuItem(root, params); err != nil {
		t.Fatalf("second AddMenuItem: %v", err)
	}

	subscriberPath := filepath.Join(root, "src", "EventListener", "Menu", "MenuSubscriber.php")
	content := readFile(t, subscriberPath)
	if !strings.Contains(content, "addChild('item_one'") {
		t.Error("expected first item")
	}
	if !strings.Contains(content, "addChild('item_two'") {
		t.Error("expected second item")
	}
}

func buildContainerXML(class, tagName, tagEvent, tagMethod string) string {
	tagAttrs := ""
	if tagEvent != "" {
		tagAttrs += ` event="` + tagEvent + `"`
	}
	if tagMethod != "" {
		tagAttrs += ` method="` + tagMethod + `"`
	}
	return `<?xml version="1.0"?>
<container xmlns="http://symfony.com/schema/dic/services">
  <services>
    <service id="` + class + `" class="` + class + `" autowire="true">
      <tag name="` + tagName + `"` + tagAttrs + `/>
    </service>
  </services>
</container>`
}

func writeContainerXML(t *testing.T, root, content string) {
	t.Helper()
	dir := filepath.Join(root, "var", "cache", "dev")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "App_KernelDevDebugContainer.xml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
