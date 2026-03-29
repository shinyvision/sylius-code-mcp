package syliusgrid

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type FieldInput struct {
	Name         string `json:"name"                   jsonschema:"Field name matching an entity property, e.g. name"`
	Type         string `json:"type"                   jsonschema:"Field type: string, datetime, boolean, money, image, or twig"`
	Label        string `json:"label,omitempty"        jsonschema:"Translation label key, defaults to app.ui.{name}"`
	Sortable     bool   `json:"sortable,omitempty"     jsonschema:"Whether the column is sortable"`
	TwigTemplate string `json:"twigTemplate,omitempty" jsonschema:"For twig type: custom template path relative to templates/. If omitted an empty template is auto-created at templates/Admin/{ClassName}/{fieldName}.html.twig."`
}

type FilterInput struct {
	Name  string `json:"name"            jsonschema:"Filter name, typically matching an entity property"`
	Type  string `json:"type"            jsonschema:"Filter type: string, boolean, entity"`
	Label string `json:"label,omitempty" jsonschema:"Translation label key, defaults to app.ui.{name}"`
}

type ToolInput struct {
	ResourceAlias string        `json:"resourceAlias" jsonschema:"Sylius resource alias, e.g. app.supplier"`
	EntityClass   string        `json:"entityClass"   jsonschema:"Fully qualified PHP entity class, e.g. App\\\\Warehouse\\\\Entity\\\\Supplier"`
	Fields        []FieldInput  `json:"fields"        jsonschema:"Grid fields to display. Only fields that exist as properties on the entity are accepted."`
	Filters       []FilterInput `json:"filters,omitempty" jsonschema:"Optional grid filters"`
}

func NewHandler(projectRoot string) func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var in ToolInput
		if err := json.Unmarshal(req.Params.Arguments, &in); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}

		params := Params{
			ResourceAlias: in.ResourceAlias,
			EntityClass:   in.EntityClass,
			Fields:        toFieldSpecs(in.Fields),
			Filters:       toFilterSpecs(in.Filters),
		}

		result, err := EnsureGrid(projectRoot, params)
		if err != nil {
			return nil, fmt.Errorf("sylius_admin_create_grid: %w", err)
		}

		text := strings.Join(result.Messages, "\n")
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil
	}
}

func toFieldSpecs(inputs []FieldInput) []FieldSpec {
	specs := make([]FieldSpec, len(inputs))
	for i, in := range inputs {
		specs[i] = FieldSpec{
			Name:         in.Name,
			Type:         in.Type,
			Label:        in.Label,
			Sortable:     in.Sortable,
			TwigTemplate: in.TwigTemplate,
		}
	}
	return specs
}

func toFilterSpecs(inputs []FilterInput) []FilterSpec {
	specs := make([]FilterSpec, len(inputs))
	for i, in := range inputs {
		specs[i] = FilterSpec{
			Name:  in.Name,
			Type:  in.Type,
			Label: in.Label,
		}
	}
	return specs
}
