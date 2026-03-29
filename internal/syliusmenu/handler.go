package syliusmenu

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type AddMenuItemInput struct {
	ParentKey           string `json:"parentKey,omitempty" jsonschema:"Optional key of an existing parent menu item (e.g. 'sales', 'catalog'). Leave empty to add to the root menu."`
	ItemKey             string `json:"itemKey"             jsonschema:"Unique snake_case identifier for the new menu item (e.g. 'my_resource')"`
	LabelTranslationKey string `json:"labelTranslationKey" jsonschema:"Symfony translation key for the menu label (e.g. 'app.ui.my_resource')"`
	Route               string `json:"route"               jsonschema:"Symfony route name for the index page (e.g. 'app_admin_my_resource_index')"`
	Icon                string `json:"icon,omitempty"      jsonschema:"Optional Tabler icon identifier (e.g. 'tabler:box')"`
	Weight              int    `json:"weight,omitempty"    jsonschema:"Optional display order weight (higher = later in the list). Only used when adding to the root menu."`
}

type GetItemsInput struct{}

func NewAddMenuItemHandler(projectRoot string) func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var in AddMenuItemInput
		if err := json.Unmarshal(req.Params.Arguments, &in); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}

		result, err := AddMenuItem(projectRoot, Params{
			ParentKey: in.ParentKey,
			ItemKey:   in.ItemKey,
			Label:     in.LabelTranslationKey,
			Route:     in.Route,
			Icon:      in.Icon,
			Weight:    in.Weight,
		})
		if err != nil {
			return nil, fmt.Errorf("sylius_admin_add_menu_item: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: result.Message}},
		}, nil
	}
}

func NewGetItemsHandler(projectRoot string) func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		result, err := GetMenuItems(projectRoot)
		if err != nil {
			return nil, fmt.Errorf("sylius_admin_get_items: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: result.Report}},
		}, nil
	}
}
