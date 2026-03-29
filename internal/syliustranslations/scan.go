package syliustranslations

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/sylius-code-mcp/internal/phpparser"
)

type ScanResult struct {
	Keys map[string]map[string]struct{}
}

func domainForKey(key string) string {
	switch {
	case strings.HasPrefix(key, "app.ui."):
		return "messages"
	case strings.HasPrefix(key, "app.validator."):
		return "validators"
	case strings.HasPrefix(key, "app.flash."):
		return "flashes"
	}
	return ""
}

func ScanProjectKeys(projectRoot string) ScanResult {
	result := ScanResult{Keys: map[string]map[string]struct{}{
		"messages":   {},
		"validators": {},
		"flashes":    {},
	}}

	twigDirs := discoverTwigDirs(projectRoot)
	for _, dir := range twigDirs {
		scanTwigDir(dir, projectRoot, result)
	}

	srcDir := filepath.Join(projectRoot, "src")
	scanPHPDir(srcDir, projectRoot, result)

	return result
}

func discoverTwigDirs(projectRoot string) []string {
	var dirs []string
	tmplDir := filepath.Join(projectRoot, "templates")
	if info, err := os.Stat(tmplDir); err == nil && info.IsDir() {
		dirs = append(dirs, tmplDir)
	}
	return dirs
}

func scanTwigDir(dir, projectRoot string, result ScanResult) {
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".twig") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(projectRoot, path)
		for _, tk := range phpparser.ExtractTwigTransKeys(string(data), rel) {
			if domain := domainForKey(tk.Key); domain != "" {
				result.Keys[domain][tk.Key] = struct{}{}
			}
		}
		return nil
	})
}

func scanPHPDir(dir, projectRoot string, result ScanResult) {
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".php") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := string(data)
		rel, _ := filepath.Rel(projectRoot, path)

		for _, tk := range phpparser.ExtractMenuLabelKeys(content, rel) {
			if domain := domainForKey(tk.Key); domain != "" {
				result.Keys[domain][tk.Key] = struct{}{}
			}
		}

		if strings.Contains(content, "->trans(") {
			for _, tk := range phpparser.ExtractPHPTransKeys(content, rel) {
				if domain := domainForKey(tk.Key); domain != "" {
					result.Keys[domain][tk.Key] = struct{}{}
				}
			}
		}

		return nil
	})
}
