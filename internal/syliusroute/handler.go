package syliusroute

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ToolInput struct {
	ResourceName string   `json:"resourceName" jsonschema:"The resource name in snake_case, e.g. my_resource"`
	Alias        string   `json:"alias"        jsonschema:"The Sylius resource alias, e.g. app.my_resource"`
	Grid         string   `json:"grid,omitempty"     jsonschema:"Optional grid name, e.g. app_my_resource"`
	Except       []string `json:"except,omitempty"   jsonschema:"Optional list of routes to exclude: index, show, create, update, delete, bulk_delete"`
	Only         []string `json:"only,omitempty"     jsonschema:"Optional list of routes to include (opposite of except): index, show, create, update, delete, bulk_delete"`
	Redirect     string   `json:"redirect,omitempty" jsonschema:"Optional post-save redirect target: index, show, update, create"`
}

func NewHandler(projectRoot string) func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var in ToolInput
		if err := json.Unmarshal(req.Params.Arguments, &in); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}

		p := Params{
			ResourceName: in.ResourceName,
			Alias:        in.Alias,
			Grid:         in.Grid,
			Except:       in.Except,
			Only:         in.Only,
			Redirect:     in.Redirect,
		}

		result, err := EnsureRoute(projectRoot, p)
		if err != nil {
			return nil, fmt.Errorf("sylius_create_resource_route: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: result.Message}},
		}, nil
	}
}
