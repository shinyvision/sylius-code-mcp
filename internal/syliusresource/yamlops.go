package syliusresource

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sylius-code-mcp/internal/yamlutil"
	"gopkg.in/yaml.v3"
)

func ensureDoctrineMapping(projectRoot string, meta Meta) (bool, error) {
	path := filepath.Join(projectRoot, "config", "packages", "doctrine.yaml")

	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("reading doctrine.yaml: %w", err)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return false, fmt.Errorf("parsing doctrine.yaml: %w", err)
	}

	mappings := yamlutil.NavigatePath(&root, "doctrine", "orm", "entity_managers", "default", "mappings")
	if mappings == nil {
		return false, fmt.Errorf("doctrine.yaml: could not find path doctrine.orm.entity_managers.default.mappings")
	}

	if yamlutil.FindInMapping(mappings, meta.DoctrineAlias) != nil {
		return false, nil
	}

	valueNode := yamlutil.MappingNode(
		yamlutil.Scalar("is_bundle"), yamlutil.BoolNode(false),
		yamlutil.Scalar("type"), yamlutil.Scalar("attribute"),
		yamlutil.Scalar("dir"), yamlutil.SingleQuotedScalar(meta.DoctrineDir),
		yamlutil.Scalar("prefix"), yamlutil.SingleQuotedScalar(meta.DoctrinePrefix),
		yamlutil.Scalar("alias"), yamlutil.Scalar(meta.DoctrineAlias),
	)

	mappings.Content = append(mappings.Content,
		yamlutil.Scalar(meta.DoctrineAlias),
		valueNode,
	)

	return true, writeYAMLFile(path, &root)
}

func ensureSyliusResource(projectRoot string, meta Meta) (bool, error) {
	path := filepath.Join(projectRoot, "config", "packages", "sylius_resource.yaml")

	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("reading sylius_resource.yaml: %w", err)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return false, fmt.Errorf("parsing sylius_resource.yaml: %w", err)
	}

	resources := yamlutil.NavigatePath(&root, "sylius_resource", "resources")
	if resources == nil {
		return false, fmt.Errorf("sylius_resource.yaml: could not find path sylius_resource.resources")
	}

	if yamlutil.FindInMapping(resources, meta.ResourceAlias) != nil {
		return false, nil
	}

	classesNode := yamlutil.MappingNode(
		yamlutil.Scalar("model"), yamlutil.Scalar(meta.EntityFQCN),
		yamlutil.Scalar("interface"), yamlutil.Scalar(`Sylius\Component\Resource\Model\ResourceInterface`),
		yamlutil.Scalar("repository"), yamlutil.Scalar(meta.RepoFQCN),
		yamlutil.Scalar("factory"), yamlutil.Scalar(`Sylius\Component\Resource\Factory\Factory`),
	)
	valueNode := yamlutil.MappingNode(
		yamlutil.Scalar("classes"), classesNode,
	)

	resources.Content = append(resources.Content,
		yamlutil.Scalar(meta.ResourceAlias),
		valueNode,
	)

	return true, writeYAMLFile(path, &root)
}

func writeYAMLFile(path string, root *yaml.Node) error {
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return fmt.Errorf("writeYAMLFile: expected DocumentNode with content")
	}
	out, err := yaml.Marshal(root.Content[0])
	if err != nil {
		return err
	}
	out = bytes.TrimPrefix(out, []byte("---\n"))
	return os.WriteFile(path, out, 0644)
}
