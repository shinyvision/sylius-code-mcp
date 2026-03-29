package syliusentity

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type AddFieldsInput struct {
	EntityClass  string             `json:"entityClass"  jsonschema:"Fully qualified PHP entity class, e.g. App\\\\Warehouse\\\\Entity\\\\Supplier"`
	Fields       []FieldInput       `json:"fields,omitempty"       jsonschema:"Scalar fields to add"`
	Associations []AssociationInput `json:"associations,omitempty" jsonschema:"Doctrine associations to add"`
}

type AddIndexInput struct {
	EntityClass string   `json:"entityClass" jsonschema:"Fully qualified PHP entity class"`
	Fields      []string `json:"fields"      jsonschema:"PHP property names to include in the index"`
}

func NewAddFieldsHandler(projectRoot string) func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var in AddFieldsInput
		if err := json.Unmarshal(req.Params.Arguments, &in); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}

		params := FieldsParams{
			EntityClass:  in.EntityClass,
			Fields:       in.Fields,
			Associations: in.Associations,
		}

		result, err := EnsureFields(projectRoot, params)
		if err != nil {
			return nil, fmt.Errorf("sylius_add_entity_fields: %w", err)
		}

		text := strings.Join(buildFieldsResultMessages(result), "\n")
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil
	}
}

func NewAddIndexHandler(projectRoot string) func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var in AddIndexInput
		if err := json.Unmarshal(req.Params.Arguments, &in); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}

		params := IndexParams{
			EntityClass: in.EntityClass,
			Fields:      in.Fields,
		}

		result, err := EnsureIndex(projectRoot, params)
		if err != nil {
			return nil, fmt.Errorf("sylius_add_entity_index: %w", err)
		}

		text := strings.Join(result.Messages, "\n")
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil
	}
}
