package syliusresource

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ToolInput struct {
	ResourceAlias string `json:"resourceAlias" jsonschema:"The Sylius resource alias, e.g. app.my_resource"`
	Namespace     string `json:"namespace"     jsonschema:"The PHP module namespace, e.g. App\\MyResource"`
}

func NewHandler(projectRoot string) func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var in ToolInput
		if err := json.Unmarshal(req.Params.Arguments, &in); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}

		p := Params{
			ResourceAlias: in.ResourceAlias,
			Namespace:     in.Namespace,
		}

		result, err := EnsureResource(projectRoot, p)
		if err != nil {
			return nil, fmt.Errorf("sylius_create_resource: %w", err)
		}

		text := strings.Join(result.Messages, "\n")
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil
	}
}
