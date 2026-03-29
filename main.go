package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sylius-code-mcp/internal/syliusconstraint"
	"github.com/sylius-code-mcp/internal/syliusentity"
	"github.com/sylius-code-mcp/internal/syliusgrid"
	"github.com/sylius-code-mcp/internal/syliusmenu"
	"github.com/sylius-code-mcp/internal/syliusresource"
	"github.com/sylius-code-mcp/internal/syliusroute"
	"github.com/sylius-code-mcp/internal/syliustranslations"
)

func main() {
	projectRoot := flag.String("project", "", "Absolute path to the Sylius project root (required)")
	flag.Parse()

	if *projectRoot == "" {
		*projectRoot = os.Getenv("SYLIUS_PROJECT_ROOT")
	}
	if *projectRoot == "" {
		fmt.Fprintln(os.Stderr, "error: provide the Sylius project root via -project flag or SYLIUS_PROJECT_ROOT env var")
		os.Exit(1)
	}

	inputSchema, err := jsonschema.For[syliusroute.ToolInput](nil)
	if err != nil {
		log.Fatalf("building input schema: %v", err)
	}

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "sylius-code-mcp",
		Version: "0.1.0",
	}, nil)

	server.AddTool(&mcp.Tool{
		Name:        "sylius_create_resource_route",
		Description: "Creates a Sylius 2.x resource route entry in config/routes/_sylius/<resource>.yaml and registers the import in config/routes/sylius_admin.yaml.",
		InputSchema: inputSchema,
	}, syliusroute.NewHandler(*projectRoot))

	inputSchema2, err := jsonschema.For[syliusresource.ToolInput](nil)
	if err != nil {
		log.Fatalf("building input schema for sylius_create_resource: %v", err)
	}

	server.AddTool(&mcp.Tool{
		Name:        "sylius_create_resource",
		Description: "Scaffolds a complete Sylius 2.x resource: PHP entity, repository, Doctrine mapping, Sylius resource registration, and admin route. Returns next steps for the developer.",
		InputSchema: inputSchema2,
	}, syliusresource.NewHandler(*projectRoot))

	inputSchema3, err := jsonschema.For[syliusmenu.AddMenuItemInput](nil)
	if err != nil {
		log.Fatalf("building input schema for sylius_admin_add_menu_item: %v", err)
	}

	server.AddTool(&mcp.Tool{
		Name: "sylius_admin_add_menu_item",
		Description: "Adds a menu item to the Sylius 2.x admin menu. " +
			"Reads the compiled Symfony container XML on every call to locate the PHP event subscriber or listener " +
			"that handles the sylius.menu.admin.main event. If no handler exists, creates " +
			"src/EventListener/Menu/MenuSubscriber.php. Inserts the menu item code and returns next steps.",
		InputSchema: inputSchema3,
	}, syliusmenu.NewAddMenuItemHandler(*projectRoot))

	inputSchema4, err := jsonschema.For[syliusmenu.GetItemsInput](nil)
	if err != nil {
		log.Fatalf("building input schema for sylius_admin_get_items: %v", err)
	}

	server.AddTool(&mcp.Tool{
		Name: "sylius_admin_get_items",
		Description: "Returns the current Sylius 2.x admin menu structure by performing PHP static analysis. " +
			"Reads the compiled Symfony container XML on every call (fresh, no caching). " +
			"Analyses the sylius_admin.menu_builder.main class for the base menu structure, then analyses all " +
			"event subscribers and listeners for sylius.menu.admin.main to show added/removed items. " +
			"Use this before calling sylius_admin_add_menu_item to discover valid parentKey values.",
		InputSchema: inputSchema4,
	}, syliusmenu.NewGetItemsHandler(*projectRoot))

	inputSchema5, err := jsonschema.For[syliusgrid.ToolInput](nil)
	if err != nil {
		log.Fatalf("building input schema for sylius_admin_create_grid: %v", err)
	}

	server.AddTool(&mcp.Tool{
		Name: "sylius_admin_create_grid",
		Description: "Creates a Sylius 2.x grid YAML at config/packages/sylius_grid/{resource}.yaml. " +
			"Validates fields against the actual PHP entity properties. " +
			"Supports native Sylius field types (string, datetime, boolean, money, image) and twig, " +
			"for which it auto-creates a starter Twig template at templates/Admin/{ClassName}/{field}.html.twig. " +
			"Always includes standard CRUD actions (create, update, delete). " +
			"Returns a summary of everything created.",
		InputSchema: inputSchema5,
	}, syliusgrid.NewHandler(*projectRoot))

	inputSchema6, err := jsonschema.For[syliusentity.AddFieldsInput](nil)
	if err != nil {
		log.Fatalf("building input schema for sylius_add_entity_fields: %v", err)
	}

	server.AddTool(&mcp.Tool{
		Name: "sylius_add_entity_fields",
		Description: "Adds scalar fields and/or Doctrine associations to an existing PHP entity class. " +
			"Generates properties with PHP 8 ORM attributes (no length constraint), getters, setters, " +
			"and add/remove methods for collections. Column names are automatically snake_cased. " +
			"For OneToMany/ManyToMany, initialises the collection in __construct. " +
			"Optionally generates the inverse side on the target entity. " +
			"Supported scalar types: string, text, int, float, bool, datetime_immutable, date, json, decimal, uuid. " +
			"Supported associations: ManyToOne, OneToMany, ManyToMany, OneToOne.",
		InputSchema: inputSchema6,
	}, syliusentity.NewAddFieldsHandler(*projectRoot))

	inputSchema7, err := jsonschema.For[syliusentity.AddIndexInput](nil)
	if err != nil {
		log.Fatalf("building input schema for sylius_add_entity_index: %v", err)
	}

	server.AddTool(&mcp.Tool{
		Name: "sylius_add_entity_index",
		Description: "Adds a Doctrine ORM database index to a PHP entity class using #[ORM\\Index]. " +
			"Index name is automatically derived as <table_name>_<field1>_<field2>_idx. " +
			"Pass PHP property names (camelCase); they are snake_cased for the index name.",
		InputSchema: inputSchema7,
	}, syliusentity.NewAddIndexHandler(*projectRoot))

	inputSchema9, err := jsonschema.For[syliustranslations.ManageTranslationsInput](nil)
	if err != nil {
		log.Fatalf("building input schema for sylius_manage_translations: %v", err)
	}

	server.AddTool(&mcp.Tool{
		Name: "sylius_manage_translations",
		Description: "Manages Symfony translation files (messages, validators, flashes domains). " +
			"check: scans the project for used app.* translation keys and reports which are missing " +
			"in the translation file — or checks a specific list of keys if provided. " +
			"add: inserts new key→value pairs into the translation file. " +
			"edit: updates existing key→value pairs. " +
			"remove: deletes keys and automatically re-nests the YAML structure. " +
			"Translation files live at translations/{domain}.{locale}.yaml. " +
			"Always generates valid, properly-nested YAML; handles key conflicts by flattening " +
			"affected subtrees and re-nesting on removal.",
		InputSchema: inputSchema9,
	}, syliustranslations.NewHandler(*projectRoot))

	inputSchema8, err := jsonschema.For[syliusconstraint.ListConstraintsInput](nil)
	if err != nil {
		log.Fatalf("building input schema for sylius_list_constraints: %v", err)
	}

	server.AddTool(&mcp.Tool{
		Name: "sylius_list_constraints",
		Description: "Lists all Symfony validator constraints available in this project. " +
			"Reads the compiled Symfony container XML to find validator services, then performs PHP static " +
			"analysis on the constraint class constructors to expose available parameters. " +
			"Also scans vendor/symfony/validator/Constraints/ directly for built-in constraints. " +
			"Use this before calling sylius_add_entity_fields to discover valid constraint names and their parameters.",
		InputSchema: inputSchema8,
	}, syliusconstraint.NewListConstraintsHandler(*projectRoot))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
