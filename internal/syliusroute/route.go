package syliusroute

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sylius-code-mcp/internal/yamlutil"
	"gopkg.in/yaml.v3"
)

var validRouteOperations = []string{"index", "show", "create", "update", "delete", "bulk_delete"}

var validRedirects = []string{"index", "show", "update", "create"}

type Params struct {
	ResourceName string
	Alias        string
	Grid         string
	Except       []string
	Only         []string
	Redirect     string
}

type Result struct {
	Message     string
	FileCreated bool
	FileUpdated bool
	ImportAdded bool
}

func (p Params) Validate() error {
	if strings.TrimSpace(p.ResourceName) == "" {
		return errors.New("resourceName is required")
	}
	if strings.TrimSpace(p.Alias) == "" {
		return errors.New("alias is required")
	}
	if len(p.Except) > 0 && len(p.Only) > 0 {
		return errors.New("except and only are mutually exclusive")
	}
	for _, op := range p.Except {
		if !sliceContains(validRouteOperations, op) {
			return fmt.Errorf("invalid except value %q: must be one of %v", op, validRouteOperations)
		}
	}
	for _, op := range p.Only {
		if !sliceContains(validRouteOperations, op) {
			return fmt.Errorf("invalid only value %q: must be one of %v", op, validRouteOperations)
		}
	}
	if p.Redirect != "" && !sliceContains(validRedirects, p.Redirect) {
		return fmt.Errorf("invalid redirect %q: must be one of %v", p.Redirect, validRedirects)
	}
	return nil
}

func routeKey(resourceName string) string {
	return "app_admin_" + resourceName
}

func importKey(resourceName string) string {
	return "app_" + resourceName
}

func routeFilePath(resourceName string) string {
	return filepath.Join("config", "routes", "_sylius", resourceName+".yaml")
}

const adminYAMLPath = "config/routes/sylius_admin.yaml"

func GenerateRouteYAML(p Params) (string, error) {
	innerYAML, err := marshalNode(buildResourceContentNode(p))
	if err != nil {
		return "", fmt.Errorf("marshalling resource content: %w", err)
	}

	outer := mappingNode(
		scalar(routeKey(p.ResourceName)),
		mappingNode(
			scalar("resource"), literalScalar(innerYAML),
			scalar("type"), scalar("sylius.resource"),
		),
	)

	return marshalNode(outer)
}

func buildResourceContentNode(p Params) *yaml.Node {
	m := &yaml.Node{Kind: yaml.MappingNode}

	addField(m, scalar("alias"), scalar(p.Alias))
	addField(m, scalar("section"), scalar("admin"))
	addField(m, scalar("templates"), doubleQuotedScalar(`@SyliusAdmin\shared\crud`))

	if len(p.Except) > 0 {
		addField(m, scalar("except"), flowSequenceNode(p.Except))
	}
	if len(p.Only) > 0 {
		addField(m, scalar("only"), flowSequenceNode(p.Only))
	}
	if p.Redirect != "" {
		addField(m, scalar("redirect"), scalar(p.Redirect))
	}

	addField(m, scalar("permission"), boolNode(true))

	if p.Grid != "" {
		addField(m, scalar("grid"), scalar(p.Grid))
	}

	return m
}

func GenerateImportYAML(resourceName string) (string, error) {
	inner := mappingNode(
		scalar("resource"), doubleQuotedScalar(fmt.Sprintf("_sylius/%s.yaml", resourceName)),
		scalar("prefix"), singleQuotedScalar("/%sylius_admin.path_name%"),
	)

	return marshalNode(mappingNode(scalar(importKey(resourceName)), inner))
}

func EnsureRoute(projectRoot string, p Params) (Result, error) {
	if err := p.Validate(); err != nil {
		return Result{}, err
	}

	routeResult, err := ensureRouteFile(projectRoot, p)
	if err != nil {
		return Result{}, err
	}
	if routeResult.alreadyExists {
		return Result{
			Message: fmt.Sprintf(
				"Route %q already exists in %s. Modify the file directly to update it.",
				routeKey(p.ResourceName),
				filepath.Join(projectRoot, routeFilePath(p.ResourceName)),
			),
		}, nil
	}

	importAdded, err := ensureAdminImport(projectRoot, p.ResourceName)
	if err != nil {
		return Result{}, err
	}

	result := Result{
		FileCreated: routeResult.fileCreated,
		FileUpdated: routeResult.fileUpdated,
		ImportAdded: importAdded,
	}
	result.Message = buildSuccessMessage(projectRoot, p, result)
	return result, nil
}

type routeFileResult struct {
	alreadyExists bool
	fileCreated   bool
	fileUpdated   bool
}

func ensureRouteFile(projectRoot string, p Params) (routeFileResult, error) {
	path := filepath.Join(projectRoot, routeFilePath(p.ResourceName))

	routeYAML, err := GenerateRouteYAML(p)
	if err != nil {
		return routeFileResult{}, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if err := writeFile(path, routeYAML); err != nil {
			return routeFileResult{}, fmt.Errorf("creating route file: %w", err)
		}
		return routeFileResult{fileCreated: true}, nil
	}
	if err != nil {
		return routeFileResult{}, fmt.Errorf("reading route file: %w", err)
	}

	if routeKeyExists(string(data), routeKey(p.ResourceName)) {
		return routeFileResult{alreadyExists: true}, nil
	}

	updated := appendWithBlankLine(string(data), routeYAML)
	if err := os.WriteFile(path, []byte(updated), 0644); err != nil {
		return routeFileResult{}, fmt.Errorf("updating route file: %w", err)
	}
	return routeFileResult{fileUpdated: true}, nil
}

func ensureAdminImport(projectRoot, resourceName string) (bool, error) {
	path := filepath.Join(projectRoot, adminYAMLPath)

	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("reading sylius_admin.yaml: %w", err)
	}

	if routeKeyExists(string(data), importKey(resourceName)) {
		return false, nil
	}

	importYAML, err := GenerateImportYAML(resourceName)
	if err != nil {
		return false, err
	}

	updated := appendWithBlankLine(string(data), importYAML)
	if err := os.WriteFile(path, []byte(updated), 0644); err != nil {
		return false, fmt.Errorf("updating sylius_admin.yaml: %w", err)
	}
	return true, nil
}

func routeKeyExists(content, key string) bool {
	prefix := key + ":"
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

func appendWithBlankLine(existing, newContent string) string {
	return strings.TrimRight(existing, "\n") + "\n\n" + newContent
}

func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

func buildSuccessMessage(projectRoot string, p Params, r Result) string {
	routePath := filepath.Join(projectRoot, routeFilePath(p.ResourceName))

	var action string
	switch {
	case r.FileCreated:
		action = fmt.Sprintf("Created resource route %q in %s.", routeKey(p.ResourceName), routePath)
	case r.FileUpdated:
		action = fmt.Sprintf("Appended resource route %q to %s.", routeKey(p.ResourceName), routePath)
	}

	if r.ImportAdded {
		adminPath := filepath.Join(projectRoot, adminYAMLPath)
		action += fmt.Sprintf(" Registered import %q in %s.", importKey(p.ResourceName), adminPath)
	}

	return action + " This route is now yours to maintain and modify directly as required."
}

func marshalNode(n *yaml.Node) (string, error)    { return yamlutil.MarshalNode(n) }
func mappingNode(pairs ...*yaml.Node) *yaml.Node  { return yamlutil.MappingNode(pairs...) }
func addField(m, key, value *yaml.Node)           { yamlutil.AddField(m, key, value) }
func scalar(value string) *yaml.Node              { return yamlutil.Scalar(value) }
func doubleQuotedScalar(value string) *yaml.Node  { return yamlutil.DoubleQuotedScalar(value) }
func singleQuotedScalar(value string) *yaml.Node  { return yamlutil.SingleQuotedScalar(value) }
func literalScalar(value string) *yaml.Node       { return yamlutil.LiteralScalar(value) }
func boolNode(v bool) *yaml.Node                  { return yamlutil.BoolNode(v) }
func flowSequenceNode(values []string) *yaml.Node { return yamlutil.FlowSequenceNode(values) }

func sliceContains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
