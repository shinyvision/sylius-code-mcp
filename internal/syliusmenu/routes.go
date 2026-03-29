package syliusmenu

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var defaultRouteActions = []string{"index", "create", "update", "show"}

func DiscoverResourceRoutes(projectRoot, routeName string) []string {
	prefix, resourceName := deriveRouteComponents(projectRoot, routeName)
	if prefix == "" {
		return nil
	}

	if routes := routesFromCache(projectRoot, prefix); len(routes) > 0 {
		return routes
	}
	return routesFromYAML(projectRoot, resourceName, prefix)
}

func deriveRouteComponents(projectRoot, routeName string) (prefix, resourceName string) {
	if !strings.HasPrefix(routeName, "app_admin_") {
		return "", ""
	}
	after := strings.TrimPrefix(routeName, "app_admin_")
	parts := strings.Split(after, "_")
	if len(parts) < 2 {
		return "", ""
	}

	syliusDir := filepath.Join(projectRoot, "config", "routes", "_sylius")

	for n := len(parts) - 1; n >= 1; n-- {
		candidate := strings.Join(parts[:n], "_")
		for _, ext := range []string{".yaml", ".yml"} {
			if _, err := os.Stat(filepath.Join(syliusDir, candidate+ext)); err == nil {
				return "app_admin_" + candidate + "_", candidate
			}
		}
	}

	candidate := strings.Join(parts[:len(parts)-1], "_")
	return "app_admin_" + candidate + "_", candidate
}

var cacheRouteKeyRe = regexp.MustCompile(`'(app_admin_[a-z0-9_]+)'\s*=>`)

func routesFromCache(projectRoot, prefix string) []string {
	path := filepath.Join(projectRoot, "var", "cache", "dev", "url_generating_routes.php")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	seen := make(map[string]struct{})
	for _, m := range cacheRouteKeyRe.FindAllSubmatch(data, -1) {
		name := string(m[1])
		if strings.HasPrefix(name, prefix) {
			seen[name] = struct{}{}
		}
	}

	routes := make([]string, 0, len(seen))
	for r := range seen {
		routes = append(routes, r)
	}
	sort.Strings(routes)
	return routes
}

func routesFromYAML(projectRoot, resourceName, prefix string) []string {
	syliusDir := filepath.Join(projectRoot, "config", "routes", "_sylius")

	var data []byte
	for _, ext := range []string{".yaml", ".yml"} {
		d, err := os.ReadFile(filepath.Join(syliusDir, resourceName+ext))
		if err == nil {
			data = d
			break
		}
	}

	primaryKey := strings.TrimSuffix(prefix, "_")
	only, except := parseRouteRestrictions(data, primaryKey)
	standard := buildRouteNames(prefix, only, except)
	custom := parseCustomRoutes(data, prefix, primaryKey)

	all := append(standard, custom...)
	sort.Strings(all)
	return dedup(all)
}

func parseRouteRestrictions(data []byte, primaryKey string) (only, except []string) {
	if len(data) == 0 {
		return nil, nil
	}
	var outer map[string]any
	if err := yaml.Unmarshal(data, &outer); err != nil {
		return nil, nil
	}
	entry, ok := outer[primaryKey].(map[string]any)
	if !ok {
		return nil, nil
	}
	resourceStr, _ := entry["resource"].(string)
	if resourceStr == "" {
		return nil, nil
	}
	var rc map[string]any
	if err := yaml.Unmarshal([]byte(resourceStr), &rc); err != nil {
		return nil, nil
	}
	return toStringSlice(rc["only"]), toStringSlice(rc["except"])
}

func parseCustomRoutes(data []byte, prefix, primaryKey string) []string {
	if len(data) == 0 {
		return nil
	}
	var outer map[string]any
	if err := yaml.Unmarshal(data, &outer); err != nil {
		return nil
	}
	var routes []string
	for key := range outer {
		if key != primaryKey && strings.HasPrefix(key, prefix) {
			routes = append(routes, key)
		}
	}
	return routes
}

func buildRouteNames(prefix string, only, except []string) []string {
	actions := defaultRouteActions
	switch {
	case len(only) > 0:
		actions = only
	case len(except) > 0:
		exceptSet := make(map[string]bool, len(except))
		for _, e := range except {
			exceptSet[e] = true
		}
		var filtered []string
		for _, a := range defaultRouteActions {
			if !exceptSet[a] {
				filtered = append(filtered, a)
			}
		}
		actions = filtered
	}

	routes := make([]string, len(actions))
	for i, a := range actions {
		routes[i] = prefix + a
	}
	return routes
}

func toStringSlice(v any) []string {
	if v == nil {
		return nil
	}
	items, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func dedup(routes []string) []string {
	seen := make(map[string]bool, len(routes))
	out := routes[:0:0]
	for _, r := range routes {
		if !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	return out
}
