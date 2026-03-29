package container

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type PSR4Map map[string][]string

func LoadPSR4(projectRoot string) (PSR4Map, error) {
	result := make(PSR4Map)

	if err := loadComposerJSON(projectRoot, result); err != nil {
		return nil, err
	}

	_ = loadVendorPSR4(projectRoot, result)

	return result, nil
}

func loadComposerJSON(projectRoot string, result PSR4Map) error {
	data, err := os.ReadFile(filepath.Join(projectRoot, "composer.json"))
	if err != nil {
		return fmt.Errorf("reading composer.json: %w", err)
	}

	var cfg struct {
		Autoload struct {
			PSR4 map[string]json.RawMessage `json:"psr-4"`
		} `json:"autoload"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parsing composer.json: %w", err)
	}

	for ns, raw := range cfg.Autoload.PSR4 {
		var single string
		if err := json.Unmarshal(raw, &single); err == nil {
			result[ns] = appendUniquePSR4(result[ns], single)
			continue
		}
		var multi []string
		if err := json.Unmarshal(raw, &multi); err == nil {
			for _, p := range multi {
				result[ns] = appendUniquePSR4(result[ns], p)
			}
		}
	}
	return nil
}

func loadVendorPSR4(projectRoot string, result PSR4Map) error {
	path := filepath.Join(projectRoot, "vendor", "composer", "autoload_psr4.php")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	content := string(data)

	entryRe := regexp.MustCompile(`'([^']+)'\s*=>\s*array\(([^)]+)\)`)
	pathRe := regexp.MustCompile(`\$(vendorDir|baseDir)\s*\.\s*'([^']*)'`)

	for _, m := range entryRe.FindAllStringSubmatch(content, -1) {
		ns := strings.ReplaceAll(m[1], `\\`, `\`)
		for _, pm := range pathRe.FindAllStringSubmatch(m[2], -1) {
			var base string
			if pm[1] == "vendorDir" {
				base = "vendor"
			} else {
				base = "."
			}
			rel := filepath.Join(base, filepath.FromSlash(pm[2]))
			result[ns] = appendUniquePSR4(result[ns], rel)
		}
	}
	return nil
}

func appendUniquePSR4(slice []string, s string) []string {
	for _, v := range slice {
		if v == s {
			return slice
		}
	}
	return append(slice, s)
}

func ResolveClass(className string, psr4 PSR4Map, projectRoot string) (string, bool) {
	className = strings.TrimLeft(className, "\\")

	for namespace, dirs := range psr4 {
		ns := strings.TrimLeft(namespace, "\\")
		if !strings.HasPrefix(className, ns) {
			continue
		}
		relClass := strings.TrimPrefix(className, ns)
		relFile := strings.ReplaceAll(relClass, "\\", string(filepath.Separator)) + ".php"

		for _, dir := range dirs {
			absDir := dir
			if !filepath.IsAbs(absDir) {
				absDir = filepath.Join(projectRoot, dir)
			}
			candidate := filepath.Join(absDir, relFile)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate, true
			}
		}
	}
	return "", false
}
