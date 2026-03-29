package container_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sylius-code-mcp/internal/container"
)

const minimalContainerXML = `<?xml version="1.0" encoding="utf-8"?>
<container xmlns="http://symfony.com/schema/dic/services">
  <services>
    <service id="App\EventListener\Menu\MenuSubscriber"
             class="App\EventListener\Menu\MenuSubscriber"
             autowire="true" autoconfigure="true">
      <tag name="kernel.event_subscriber"/>
      <argument type="service" id="some.dependency"/>
    </service>

    <service id="app.my_listener"
             class="App\SomeListener"
             public="true">
      <tag name="kernel.event_listener" event="sylius.menu.admin.main" method="onMenu"/>
    </service>

    <service id="abstract.base" class="App\Base" abstract="true">
      <tag name="some.tag"/>
    </service>

    <service id="app.alias" alias="App\EventListener\Menu\MenuSubscriber" public="false"/>

    <service id="twig.loader.native_filesystem"
             class="Twig\Loader\FilesystemLoader">
      <call method="addPath">
        <argument>/abs/templates</argument>
      </call>
      <call method="addPath">
        <argument>/abs/bundle/templates</argument>
        <argument>MyBundle</argument>
      </call>
      <call method="addPath">
        <argument>/abs/excluded/templates</argument>
        <argument>!ExcludedBundle</argument>
      </call>
    </service>
  </services>
</container>`

func writeContainerXML(t *testing.T, root, content string) string {
	t.Helper()
	dir := filepath.Join(root, "var", "cache", "dev")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "App_KernelDevDebugContainer.xml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParseXML_services(t *testing.T) {
	root := t.TempDir()
	xmlPath := writeContainerXML(t, root, minimalContainerXML)

	c, err := container.ParseXML(xmlPath, root)
	if err != nil {
		t.Fatalf("ParseXML: %v", err)
	}

	svc, ok := c.Services[`App\EventListener\Menu\MenuSubscriber`]
	if !ok {
		t.Fatal("expected MenuSubscriber service")
	}
	if svc.Class != `App\EventListener\Menu\MenuSubscriber` {
		t.Errorf("unexpected class: %s", svc.Class)
	}
}

func TestParseXML_serviceTags(t *testing.T) {
	root := t.TempDir()
	xmlPath := writeContainerXML(t, root, minimalContainerXML)

	c, err := container.ParseXML(xmlPath, root)
	if err != nil {
		t.Fatalf("ParseXML: %v", err)
	}

	svc := c.Services[`App\EventListener\Menu\MenuSubscriber`]
	if !svc.HasTag("kernel.event_subscriber") {
		t.Error("expected kernel.event_subscriber tag on MenuSubscriber")
	}

	listener := c.Services["app.my_listener"]
	if !listener.HasTag("kernel.event_listener") {
		t.Error("expected kernel.event_listener tag on app.my_listener")
	}
	if listener.TagAttr("kernel.event_listener", "event") != "sylius.menu.admin.main" {
		t.Errorf("expected event attr 'sylius.menu.admin.main', got %q",
			listener.TagAttr("kernel.event_listener", "event"))
	}
	if listener.TagAttr("kernel.event_listener", "method") != "onMenu" {
		t.Errorf("unexpected method attr: %s", listener.TagAttr("kernel.event_listener", "method"))
	}
}

func TestParseXML_abstractServiceSkipped(t *testing.T) {
	root := t.TempDir()
	xmlPath := writeContainerXML(t, root, minimalContainerXML)

	c, err := container.ParseXML(xmlPath, root)
	if err != nil {
		t.Fatalf("ParseXML: %v", err)
	}

	if _, ok := c.Services["abstract.base"]; ok {
		t.Error("abstract service should not be stored")
	}
}

func TestParseXML_aliases(t *testing.T) {
	root := t.TempDir()
	xmlPath := writeContainerXML(t, root, minimalContainerXML)

	c, err := container.ParseXML(xmlPath, root)
	if err != nil {
		t.Fatalf("ParseXML: %v", err)
	}

	target, ok := c.Aliases["app.alias"]
	if !ok {
		t.Fatal("expected alias app.alias")
	}
	if target != `App\EventListener\Menu\MenuSubscriber` {
		t.Errorf("unexpected alias target: %s", target)
	}
}

func TestParseXML_twigRoots(t *testing.T) {
	root := t.TempDir()
	xmlPath := writeContainerXML(t, root, minimalContainerXML)

	c, err := container.ParseXML(xmlPath, root)
	if err != nil {
		t.Fatalf("ParseXML: %v", err)
	}

	if len(c.TwigRoots) != 1 || c.TwigRoots[0] != "/abs/templates" {
		t.Errorf("expected TwigRoots=[/abs/templates], got %v", c.TwigRoots)
	}

	bundlePaths, ok := c.TwigBundles["MyBundle"]
	if !ok || len(bundlePaths) != 1 || bundlePaths[0] != "/abs/bundle/templates" {
		t.Errorf("unexpected TwigBundles: %v", c.TwigBundles)
	}

	if _, excluded := c.TwigBundles["!ExcludedBundle"]; excluded {
		t.Error("bundle starting with '!' should be excluded")
	}
}

func TestServicesWithTag(t *testing.T) {
	root := t.TempDir()
	xmlPath := writeContainerXML(t, root, minimalContainerXML)

	c, err := container.ParseXML(xmlPath, root)
	if err != nil {
		t.Fatalf("ParseXML: %v", err)
	}

	subscribers := c.ServicesWithTag("kernel.event_subscriber")
	if len(subscribers) != 1 {
		t.Fatalf("expected 1 event subscriber, got %d", len(subscribers))
	}
	if subscribers[0].ID != `App\EventListener\Menu\MenuSubscriber` {
		t.Errorf("unexpected subscriber ID: %s", subscribers[0].ID)
	}
}

func TestFindContainerXML(t *testing.T) {
	root := t.TempDir()
	writeContainerXML(t, root, minimalContainerXML)

	path, err := container.FindContainerXML(root)
	if err != nil {
		t.Fatalf("FindContainerXML: %v", err)
	}
	if !strings.HasSuffix(path, ".xml") {
		t.Errorf("expected .xml path, got %s", path)
	}
}

func TestFindContainerXML_missing(t *testing.T) {
	_, err := container.FindContainerXML(t.TempDir())
	if err == nil {
		t.Error("expected error for missing container XML")
	}
}

func TestTwigTemplates(t *testing.T) {
	root := t.TempDir()

	tplDir := filepath.Join(root, "templates")
	if err := os.MkdirAll(tplDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(tplDir, "index.html.twig"), "")
	writeFile(t, filepath.Join(tplDir, "sub", "page.html.twig"), "")
	writeFile(t, filepath.Join(tplDir, "ignore.txt"), "")

	xml := strings.ReplaceAll(minimalContainerXML,
		"<argument>/abs/templates</argument>",
		"<argument>templates</argument>",
	)
	xmlPath := writeContainerXML(t, root, xml)

	c, err := container.ParseXML(xmlPath, root)
	if err != nil {
		t.Fatalf("ParseXML: %v", err)
	}

	templates := c.TwigTemplates()
	found := make(map[string]bool)
	for _, tmpl := range templates {
		found[tmpl] = true
	}

	if !found["index.html.twig"] {
		t.Error("expected index.html.twig")
	}
	if !found["sub/page.html.twig"] {
		t.Error("expected sub/page.html.twig")
	}
	if found["ignore.txt"] {
		t.Error("should not include non-twig files")
	}
}
