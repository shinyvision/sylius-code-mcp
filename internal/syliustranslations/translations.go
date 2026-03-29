package syliustranslations

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type missingKey struct {
	locale string
	domain string
	key    string
}

func translationFilePath(projectRoot, domain, locale string) string {
	base := filepath.Join(projectRoot, "translations")
	yaml := filepath.Join(base, fmt.Sprintf("%s.%s.yaml", domain, locale))
	yml := filepath.Join(base, fmt.Sprintf("%s.%s.yml", domain, locale))
	if _, err := os.Stat(yaml); err == nil {
		return yaml
	}
	if _, err := os.Stat(yml); err == nil {
		return yml
	}
	return yaml
}

func relTranslPath(projectRoot, abs string) string {
	rel, err := filepath.Rel(projectRoot, abs)
	if err != nil {
		return abs
	}
	return rel
}

func flatMapHasKey(flat map[string]string, key string) bool {
	_, ok := flat[key]
	return ok
}

type CheckResult struct {
	Missing []missingKey
}

func CheckKeys(projectRoot, domain, locale string, keys []string) (CheckResult, error) {
	if len(keys) == 0 {
		scanned := ScanProjectKeys(projectRoot)
		domainKeys := scanned.Keys[domain]
		for k := range domainKeys {
			keys = append(keys, k)
		}
		sort.Strings(keys)
	}

	path := translationFilePath(projectRoot, domain, locale)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		data = nil
	} else if err != nil {
		return CheckResult{}, fmt.Errorf("reading %s: %w", relTranslPath(projectRoot, path), err)
	}

	flat, err := readFlatMap(data)
	if err != nil {
		return CheckResult{}, fmt.Errorf("parsing %s: %w", relTranslPath(projectRoot, path), err)
	}

	var result CheckResult
	for _, key := range keys {
		if !strings.HasPrefix(key, "app.") {
			continue
		}
		if !flatMapHasKey(flat, key) {
			result.Missing = append(result.Missing, missingKey{locale: locale, domain: domain, key: key})
		}
	}
	return result, nil
}

type AddResult struct {
	Added map[string][]string
}

func AddKeys(projectRoot, domain, locale string, entries map[string]string) (AddResult, error) {
	path := translationFilePath(projectRoot, domain, locale)
	rel := relTranslPath(projectRoot, path)
	result := AddResult{Added: make(map[string][]string)}

	doc, err := loadYAMLDoc(path)
	if err != nil {
		return AddResult{}, err
	}

	changed := false
	for key, value := range entries {
		if insertInDoc(doc, key, value, false) {
			result.Added[rel] = append(result.Added[rel], key)
			changed = true
		}
	}

	if changed {
		if err := saveYAMLDoc(path, doc); err != nil {
			return AddResult{}, fmt.Errorf("writing %s: %w", rel, err)
		}
		sort.Strings(result.Added[rel])
	}

	return result, nil
}

type EditResult struct {
	Edited   map[string][]string
	NotFound []string
}

func EditKeys(projectRoot, domain, locale string, entries map[string]string) (EditResult, error) {
	path := translationFilePath(projectRoot, domain, locale)
	rel := relTranslPath(projectRoot, path)
	result := EditResult{Edited: make(map[string][]string)}

	doc, err := loadYAMLDoc(path)
	if err != nil {
		return EditResult{}, err
	}

	changed := false
	for key, value := range entries {
		if !keyExistsInDoc(doc, key) {
			result.NotFound = append(result.NotFound, key)
			continue
		}
		insertInDoc(doc, key, value, true)
		result.Edited[rel] = append(result.Edited[rel], key)
		changed = true
	}

	if changed {
		if err := saveYAMLDoc(path, doc); err != nil {
			return EditResult{}, fmt.Errorf("writing %s: %w", rel, err)
		}
		sort.Strings(result.Edited[rel])
	}
	sort.Strings(result.NotFound)

	return result, nil
}

type RemoveResult struct {
	Removed  map[string][]string
	NotFound []string
}

func RemoveKeys(projectRoot, domain, locale string, keys []string) (RemoveResult, error) {
	path := translationFilePath(projectRoot, domain, locale)
	rel := relTranslPath(projectRoot, path)
	result := RemoveResult{Removed: make(map[string][]string)}

	doc, err := loadYAMLDoc(path)
	if err != nil {
		return RemoveResult{}, err
	}

	changed := false
	for _, key := range keys {
		if deleteFromDoc(doc, key) {
			result.Removed[rel] = append(result.Removed[rel], key)
			changed = true
		} else {
			result.NotFound = append(result.NotFound, key)
		}
	}

	if changed {
		if err := saveYAMLDoc(path, doc); err != nil {
			return RemoveResult{}, fmt.Errorf("writing %s: %w", rel, err)
		}
		sort.Strings(result.Removed[rel])
	}
	sort.Strings(result.NotFound)

	return result, nil
}
