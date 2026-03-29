package syliusconstraint

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ListConstraintsInput struct{}

func NewListConstraintsHandler(projectRoot string) func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		infos, err := ListConstraints(projectRoot)
		if err != nil {
			return nil, fmt.Errorf("sylius_list_constraints: %w", err)
		}

		text := FormatForLLM(infos)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil
	}
}
