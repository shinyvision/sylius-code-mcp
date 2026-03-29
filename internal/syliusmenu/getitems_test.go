package syliusmenu_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sylius-code-mcp/internal/phpparser"
	"github.com/sylius-code-mcp/internal/syliusmenu"
)

const minimalMenuBuilderPHP = `<?php
namespace Sylius\Bundle\AdminBundle\Menu;

use Knp\Menu\FactoryInterface;
use Knp\Menu\ItemInterface;

final class MainMenuBuilder
{
    public function createMenu(array $options): ItemInterface
    {
        $menu = $this->factory->createItem('root');
        $this->addCatalogSubMenu($menu);
        $this->addSalesSubMenu($menu);
        return $menu;
    }

    private function addCatalogSubMenu(ItemInterface $menu): void
    {
        $catalog = $menu->addChild('catalog');
        $catalog->addChild('products');
        $catalog->addChild('taxons');
    }

    private function addSalesSubMenu(ItemInterface $menu): void
    {
        $sales = $menu->addChild('sales');
        $sales->addChild('orders');
        $sales->addChild('shipments');
    }
}
`

const testMenuSubscriberPHP = `<?php
namespace App\EventListener\Menu;

use Knp\Menu\ItemInterface;
use Sylius\Bundle\UiBundle\Menu\Event\MenuBuilderEvent;
use Symfony\Component\EventDispatcher\EventSubscriberInterface;

class MenuSubscriber implements EventSubscriberInterface
{
    public static function getSubscribedEvents(): array
    {
        return ['sylius.menu.admin.main' => ['addAdminMenuItems', -2048]];
    }

    public function addAdminMenuItems(MenuBuilderEvent $event): void
    {
        $menu = $event->getMenu();
        $menu->removeChild('official_support');
        $salesMenu = $menu->getChild('sales');
        $salesMenu->removeChild('shipments');
        $menu->addChild('warehouse');
    }
}
`

func newGetItemsProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	writeFile(t, filepath.Join(root, "composer.json"),
		`{"autoload":{"psr-4":{"App\\":"src/","Sylius\\Bundle\\AdminBundle\\":"vendor/sylius/admin/"}}}`)

	writeFile(t,
		filepath.Join(root, "vendor", "sylius", "admin", "Menu", "MainMenuBuilder.php"),
		minimalMenuBuilderPHP)

	writeFile(t,
		filepath.Join(root, "src", "EventListener", "Menu", "MenuSubscriber.php"),
		testMenuSubscriberPHP)

	xmlContent := buildGetItemsContainerXML()
	writeContainerXML(t, root, xmlContent)

	return root
}

func buildGetItemsContainerXML() string {
	return `<?xml version="1.0"?>
<container xmlns="http://symfony.com/schema/dic/services">
  <services>
    <service id="sylius_admin.menu_builder.main"
             class="Sylius\Bundle\AdminBundle\Menu\MainMenuBuilder">
    </service>
    <service id="App\EventListener\Menu\MenuSubscriber"
             class="App\EventListener\Menu\MenuSubscriber"
             autowire="true">
      <tag name="kernel.event_subscriber"/>
    </service>
  </services>
</container>`
}

func TestGetMenuItems_containsBaseItems(t *testing.T) {
	root := newGetItemsProject(t)

	result, err := syliusmenu.GetMenuItems(root)
	if err != nil {
		t.Fatalf("GetMenuItems: %v", err)
	}

	topKeys := topLevelKeys(result.Tree)
	assertIn(t, topKeys, "catalog")
	assertIn(t, topKeys, "sales")
}

func TestGetMenuItems_containsNestedItems(t *testing.T) {
	root := newGetItemsProject(t)

	result, err := syliusmenu.GetMenuItems(root)
	if err != nil {
		t.Fatalf("GetMenuItems: %v", err)
	}

	catalog := findNode(result.Tree, "catalog")
	if catalog == nil {
		t.Fatal("expected catalog node")
	}
	childKeys := nodeChildKeys(catalog)
	assertIn(t, childKeys, "products")
	assertIn(t, childKeys, "taxons")
}

func TestGetMenuItems_subscriberAddsItem(t *testing.T) {
	root := newGetItemsProject(t)

	result, err := syliusmenu.GetMenuItems(root)
	if err != nil {
		t.Fatalf("GetMenuItems: %v", err)
	}

	topKeys := topLevelKeys(result.Tree)
	assertIn(t, topKeys, "warehouse")
}

func TestGetMenuItems_reportContainsRemoves(t *testing.T) {
	root := newGetItemsProject(t)

	result, err := syliusmenu.GetMenuItems(root)
	if err != nil {
		t.Fatalf("GetMenuItems: %v", err)
	}

	if !strings.Contains(result.Report, "removed") {
		t.Error("report should mention removed items")
	}
	if !strings.Contains(result.Report, "shipments") {
		t.Error("report should mention shipments removal")
	}
}

func TestGetMenuItems_reportContainsTopLevelKeys(t *testing.T) {
	root := newGetItemsProject(t)

	result, err := syliusmenu.GetMenuItems(root)
	if err != nil {
		t.Fatalf("GetMenuItems: %v", err)
	}

	if !strings.Contains(result.Report, "parentKey") {
		t.Error("report should reference parentKey for the LLM")
	}
	if !strings.Contains(result.Report, "`catalog`") {
		t.Error("report should list catalog as a top-level key")
	}
}

func TestGetMenuItems_noContainerXMLReturnsError(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "composer.json"),
		`{"autoload":{"psr-4":{"App\\":"src/"}}}`)

	_, err := syliusmenu.GetMenuItems(root)
	if err == nil {
		t.Error("expected error when no container XML exists")
	}
}

func TestBuildMenuItemsResult_directOpComposition(t *testing.T) {
	base := phpparser.MenuAnalysis{
		FilePath:  "/project/MainMenuBuilder.php",
		ClassName: "MainMenuBuilder",
		Ops: []phpparser.MenuOp{
			{Kind: phpparser.OpAdd, ParentPath: []string{}, ChildKey: "catalog"},
			{Kind: phpparser.OpAdd, ParentPath: []string{"catalog"}, ChildKey: "products"},
			{Kind: phpparser.OpAdd, ParentPath: []string{}, ChildKey: "sales"},
			{Kind: phpparser.OpAdd, ParentPath: []string{"sales"}, ChildKey: "orders"},
		},
	}
	modifier := phpparser.MenuAnalysis{
		FilePath:  "/project/src/EventListener/Menu/MenuSubscriber.php",
		ClassName: "MenuSubscriber",
		Ops: []phpparser.MenuOp{
			{Kind: phpparser.OpAdd, ParentPath: []string{}, ChildKey: "warehouse"},
			{Kind: phpparser.OpRemove, ParentPath: []string{"sales"}, ChildKey: "orders"},
		},
	}

	result := syliusmenu.BuildMenuItemsResult(base, []phpparser.MenuAnalysis{modifier})

	topKeys := topLevelKeys(result.Tree)
	assertIn(t, topKeys, "catalog")
	assertIn(t, topKeys, "sales")
	assertIn(t, topKeys, "warehouse")

	if !strings.Contains(result.Report, "orders") {
		t.Error("report should mention the removed 'orders' item")
	}
}

func topLevelKeys(nodes []syliusmenu.MenuNode) []string {
	keys := make([]string, len(nodes))
	for i, n := range nodes {
		keys[i] = n.Key
	}
	return keys
}

func findNode(nodes []syliusmenu.MenuNode, key string) *syliusmenu.MenuNode {
	for i := range nodes {
		if nodes[i].Key == key {
			return &nodes[i]
		}
	}
	return nil
}

func nodeChildKeys(n *syliusmenu.MenuNode) []string {
	if n == nil {
		return nil
	}
	keys := make([]string, len(n.Children))
	for i, c := range n.Children {
		keys[i] = c.Key
	}
	return keys
}

func assertIn(t *testing.T, slice []string, want string) {
	t.Helper()
	if slices.Contains(slice, want) {
		return
	}
	t.Errorf("expected %q in %v", want, slice)
}

func loadFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(data)
}

var _ = loadFile
