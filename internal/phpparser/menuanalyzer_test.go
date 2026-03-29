package phpparser_test

import (
	"strings"
	"testing"

	"github.com/sylius-code-mcp/internal/phpparser"
)

const menuBuilderPHP = `<?php
namespace Sylius\Bundle\AdminBundle\Menu;

use Knp\Menu\FactoryInterface;
use Knp\Menu\ItemInterface;

final class MainMenuBuilder
{
    public function createMenu(array $options): ItemInterface
    {
        $menu = $this->factory->createItem('root');

        $this->addDashboardItem($menu);
        $this->addCatalogSubMenu($menu);
        $this->addSalesSubMenu($menu);

        return $menu;
    }

    private function addDashboardItem(ItemInterface $menu): void
    {
        $menu->addChild('dashboard');
    }

    private function addCatalogSubMenu(ItemInterface $menu): void
    {
        $catalog = $menu->addChild('catalog');

        $catalog->addChild('taxons', ['route' => 'sylius_admin_taxon_create']);
        $catalog->addChild('products', ['route' => 'sylius_admin_product_index']);
        $catalog->addChild('attributes', ['route' => 'sylius_admin_product_attribute_index']);
    }

    private function addSalesSubMenu(ItemInterface $menu): void
    {
        $sales = $menu
            ->addChild('sales');

        $sales->addChild('orders', ['route' => 'sylius_admin_order_index']);
        $sales->addChild('shipments', ['route' => 'sylius_admin_shipment_index']);
    }
}
`

const menuSubscriberPHP = `<?php
namespace App\EventListener\Menu;

use Knp\Menu\ItemInterface;
use Sylius\Bundle\UiBundle\Menu\Event\MenuBuilderEvent;
use Symfony\Component\EventDispatcher\EventSubscriberInterface;

class MenuSubscriber implements EventSubscriberInterface
{
    public static function getSubscribedEvents(): array
    {
        return [
            'sylius.menu.admin.main' => ['addAdminMenuItems', -2048],
        ];
    }

    public function addAdminMenuItems(MenuBuilderEvent $event): void
    {
        $menu = $event->getMenu();

        $menu->removeChild('official_support');

        $salesMenu = $menu->getChild('sales');
        $salesMenu->removeChild('shipments');

        $warehouseMenu = $menu->addChild('warehouse');
        $this->extendWarehouseMenu($warehouseMenu);
    }

    private function extendWarehouseMenu(ItemInterface $warehouseMenu): void
    {
        $warehouseMenu->addChild('stock', ['route' => 'app_admin_stock_index']);
        $warehouseMenu->addChild('products', ['route' => 'app_admin_warehouse_product_index']);
    }
}
`

func TestAnalyzeMenuFile_builderTopLevelItems(t *testing.T) {
	a := phpparser.AnalyzeMenuFile(menuBuilderPHP, "MainMenuBuilder.php")

	adds := opsOfKind(a.Ops, phpparser.OpAdd)
	keys := opKeys(adds, nil)

	assertContainsKey(t, keys, "dashboard")
	assertContainsKey(t, keys, "catalog")
	assertContainsKey(t, keys, "sales")
}

func TestAnalyzeMenuFile_builderNestedItems(t *testing.T) {
	a := phpparser.AnalyzeMenuFile(menuBuilderPHP, "MainMenuBuilder.php")

	adds := opsOfKind(a.Ops, phpparser.OpAdd)

	catalogChildren := opKeys(adds, []string{"catalog"})
	assertContainsKey(t, catalogChildren, "taxons")
	assertContainsKey(t, catalogChildren, "products")
	assertContainsKey(t, catalogChildren, "attributes")

	salesChildren := opKeys(adds, []string{"sales"})
	assertContainsKey(t, salesChildren, "orders")
	assertContainsKey(t, salesChildren, "shipments")
}

func TestAnalyzeMenuFile_builderClassName(t *testing.T) {
	a := phpparser.AnalyzeMenuFile(menuBuilderPHP, "MainMenuBuilder.php")
	if a.ClassName != "MainMenuBuilder" {
		t.Errorf("expected class MainMenuBuilder, got %q", a.ClassName)
	}
}

func TestAnalyzeMenuFile_subscriberAddsAndRemoves(t *testing.T) {
	a := phpparser.AnalyzeMenuFile(menuSubscriberPHP, "MenuSubscriber.php")

	adds := opsOfKind(a.Ops, phpparser.OpAdd)
	removes := opsOfKind(a.Ops, phpparser.OpRemove)

	rootAdds := opKeys(adds, nil)
	assertContainsKey(t, rootAdds, "warehouse")

	warehouseChildren := opKeys(adds, []string{"warehouse"})
	assertContainsKey(t, warehouseChildren, "stock")
	assertContainsKey(t, warehouseChildren, "products")

	rootRemoves := opKeys(removes, nil)
	assertContainsKey(t, rootRemoves, "official_support")

	salesRemoves := opKeys(removes, []string{"sales"})
	assertContainsKey(t, salesRemoves, "shipments")
}

func TestAnalyzeMenuFile_multiLineChainedAddChild(t *testing.T) {
	content := `<?php
class Builder {
    public function build(ItemInterface $menu): void
    {
        $sub = $menu
            ->addChild('catalog');
        $sub->addChild('products');
    }
}`
	a := phpparser.AnalyzeMenuFile(content, "Builder.php")
	adds := opsOfKind(a.Ops, phpparser.OpAdd)

	rootAdds := opKeys(adds, nil)
	assertContainsKey(t, rootAdds, "catalog")

	catalogChildren := opKeys(adds, []string{"catalog"})
	assertContainsKey(t, catalogChildren, "products")
}

func TestAnalyzeMenuFile_nullsafeGetChild(t *testing.T) {
	content := `<?php
class Sub {
    public function handle(MenuBuilderEvent $event): void
    {
        $menu = $event->getMenu();
        $salesMenu = $menu->getChild('sales');
        $salesMenu?->addChild('my_item');
    }
}`
	a := phpparser.AnalyzeMenuFile(content, "Sub.php")
	adds := opsOfKind(a.Ops, phpparser.OpAdd)

	salesChildren := opKeys(adds, []string{"sales"})
	assertContainsKey(t, salesChildren, "my_item")
}

func TestAnalyzeMenuFile_stripsLineComments(t *testing.T) {
	content := `<?php
class X {
    public function handle(MenuBuilderEvent $event): void
    {
        $menu = $event->getMenu();
        // $menu->addChild('fake_comment_item'); // this should be ignored
        $menu->addChild('real_item');
    }
}`
	a := phpparser.AnalyzeMenuFile(content, "X.php")
	adds := opsOfKind(a.Ops, phpparser.OpAdd)
	rootAdds := opKeys(adds, nil)

	assertContainsKey(t, rootAdds, "real_item")
	for _, k := range rootAdds {
		if k == "fake_comment_item" {
			t.Error("should not include items from comments")
		}
	}
}

func TestAnalyzeMenuFile_noDuplicates(t *testing.T) {
	a := phpparser.AnalyzeMenuFile(menuBuilderPHP, "MainMenuBuilder.php")
	seen := make(map[string]int)
	for _, op := range a.Ops {
		k := strings.Join(op.ParentPath, "/") + ">" + op.ChildKey
		seen[k]++
	}
	for k, count := range seen {
		if count > 1 {
			t.Errorf("op %q appears %d times (expected 1)", k, count)
		}
	}
}

func opsOfKind(ops []phpparser.MenuOp, kind phpparser.OpKind) []phpparser.MenuOp {
	var result []phpparser.MenuOp
	for _, op := range ops {
		if op.Kind == kind {
			result = append(result, op)
		}
	}
	return result
}

func opKeys(ops []phpparser.MenuOp, parentPath []string) []string {
	var result []string
	for _, op := range ops {
		if pathEqual(op.ParentPath, parentPath) {
			result = append(result, op.ChildKey)
		}
	}
	return result
}

func pathEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func assertContainsKey(t *testing.T, keys []string, want string) {
	t.Helper()
	for _, k := range keys {
		if k == want {
			return
		}
	}
	t.Errorf("expected key %q in %v", want, keys)
}
