package phpparser_test

import (
	"strings"
	"testing"

	"github.com/sylius-code-mcp/internal/phpparser"
)

const subscriberSimple = `<?php
class MySubscriber implements EventSubscriberInterface
{
    public static function getSubscribedEvents(): array
    {
        return [
            'sylius.menu.admin.main' => ['addAdminMenuItems', -2048],
            'kernel.request' => 'onRequest',
        ];
    }
}
`

func TestParseSubscribedEvents_basic(t *testing.T) {
	events := phpparser.ParseSubscribedEvents(subscriberSimple)

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d: %v", len(events), events)
	}

	byName := make(map[string]string)
	for _, e := range events {
		byName[e.Event] = e.Method
	}

	if byName["sylius.menu.admin.main"] != "addAdminMenuItems" {
		t.Errorf("expected addAdminMenuItems for sylius.menu.admin.main, got %q", byName["sylius.menu.admin.main"])
	}
	if byName["kernel.request"] != "onRequest" {
		t.Errorf("expected onRequest for kernel.request, got %q", byName["kernel.request"])
	}
}

func TestParseSubscribedEvents_noMethod(t *testing.T) {
	events := phpparser.ParseSubscribedEvents("<?php class Foo {}")
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestImplementsEventSubscriber_true(t *testing.T) {
	content := `class MySubscriber implements EventSubscriberInterface {}`
	if !phpparser.ImplementsEventSubscriber(content) {
		t.Error("expected true")
	}
}

func TestImplementsEventSubscriber_multiple(t *testing.T) {
	content := `class MySubscriber implements SomeInterface, EventSubscriberInterface {}`
	if !phpparser.ImplementsEventSubscriber(content) {
		t.Error("expected true for multiple implements")
	}
}

func TestImplementsEventSubscriber_false(t *testing.T) {
	content := `class MyClass implements SomeOtherInterface {}`
	if phpparser.ImplementsEventSubscriber(content) {
		t.Error("expected false")
	}
}

const phpWithMethod = `<?php
class Foo
{
    public function doSomething(): void
    {
        $a = 1;
        if (true) {
            $b = 2;
        }
    }

    public function other(): void
    {
        $c = 3;
    }
}
`

func TestFindMethodBounds_found(t *testing.T) {
	bounds, ok := phpparser.FindMethodBounds(phpWithMethod, "doSomething")
	if !ok {
		t.Fatal("expected method to be found")
	}
	lines := strings.Split(phpWithMethod, "\n")
	if !strings.Contains(lines[bounds.StartLine], "doSomething") {
		t.Errorf("start line %d does not contain 'doSomething': %q", bounds.StartLine, lines[bounds.StartLine])
	}
	if strings.TrimSpace(lines[bounds.EndLine]) != "}" {
		t.Errorf("end line %d is not '}': %q", bounds.EndLine, lines[bounds.EndLine])
	}
}

func TestFindMethodBounds_notFound(t *testing.T) {
	_, ok := phpparser.FindMethodBounds(phpWithMethod, "missingMethod")
	if ok {
		t.Error("expected not found")
	}
}

func TestFindMethodBounds_doesNotOverlapNextMethod(t *testing.T) {
	bounds1, ok1 := phpparser.FindMethodBounds(phpWithMethod, "doSomething")
	bounds2, ok2 := phpparser.FindMethodBounds(phpWithMethod, "other")
	if !ok1 || !ok2 {
		t.Fatal("both methods should be found")
	}
	if bounds1.EndLine >= bounds2.StartLine {
		t.Errorf("first method ends (%d) after second starts (%d)", bounds1.EndLine, bounds2.StartLine)
	}
}

const phpSimpleMethod = `<?php
class Bar
{
    public function handle(): void
    {
        $menu = $event->getMenu();
    }
}
`

func TestInsertBeforeMethodEnd_insertsCode(t *testing.T) {
	result, err := phpparser.InsertBeforeMethodEnd(phpSimpleMethod, "handle", "        $menu->addChild('foo');")
	if err != nil {
		t.Fatalf("InsertBeforeMethodEnd: %v", err)
	}
	if !strings.Contains(result, "$menu->addChild('foo');") {
		t.Error("expected inserted code to be present")
	}
	insertPos := strings.Index(result, "$menu->addChild('foo');")
	closingPos := strings.LastIndex(result, "    }")
	if insertPos > closingPos {
		t.Error("inserted code should appear before the method's closing brace")
	}
}

func TestInsertBeforeMethodEnd_unknownMethod(t *testing.T) {
	_, err := phpparser.InsertBeforeMethodEnd(phpSimpleMethod, "missing", "code")
	if err == nil {
		t.Error("expected error for missing method")
	}
}

func TestRenderMenuItemCode_withParent(t *testing.T) {
	code := phpparser.RenderMenuItemCode("sales", "my_item", "My Item", "app_my_item_index", "tabler:box", 0, nil)
	if !strings.Contains(code, "getChild('sales')") {
		t.Error("expected getChild for parent")
	}
	if !strings.Contains(code, "addChild('my_item'") {
		t.Error("expected addChild with item key")
	}
	if !strings.Contains(code, "'app_my_item_index'") {
		t.Error("expected route")
	}
	if !strings.Contains(code, "setLabel('My Item')") {
		t.Error("expected setLabel")
	}
	if !strings.Contains(code, "'tabler:box'") {
		t.Error("expected icon")
	}
}

func TestRenderMenuItemCode_rootMenu(t *testing.T) {
	code := phpparser.RenderMenuItemCode("", "my_item", "My Item", "app_my_item_index", "", 5, nil)
	if strings.Contains(code, "getChild") {
		t.Error("root menu item should not use getChild")
	}
	if !strings.Contains(code, "$menu->addChild") {
		t.Error("expected direct addChild on menu")
	}
	if !strings.Contains(code, "setExtra") {
		t.Error("expected weight via setExtra")
	}
}

func TestRenderMenuItemCode_noIcon(t *testing.T) {
	code := phpparser.RenderMenuItemCode("", "item", "Label", "route", "", 0, nil)
	if strings.Contains(code, "setLabelAttribute") {
		t.Error("should not include icon when empty")
	}
}

func TestRenderMenuItemCode_withRoutes(t *testing.T) {
	routes := []string{
		"app_admin_my_resource_index",
		"app_admin_my_resource_create",
		"app_admin_my_resource_update",
	}
	code := phpparser.RenderMenuItemCode("", "my_resource", "app.ui.my_resource", "app_admin_my_resource_index", "", 0, routes)
	if !strings.Contains(code, "->setExtra('routes', [") {
		t.Error("expected setExtra('routes', [...])")
	}
	for _, r := range routes {
		if !strings.Contains(code, "'"+r+"'") {
			t.Errorf("expected route %q in output", r)
		}
	}
	if !strings.HasSuffix(strings.TrimSpace(code), ";") {
		t.Error("generated code must end with ;")
	}
}

func TestNewMenuSubscriberContent(t *testing.T) {
	content := phpparser.NewMenuSubscriberContent()
	checks := []string{
		"namespace App\\EventListener\\Menu",
		"class MenuSubscriber implements EventSubscriberInterface",
		"getSubscribedEvents",
		"sylius.menu.admin.main",
		"addAdminMenuItems",
		"MenuBuilderEvent",
		"$menu = $event->getMenu();",
	}
	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Errorf("expected %q in new subscriber content", check)
		}
	}
}
