package container

import (
	"encoding/xml"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ServiceTag struct {
	Name  string
	Attrs map[string]string
}

type Service struct {
	ID    string
	Class string
	Tags  []ServiceTag
}

func (s Service) HasTag(name string) bool {
	for _, t := range s.Tags {
		if t.Name == name {
			return true
		}
	}
	return false
}

func (s Service) TagAttr(tagName, attr string) string {
	for _, t := range s.Tags {
		if t.Name == tagName {
			return t.Attrs[attr]
		}
	}
	return ""
}

type Container struct {
	WorkspaceRoot string
	Services      map[string]Service
	Aliases       map[string]string

	TwigRoots   []string
	TwigBundles map[string][]string
}

func FindContainerXML(projectRoot string) (string, error) {
	pattern := filepath.Join(projectRoot, "var", "cache", "dev", "*KernelDevDebugContainer.xml")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("globbing container XML: %w", err)
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no compiled container XML found at %s (run bin/console cache:warmup first)", pattern)
	}
	return matches[0], nil
}

func Load(projectRoot string) (*Container, error) {
	xmlPath, err := FindContainerXML(projectRoot)
	if err != nil {
		return nil, err
	}
	return ParseXML(xmlPath, projectRoot)
}

func ParseXML(absPath, workspaceRoot string) (*Container, error) {
	f, err := os.Open(absPath)
	if err != nil {
		return nil, fmt.Errorf("opening container XML: %w", err)
	}
	defer f.Close()

	c := &Container{
		WorkspaceRoot: workspaceRoot,
		Services:      make(map[string]Service),
		Aliases:       make(map[string]string),
		TwigBundles:   make(map[string][]string),
	}

	dec := xml.NewDecoder(f)
	dec.Strict = false

	type serviceFrame struct {
		id    string
		class string
	}
	var serviceStack []serviceFrame

	var currentTags []ServiceTag

	inTwigService := false
	twigServiceDepth := 0
	inAddPathCall := false
	var pathCallArgs []string
	inPathArg := false
	var argBuf strings.Builder

	twigServiceID := "twig.loader.native_filesystem"

	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}

		switch t := tok.(type) {
		case xml.StartElement:
			local := t.Name.Local

			switch local {
			case "service":
				id, class, alias, isAbstract := "", "", "", false
				for _, a := range t.Attr {
					switch a.Name.Local {
					case "id":
						id = a.Value
					case "class":
						class = a.Value
					case "alias":
						alias = a.Value
					case "abstract":
						isAbstract = a.Value == "true"
					}
				}

				if len(serviceStack) == 0 {
					currentTags = nil
					if !isAbstract && id != "" && !strings.Contains(id, " ") {
						if class != "" {
							if _, exists := c.Services[id]; !exists {
								c.Services[id] = Service{ID: id, Class: class}
							}
						} else if alias != "" {
							c.Aliases[id] = alias
						}
					}
				}
				serviceStack = append(serviceStack, serviceFrame{id: id, class: class})

				if id == twigServiceID {
					inTwigService = true
					twigServiceDepth = len(serviceStack)
				}

			case "tag":
				if len(serviceStack) == 1 {
					tag := ServiceTag{Attrs: make(map[string]string)}
					for _, a := range t.Attr {
						if a.Name.Local == "name" {
							tag.Name = a.Value
						} else {
							tag.Attrs[a.Name.Local] = a.Value
						}
					}
					currentTags = append(currentTags, tag)
				}

			case "call":
				if inTwigService && len(serviceStack) == twigServiceDepth {
					method := ""
					for _, a := range t.Attr {
						if a.Name.Local == "method" {
							method = a.Value
							break
						}
					}
					if method == "addPath" {
						inAddPathCall = true
						pathCallArgs = pathCallArgs[:0]
					}
				}

			case "argument":
				if inTwigService && inAddPathCall {
					inPathArg = true
					argBuf.Reset()
				}
			}

		case xml.CharData:
			if inPathArg {
				argBuf.Write(t)
			}

		case xml.EndElement:
			local := t.Name.Local

			switch local {
			case "service":
				if len(serviceStack) == 0 {
					break
				}
				frame := serviceStack[len(serviceStack)-1]
				serviceStack = serviceStack[:len(serviceStack)-1]

				if len(serviceStack) == 0 && frame.id != "" {
					if svc, exists := c.Services[frame.id]; exists {
						svc.Tags = currentTags
						c.Services[frame.id] = svc
					}
					currentTags = nil
				}

				if inTwigService && len(serviceStack) < twigServiceDepth-1 {
					inTwigService = false
				}

			case "argument":
				if inPathArg {
					val := strings.TrimSpace(argBuf.String())
					pathCallArgs = append(pathCallArgs, val)
					inPathArg = false
				}

			case "call":
				if inAddPathCall {
					inAddPathCall = false
					if len(pathCallArgs) > 0 {
						base := strings.TrimSpace(pathCallArgs[0])
						if base != "" {
							if len(pathCallArgs) >= 2 {
								bundle := strings.TrimSpace(pathCallArgs[1])
								if !strings.HasPrefix(bundle, "!") && bundle != "" {
									c.TwigBundles[bundle] = appendUnique(c.TwigBundles[bundle], base)
								}
							} else {
								c.TwigRoots = appendUnique(c.TwigRoots, base)
							}
						}
					}
				}
			}
		}
	}

	return c, nil
}

func (c *Container) ServicesWithTag(tagName string) []Service {
	var result []Service
	for _, svc := range c.Services {
		if svc.HasTag(tagName) {
			result = append(result, svc)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func (c *Container) ResolveAlias(id string) string {
	for range 10 {
		target, ok := c.Aliases[id]
		if !ok {
			return id
		}
		id = target
	}
	return id
}

func (c *Container) TwigTemplates() []string {
	seen := make(map[string]struct{})

	add := func(value string) {
		value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
		value = strings.TrimPrefix(value, "./")
		if value != "" {
			seen[value] = struct{}{}
		}
	}

	for _, root := range c.TwigRoots {
		base := root
		if !filepath.IsAbs(base) {
			base = filepath.Join(c.WorkspaceRoot, base)
		}
		walkTwig(base, func(path string) {
			if rel, err := filepath.Rel(base, path); err == nil {
				add(filepath.ToSlash(rel))
			}
		})
	}

	for bundle, bases := range c.TwigBundles {
		if bundle == "" {
			continue
		}
		for _, base := range bases {
			abs := base
			if !filepath.IsAbs(abs) {
				abs = filepath.Join(c.WorkspaceRoot, abs)
			}
			walkTwig(abs, func(path string) {
				if rel, err := filepath.Rel(abs, path); err == nil {
					add("@" + bundle + "/" + filepath.ToSlash(rel))
				}
			})
		}
	}

	templates := make([]string, 0, len(seen))
	for v := range seen {
		templates = append(templates, v)
	}
	sort.Strings(templates)
	return templates
}

func walkTwig(base string, fn func(string)) {
	info, err := os.Stat(base)
	if err != nil || !info.IsDir() {
		return
	}
	filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".twig") {
			fn(path)
		}
		return nil
	})
}

func appendUnique(slice []string, s string) []string {
	for _, v := range slice {
		if v == s {
			return slice
		}
	}
	return append(slice, s)
}
