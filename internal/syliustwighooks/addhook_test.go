package syliustwighooks_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sylius-code-mcp/internal/syliustwighooks"
)

func intPtr(v int) *int    { return &v }
func boolPtr(v bool) *bool { return &v }

func readYAML(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(data)
}

func TestAddHookCreatesFile(t *testing.T) {
	root := t.TempDir()

	res, err := syliustwighooks.AddHook(root, syliustwighooks.AddHookParams{
		CategoryName: "payment_method",
		HookName:     "sylius_admin.payment_method.update.content.header",
		Hookable: syliustwighooks.HookableSpec{
			Name:     "breadcrumbs",
			Template: "@SyliusAdmin/shared/crud/update/content/header/breadcrumbs.html.twig",
			Priority: intPtr(100),
			Enabled:  boolPtr(true),
		},
	})
	if err != nil {
		t.Fatalf("AddHook: %v", err)
	}
	if !res.FileCreated {
		t.Errorf("expected FileCreated=true")
	}

	abs := filepath.Join(root, "config/packages/twig_hooks/payment_method.yaml")
	got := readYAML(t, abs)

	for _, want := range []string{
		"sylius_twig_hooks:",
		"    hooks:",
		"        'sylius_admin.payment_method.update.content.header':",
		"            breadcrumbs:",
		"                template: '@SyliusAdmin/shared/crud/update/content/header/breadcrumbs.html.twig'",
		"                priority: 100",
		"                enabled: true",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected file to contain %q.\nGot:\n%s", want, got)
		}
	}
}

func TestAddHookOmitsOptionalFields(t *testing.T) {
	root := t.TempDir()

	_, err := syliustwighooks.AddHook(root, syliustwighooks.AddHookParams{
		CategoryName: "payment_method",
		HookName:     "sylius_admin.payment_method.update.content.header",
		Hookable: syliustwighooks.HookableSpec{
			Name:     "breadcrumbs",
			Template: "@SyliusAdmin/foo.html.twig",
		},
	})
	if err != nil {
		t.Fatalf("AddHook: %v", err)
	}

	got := readYAML(t, filepath.Join(root, "config/packages/twig_hooks/payment_method.yaml"))
	if strings.Contains(got, "priority:") {
		t.Errorf("did not expect priority field.\nGot:\n%s", got)
	}
	if strings.Contains(got, "enabled:") {
		t.Errorf("did not expect enabled field.\nGot:\n%s", got)
	}
}

func TestAddHookSortsHooksLexicographically(t *testing.T) {
	root := t.TempDir()

	add := func(hook, hookable string) {
		t.Helper()
		_, err := syliustwighooks.AddHook(root, syliustwighooks.AddHookParams{
			CategoryName: "shared",
			HookName:     hook,
			Hookable: syliustwighooks.HookableSpec{
				Name:     hookable,
				Template: "@App/" + hookable + ".html.twig",
			},
		})
		if err != nil {
			t.Fatalf("AddHook(%s): %v", hook, err)
		}
	}

	add("sylius_admin.payment_method.update.content.sections", "general")
	add("sylius_admin.channel.index.content", "main")
	add("sylius_admin.payment_method.update.content.header", "breadcrumbs")

	got := readYAML(t, filepath.Join(root, "config/packages/twig_hooks/shared.yaml"))

	channelIdx := strings.Index(got, "'sylius_admin.channel.index.content'")
	headerIdx := strings.Index(got, "'sylius_admin.payment_method.update.content.header'")
	sectionsIdx := strings.Index(got, "'sylius_admin.payment_method.update.content.sections'")

	if channelIdx < 0 || headerIdx < 0 || sectionsIdx < 0 {
		t.Fatalf("missing one of the hook keys.\nGot:\n%s", got)
	}
	if !(channelIdx < headerIdx && headerIdx < sectionsIdx) {
		t.Errorf("expected lexicographic ordering channel < header < sections, got idx %d %d %d.\nFile:\n%s",
			channelIdx, headerIdx, sectionsIdx, got)
	}
}

func TestAddHookReplacesExistingHookable(t *testing.T) {
	root := t.TempDir()

	_, err := syliustwighooks.AddHook(root, syliustwighooks.AddHookParams{
		CategoryName: "payment_method",
		HookName:     "sylius_admin.payment_method.update.content.header",
		Hookable: syliustwighooks.HookableSpec{
			Name:     "breadcrumbs",
			Template: "@App/old.html.twig",
			Priority: intPtr(10),
		},
	})
	if err != nil {
		t.Fatalf("first AddHook: %v", err)
	}

	res, err := syliustwighooks.AddHook(root, syliustwighooks.AddHookParams{
		CategoryName: "payment_method",
		HookName:     "sylius_admin.payment_method.update.content.header",
		Hookable: syliustwighooks.HookableSpec{
			Name:     "breadcrumbs",
			Template: "@App/new.html.twig",
		},
	})
	if err != nil {
		t.Fatalf("second AddHook: %v", err)
	}
	if !res.HookableUpdated {
		t.Errorf("expected HookableUpdated=true")
	}
	if res.HookableCreated {
		t.Errorf("expected HookableCreated=false on replace")
	}

	got := readYAML(t, filepath.Join(root, "config/packages/twig_hooks/payment_method.yaml"))
	if !strings.Contains(got, "@App/new.html.twig") {
		t.Errorf("expected new template path.\nGot:\n%s", got)
	}
	if strings.Contains(got, "@App/old.html.twig") {
		t.Errorf("expected old template removed.\nGot:\n%s", got)
	}
	if strings.Contains(got, "priority: 10") {
		t.Errorf("expected old priority cleared on replace.\nGot:\n%s", got)
	}
}

func TestAddHookAppendsHookableUnderExistingHook(t *testing.T) {
	root := t.TempDir()

	_, err := syliustwighooks.AddHook(root, syliustwighooks.AddHookParams{
		CategoryName: "payment_method",
		HookName:     "sylius_admin.payment_method.update.content.header",
		Hookable: syliustwighooks.HookableSpec{
			Name:     "breadcrumbs",
			Template: "@App/breadcrumbs.html.twig",
		},
	})
	if err != nil {
		t.Fatalf("first AddHook: %v", err)
	}

	res, err := syliustwighooks.AddHook(root, syliustwighooks.AddHookParams{
		CategoryName: "payment_method",
		HookName:     "sylius_admin.payment_method.update.content.header",
		Hookable: syliustwighooks.HookableSpec{
			Name:     "title",
			Template: "@App/title.html.twig",
		},
	})
	if err != nil {
		t.Fatalf("second AddHook: %v", err)
	}
	if !res.HookableCreated || res.HookCreated {
		t.Errorf("expected HookableCreated=true, HookCreated=false; got %+v", res)
	}

	got := readYAML(t, filepath.Join(root, "config/packages/twig_hooks/payment_method.yaml"))
	if !strings.Contains(got, "breadcrumbs:") {
		t.Errorf("expected breadcrumbs to remain.\nGot:\n%s", got)
	}
	if !strings.Contains(got, "title:") {
		t.Errorf("expected title to be added.\nGot:\n%s", got)
	}
}

func TestAddHookValidatesInput(t *testing.T) {
	root := t.TempDir()

	cases := []syliustwighooks.AddHookParams{
		{CategoryName: "", HookName: "x.y", Hookable: syliustwighooks.HookableSpec{Name: "a", Template: "t"}},
		{CategoryName: "Bad-Name", HookName: "x.y", Hookable: syliustwighooks.HookableSpec{Name: "a", Template: "t"}},
		{CategoryName: "ok", HookName: "", Hookable: syliustwighooks.HookableSpec{Name: "a", Template: "t"}},
		{CategoryName: "ok", HookName: "x.y", Hookable: syliustwighooks.HookableSpec{Name: "", Template: "t"}},
		{CategoryName: "ok", HookName: "x.y", Hookable: syliustwighooks.HookableSpec{Name: "a", Template: ""}},
	}
	for i, p := range cases {
		if _, err := syliustwighooks.AddHook(root, p); err == nil {
			t.Errorf("case %d: expected error for %+v", i, p)
		}
	}
}
