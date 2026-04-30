package syliustwighooks

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ToolInput struct {
	Prefix string `json:"prefix,omitempty" jsonschema:"Optional dot-separated hook prefix to drill into (e.g. 'sylius_admin.payment_method.update'). Leave empty to inspect the root; the tool will then ask you to be more specific if too many hooks match."`
}

func NewHandler(projectRoot string) func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var in ToolInput
		if len(req.Params.Arguments) > 0 {
			if err := json.Unmarshal(req.Params.Arguments, &in); err != nil {
				return nil, fmt.Errorf("invalid arguments: %w", err)
			}
		}

		text, err := GetTwigHooks(projectRoot, in.Prefix)
		if err != nil {
			return nil, fmt.Errorf("sylius_get_twig_hooks: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil
	}
}

type AddHookHookableInput struct {
	Name     string `json:"name"               jsonschema:"Hookable identifier (e.g. 'breadcrumbs'). Becomes the YAML key under the hook."`
	Template string `json:"template"           jsonschema:"Twig template path for this hookable (e.g. '@SyliusAdmin/shared/crud/update/content/header/breadcrumbs.html.twig')."`
	Priority *int   `json:"priority,omitempty" jsonschema:"Optional integer priority. Higher values render earlier."`
	Enabled  *bool  `json:"enabled,omitempty"  jsonschema:"Optional explicit enabled flag. Omit to inherit the default."`
}

type AddHookInput struct {
	CategoryName string               `json:"categoryName" jsonschema:"Snake_case category that determines the YAML file at config/packages/twig_hooks/<categoryName>.yaml (e.g. 'payment_method')."`
	HookName     string               `json:"hookName"     jsonschema:"Full dot-separated hook name to attach the hookable to (e.g. 'sylius_admin.payment_method.update.content.header')."`
	Hookable     AddHookHookableInput `json:"hookable"     jsonschema:"Hookable definition. Required fields: name, template. Optional fields: priority (int), enabled (bool)."`
}

func NewAddHookHandler(projectRoot string) func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var in AddHookInput
		if err := json.Unmarshal(req.Params.Arguments, &in); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}

		params := AddHookParams{
			CategoryName: in.CategoryName,
			HookName:     in.HookName,
			Hookable: HookableSpec{
				Name:     in.Hookable.Name,
				Template: in.Hookable.Template,
				Priority: in.Hookable.Priority,
				Enabled:  in.Hookable.Enabled,
			},
		}

		result, err := AddHook(projectRoot, params)
		if err != nil {
			return nil, fmt.Errorf("sylius_add_twig_hook: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: result.Message}},
		}, nil
	}
}
