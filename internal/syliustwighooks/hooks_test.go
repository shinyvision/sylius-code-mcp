package syliustwighooks_test

import (
	"strings"
	"testing"

	"github.com/sylius-code-mcp/internal/syliustwighooks"
)

var sampleIDs = []string{
	"sylius_twig_hooks.hook.normalizer.name.composite",
	"sylius_twig_hooks.hook.sylius_admin.payment_method.update.content.hookable.form",
	"sylius_twig_hooks.hook.sylius_admin.payment_method.update.content.hookable.mollie_info",
	"sylius_twig_hooks.hook.sylius_admin.payment_method.update.content.form.hookable.sections",
	"sylius_twig_hooks.hook.sylius_admin.payment_method.update.content.form.sections.hookable.general",
	"sylius_twig_hooks.hook.sylius_admin.payment_method.update.content.form.sections.general.hookable.code",
	"sylius_twig_hooks.hook.sylius_admin.payment_method.create.hookable.content",
	"sylius_twig_hooks.hook.sylius_admin.admin_user.create.content.form.sections#left.hookable.account",
	"some.unrelated.service",
}

func TestBuildAndRenderSubtree(t *testing.T) {
	root := syliustwighooks.BuildHookTreeFromIDs(sampleIDs)

	out := syliustwighooks.RenderSubtree(root, "sylius_admin.payment_method.update")

	for _, want := range []string{
		"# Sylius Twig hooks under `sylius_admin.payment_method.update`",
		"- content",
		"  - form",
		"    - sections",
		"      - general",
		"        - code",
		"  - mollie_info",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q.\nGot:\n%s", want, out)
		}
	}
}

func TestSectionSeparatorTreatedAsSegment(t *testing.T) {
	root := syliustwighooks.BuildHookTreeFromIDs(sampleIDs)
	out := syliustwighooks.RenderSubtree(root, "sylius_admin.admin_user.create.content.form")
	if !strings.Contains(out, "- sections#left") {
		t.Errorf("expected sections#left as direct child.\nGot:\n%s", out)
	}
	if !strings.Contains(out, "  - account") {
		t.Errorf("expected account under sections#left.\nGot:\n%s", out)
	}
}

func TestPrefixNotFound(t *testing.T) {
	root := syliustwighooks.BuildHookTreeFromIDs(sampleIDs)
	out := syliustwighooks.RenderSubtree(root, "does.not.exist")
	if !strings.Contains(out, "No hooks found for prefix `does.not.exist`") {
		t.Errorf("expected not-found message.\nGot:\n%s", out)
	}
	if !strings.Contains(out, "Top-level hook namespaces:") {
		t.Errorf("expected top-level namespaces hint.\nGot:\n%s", out)
	}
}

func TestTooManyTriggersDrillDownHint(t *testing.T) {
	ids := make([]string, 0, 300)
	for i := 0; i < 260; i++ {
		ids = append(ids,
			"sylius_twig_hooks.hook.app.big.hookable.item_"+strings.Repeat("x", 1)+itoa(i))
	}
	root := syliustwighooks.BuildHookTreeFromIDs(ids)
	out := syliustwighooks.RenderSubtree(root, "app.big")
	if !strings.Contains(out, "Too many hooks under `app.big`") {
		t.Errorf("expected too-many message.\nGot:\n%s", out)
	}
	if !strings.Contains(out, "Direct children you can drill into:") {
		t.Errorf("expected drill-down hint.\nGot:\n%s", out)
	}
}

func TestEmptyPrefixOnSmallTreeRenders(t *testing.T) {
	root := syliustwighooks.BuildHookTreeFromIDs([]string{
		"sylius_twig_hooks.hook.foo.hookable.bar",
		"sylius_twig_hooks.hook.foo.bar.hookable.baz",
	})
	out := syliustwighooks.RenderSubtree(root, "")
	for _, want := range []string{"# Sylius Twig hooks", "- foo", "  - bar", "    - baz"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q.\nGot:\n%s", want, out)
		}
	}
}

func TestExistingHookWithoutChildren(t *testing.T) {
	root := syliustwighooks.BuildHookTreeFromIDs([]string{
		"sylius_twig_hooks.hook.foo.hookable.bar",
	})
	out := syliustwighooks.RenderSubtree(root, "foo.bar")
	if !strings.Contains(out, "exists but has no hookable children") {
		t.Errorf("expected leaf message.\nGot:\n%s", out)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	n := i
	if n < 0 {
		n = -n
	}
	for n > 0 {
		pos--
		b[pos] = byte('0' + n%10)
		n /= 10
	}
	if i < 0 {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}
