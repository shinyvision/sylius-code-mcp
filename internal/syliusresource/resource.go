package syliusresource

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/sylius-code-mcp/internal/syliusroute"
)

type Params struct {
	ResourceAlias string
	Namespace     string
}

type Meta struct {
	AliasPrefix     string
	ResourceAlias   string
	ResourceKey     string
	ClassName       string
	TableName       string
	Namespace       string
	EntityNamespace string
	RepoNamespace   string
	EntityClass     string
	RepoClass       string
	EntityFQCN      string
	RepoFQCN        string
	EntityFilePath  string
	RepoFilePath    string
	DoctrineDir     string
	DoctrinePrefix  string
	DoctrineAlias   string
	GridName        string
}

type Result struct {
	EntityCreated  bool
	RepoCreated    bool
	DoctrineAdded  bool
	SyliusResAdded bool
	RouteMessage   string
	Messages       []string
}

func Validate(p Params) error {
	if !strings.Contains(p.ResourceAlias, ".") {
		return errors.New("resourceAlias must contain a dot, e.g. app.my_resource")
	}
	if !strings.HasPrefix(p.Namespace, `App\`) {
		return errors.New(`namespace must start with "App\\"`)
	}
	return nil
}

func ToPascalCase(snake string) string {
	parts := strings.Split(snake, "_")
	var sb strings.Builder
	for _, p := range parts {
		if len(p) == 0 {
			continue
		}
		sb.WriteString(strings.ToUpper(p[:1]) + p[1:])
	}
	return sb.String()
}

func DeriveMeta(p Params) Meta {
	dotIdx := strings.Index(p.ResourceAlias, ".")
	aliasPrefix := p.ResourceAlias[:dotIdx]
	resourceKey := p.ResourceAlias[dotIdx+1:]
	className := ToPascalCase(resourceKey)
	tableName := strings.ReplaceAll(p.ResourceAlias, ".", "_")

	moduleRelPath := strings.TrimPrefix(p.Namespace, `App\`)
	moduleRelPath = strings.ReplaceAll(moduleRelPath, `\`, "/")

	entityFilePath := fmt.Sprintf("src/%s/Entity/%s.php", moduleRelPath, className)
	repoFilePath := fmt.Sprintf("src/%s/Repository/%sRepository.php", moduleRelPath, className)
	doctrineDir := fmt.Sprintf("%%kernel.project_dir%%/src/%s/Entity", moduleRelPath)
	doctrinePrefix := p.Namespace + `\Entity`
	doctrineAlias := p.Namespace
	gridName := "app_" + resourceKey

	entityNamespace := p.Namespace + `\Entity`
	repoNamespace := p.Namespace + `\Repository`
	entityClass := className
	repoClass := className + "Repository"
	entityFQCN := entityNamespace + `\` + entityClass
	repoFQCN := repoNamespace + `\` + repoClass

	return Meta{
		AliasPrefix:     aliasPrefix,
		ResourceAlias:   p.ResourceAlias,
		ResourceKey:     resourceKey,
		ClassName:       className,
		TableName:       tableName,
		Namespace:       p.Namespace,
		EntityNamespace: entityNamespace,
		RepoNamespace:   repoNamespace,
		EntityClass:     entityClass,
		RepoClass:       repoClass,
		EntityFQCN:      entityFQCN,
		RepoFQCN:        repoFQCN,
		EntityFilePath:  entityFilePath,
		RepoFilePath:    repoFilePath,
		DoctrineDir:     doctrineDir,
		DoctrinePrefix:  doctrinePrefix,
		DoctrineAlias:   doctrineAlias,
		GridName:        gridName,
	}
}

var entityTemplate = template.Must(template.New("entity").Parse(`<?php

declare(strict_types=1);

namespace {{ .EntityNamespace }};

use Doctrine\ORM\Mapping as ORM;
use Sylius\Component\Resource\Model\ResourceInterface;
use {{ .RepoFQCN }};

#[ORM\Entity(repositoryClass: {{ .RepoClass }}::class)]
#[ORM\Table(name: '{{ .TableName }}')]
class {{ .EntityClass }} implements ResourceInterface
{
    #[ORM\Id]
    #[ORM\GeneratedValue]
    #[ORM\Column]
    private ?int $id = null;

    public function getId(): ?int
    {
        return $this->id;
    }
}
`))

var repoTemplate = template.Must(template.New("repo").Parse(`<?php

declare(strict_types=1);

namespace {{ .RepoNamespace }};

use {{ .EntityFQCN }};
use Doctrine\ORM\EntityManager;
use Doctrine\ORM\Mapping\ClassMetadata;
use Sylius\Bundle\ResourceBundle\Doctrine\ORM\EntityRepository;

/**
 * @method {{ .EntityClass }}|null find($id, $lockMode = null, $lockVersion = null)
 * @method {{ .EntityClass }}|null findOneBy(array $criteria, ?array $orderBy = null)
 * @method {{ .EntityClass }}[] findAll()
 * @method {{ .EntityClass }}[] findBy(array $criteria, ?array $orderBy = null, $limit = null, $offset = null)
 */
class {{ .RepoClass }} extends EntityRepository
{
    public function __construct(EntityManager $entityManager, ClassMetadata $class)
    {
        parent::__construct($entityManager, $class);
    }
}
`))

func generateEntityFile(meta Meta) (string, error) {
	var buf bytes.Buffer
	if err := entityTemplate.Execute(&buf, meta); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func generateRepoFile(meta Meta) (string, error) {
	var buf bytes.Buffer
	if err := repoTemplate.Execute(&buf, meta); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func ensureEntityFile(projectRoot string, meta Meta) (bool, error) {
	path := filepath.Join(projectRoot, meta.EntityFilePath)
	if _, err := os.Stat(path); err == nil {
		return false, nil
	}
	content, err := generateEntityFile(meta)
	if err != nil {
		return false, fmt.Errorf("generating entity: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return false, fmt.Errorf("creating entity dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return false, fmt.Errorf("writing entity file: %w", err)
	}
	return true, nil
}

func ensureRepositoryFile(projectRoot string, meta Meta) (bool, error) {
	path := filepath.Join(projectRoot, meta.RepoFilePath)
	if _, err := os.Stat(path); err == nil {
		return false, nil
	}
	content, err := generateRepoFile(meta)
	if err != nil {
		return false, fmt.Errorf("generating repository: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return false, fmt.Errorf("creating repository dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return false, fmt.Errorf("writing repository file: %w", err)
	}
	return true, nil
}

func EnsureResource(projectRoot string, p Params) (Result, error) {
	if err := Validate(p); err != nil {
		return Result{}, err
	}

	meta := DeriveMeta(p)

	entityCreated, err := ensureEntityFile(projectRoot, meta)
	if err != nil {
		return Result{}, fmt.Errorf("entity file: %w", err)
	}

	repoCreated, err := ensureRepositoryFile(projectRoot, meta)
	if err != nil {
		return Result{}, fmt.Errorf("repository file: %w", err)
	}

	doctrineAdded, err := ensureDoctrineMapping(projectRoot, meta)
	if err != nil {
		return Result{}, fmt.Errorf("doctrine mapping: %w", err)
	}

	syliusResAdded, err := ensureSyliusResource(projectRoot, meta)
	if err != nil {
		return Result{}, fmt.Errorf("sylius resource: %w", err)
	}

	routeResult, err := syliusroute.EnsureRoute(projectRoot, syliusroute.Params{
		ResourceName: meta.ResourceKey,
		Alias:        meta.ResourceAlias,
		Grid:         meta.GridName,
		Redirect:     "update",
	})
	if err != nil {
		return Result{}, fmt.Errorf("route: %w", err)
	}

	r := Result{
		EntityCreated:  entityCreated,
		RepoCreated:    repoCreated,
		DoctrineAdded:  doctrineAdded,
		SyliusResAdded: syliusResAdded,
		RouteMessage:   routeResult.Message,
	}
	r.Messages = []string{buildResultMessage(projectRoot, meta, r)}
	return r, nil
}

func buildResultMessage(projectRoot string, meta Meta, r Result) string {
	var lines []string

	if r.EntityCreated {
		lines = append(lines, fmt.Sprintf("Created entity at %s.", meta.EntityFilePath))
	}
	if r.RepoCreated {
		lines = append(lines, fmt.Sprintf("Created repository at %s.", meta.RepoFilePath))
	}
	if r.DoctrineAdded {
		lines = append(lines, fmt.Sprintf("Added Doctrine mapping for %q to config/packages/doctrine.yaml.", meta.DoctrineAlias))
	}
	if r.SyliusResAdded {
		lines = append(lines, fmt.Sprintf("Registered resource %q in config/packages/sylius_resource.yaml.", meta.ResourceAlias))
	}
	if r.RouteMessage != "" {
		lines = append(lines, r.RouteMessage)
	}

	lines = append(lines, "")
	lines = append(lines, "Next steps:")
	lines = append(lines, fmt.Sprintf(
		"1. Create a grid at config/packages/sylius_grid/%s.yaml with grid name %q.",
		meta.ResourceKey, meta.GridName,
	))
	lines = append(lines, fmt.Sprintf(
		"2. Complete the entity at %s — add fields, relations, and validation.",
		meta.EntityFilePath,
	))
	lines = append(lines, "3. Run `php bin/console doctrine:migrations:diff` to generate a database migration.")

	_ = projectRoot
	return strings.Join(lines, "\n")
}
