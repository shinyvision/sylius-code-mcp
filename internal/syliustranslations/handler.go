package syliustranslations

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ManageTranslationsInput struct {
	Action string `json:"action" jsonschema:"Action to perform: check (report missing keys), add (insert entries), edit (update entries), remove (delete keys)"`

	Domain string `json:"domain" jsonschema:"Translation domain: messages (app.ui.*), validators (app.validator.*), or flashes (app.flash.*)"`

	Locale string `json:"locale"`

	Keys []string `json:"keys,omitempty"`

	Entries map[string]string `json:"entries,omitempty"`
}

func NewHandler(projectRoot string) func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var in ManageTranslationsInput
		if err := json.Unmarshal(req.Params.Arguments, &in); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}

		if err := validateInput(in); err != nil {
			return nil, err
		}

		var text string
		var err error

		switch in.Action {
		case "check":
			text, err = handleCheck(projectRoot, in)
		case "add":
			text, err = handleAdd(projectRoot, in)
		case "edit":
			text, err = handleEdit(projectRoot, in)
		case "remove":
			text, err = handleRemove(projectRoot, in)
		default:
			return nil, fmt.Errorf("action must be check, add, edit, or remove")
		}

		if err != nil {
			return nil, err
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil
	}
}

func validateInput(in ManageTranslationsInput) error {
	switch in.Domain {
	case "messages", "validators", "flashes":
	default:
		return fmt.Errorf("domain must be messages, validators, or flashes")
	}
	if in.Locale == "" {
		return fmt.Errorf("locale is required")
	}
	switch in.Action {
	case "add", "edit":
		if len(in.Entries) == 0 {
			return fmt.Errorf("entries is required for action %q", in.Action)
		}
	case "remove":
		if len(in.Keys) == 0 {
			return fmt.Errorf("keys is required for action remove")
		}
	}
	return nil
}

func handleCheck(projectRoot string, in ManageTranslationsInput) (string, error) {
	result, err := CheckKeys(projectRoot, in.Domain, in.Locale, in.Keys)
	if err != nil {
		return "", err
	}

	if len(result.Missing) == 0 {
		if len(in.Keys) == 0 {
			return fmt.Sprintf("No missing keys found in %s/%s (project auto-scan).", in.Domain, in.Locale), nil
		}
		return fmt.Sprintf("All %d keys are present in %s/%s.", len(in.Keys), in.Domain, in.Locale), nil
	}

	var sb strings.Builder
	sb.WriteString("Missing translation keys:\n")
	sb.WriteString(fmt.Sprintf("\n%s / %s:\n", in.Locale, in.Domain))
	sort.Slice(result.Missing, func(i, j int) bool { return result.Missing[i].key < result.Missing[j].key })
	for _, m := range result.Missing {
		fmt.Fprintf(&sb, "  - %s\n", m.key)
	}
	sb.WriteString("\nRun with action \"add\" and entries to insert placeholder values.")
	return sb.String(), nil
}

func handleAdd(projectRoot string, in ManageTranslationsInput) (string, error) {
	result, err := AddKeys(projectRoot, in.Domain, in.Locale, in.Entries)
	if err != nil {
		return "", err
	}

	total := 0
	var sb strings.Builder
	sb.WriteString("Added translation keys:\n")

	for _, file := range sortedKeys(result.Added) {
		keys := result.Added[file]
		if len(keys) == 0 {
			continue
		}
		fmt.Fprintf(&sb, "\n%s\n", file)
		for _, k := range keys {
			fmt.Fprintf(&sb, "  + %s\n", k)
			total++
		}
	}

	if total == 0 {
		return "All keys already exist — nothing added.", nil
	}

	fmt.Fprintf(&sb, "\n%d key(s) added.", total)
	return sb.String(), nil
}

func handleEdit(projectRoot string, in ManageTranslationsInput) (string, error) {
	result, err := EditKeys(projectRoot, in.Domain, in.Locale, in.Entries)
	if err != nil {
		return "", err
	}

	total := 0
	var sb strings.Builder
	sb.WriteString("Edited translation keys:\n")

	for _, file := range sortedKeys(result.Edited) {
		keys := result.Edited[file]
		if len(keys) == 0 {
			continue
		}
		fmt.Fprintf(&sb, "\n%s\n", file)
		for _, k := range keys {
			fmt.Fprintf(&sb, "  ~ %s\n", k)
			total++
		}
	}

	if len(result.NotFound) > 0 {
		sb.WriteString("\nKeys not found (not edited):\n")
		for _, k := range result.NotFound {
			fmt.Fprintf(&sb, "  ! %s\n", k)
		}
	}

	if total == 0 && len(result.NotFound) > 0 {
		return sb.String(), nil
	}
	if total == 0 {
		return "Nothing to edit.", nil
	}

	fmt.Fprintf(&sb, "\n%d key(s) edited.", total)
	return sb.String(), nil
}

func handleRemove(projectRoot string, in ManageTranslationsInput) (string, error) {
	result, err := RemoveKeys(projectRoot, in.Domain, in.Locale, in.Keys)
	if err != nil {
		return "", err
	}

	total := 0
	var sb strings.Builder
	sb.WriteString("Removed translation keys:\n")

	for _, file := range sortedKeys(result.Removed) {
		keys := result.Removed[file]
		if len(keys) == 0 {
			continue
		}
		fmt.Fprintf(&sb, "\n%s\n", file)
		for _, k := range keys {
			fmt.Fprintf(&sb, "  - %s\n", k)
			total++
		}
	}

	if len(result.NotFound) > 0 {
		sb.WriteString("\nKeys not found (already absent):\n")
		for _, k := range result.NotFound {
			fmt.Fprintf(&sb, "  ! %s\n", k)
		}
	}

	if total == 0 {
		return sb.String(), nil
	}

	fmt.Fprintf(&sb, "\n%d key(s) removed.", total)
	return sb.String(), nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
