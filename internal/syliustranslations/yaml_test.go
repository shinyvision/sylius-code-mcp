package syliustranslations

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestReadFlatMap_nested(t *testing.T) {
	src := `
app:
    ui:
        chat_agents: Agents
        name: Name
`
	flat, err := readFlatMap([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"app.ui.chat_agents": "Agents",
		"app.ui.name":        "Name",
	}
	assertFlatMap(t, flat, want)
}

func TestReadFlatMap_flatKeys(t *testing.T) {
	src := `
app:
    ui:
        chat_agents.gemini: Gemini Pro
        chat_agents.gemini.help: Help text
`
	flat, err := readFlatMap([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"app.ui.chat_agents.gemini":      "Gemini Pro",
		"app.ui.chat_agents.gemini.help": "Help text",
	}
	assertFlatMap(t, flat, want)
}

func TestReadFlatMap_empty(t *testing.T) {
	flat, err := readFlatMap(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(flat) != 0 {
		t.Errorf("expected empty map, got %v", flat)
	}
}

func TestInsertInDoc_simple(t *testing.T) {
	doc := emptyDoc()
	insertInDoc(doc, "app.ui.name", "Name", false)
	insertInDoc(doc, "app.ui.email", "Email", false)

	out := marshalDoc(t, doc)

	// Must produce nested YAML.
	if !strings.Contains(out, "ui:") {
		t.Errorf("expected nested 'ui:' mapping, got:\n%s", out)
	}

	// Roundtrip.
	flat, err := readFlatMap([]byte(out))
	if err != nil {
		t.Fatal(err)
	}
	assertFlatMap(t, flat, map[string]string{
		"app.ui.name":  "Name",
		"app.ui.email": "Email",
	})
}

// verifies that a second insert with overwrite=false
// leaves the original value intact and returns false.
func TestInsertInDoc_noOverwrite(t *testing.T) {
	doc := emptyDoc()
	insertInDoc(doc, "app.ui.name", "Original", false)
	changed := insertInDoc(doc, "app.ui.name", "New", false)
	if changed {
		t.Error("expected insertInDoc to return false when key already exists and overwrite=false")
	}
	out := marshalDoc(t, doc)
	flat, _ := readFlatMap([]byte(out))
	if flat["app.ui.name"] != "Original" {
		t.Errorf("expected value 'Original', got %q", flat["app.ui.name"])
	}
}

// verifies that overwrite=true updates an existing value.
func TestInsertInDoc_overwrite(t *testing.T) {
	doc := emptyDoc()
	insertInDoc(doc, "app.ui.name", "Original", false)
	changed := insertInDoc(doc, "app.ui.name", "Updated", true)
	if !changed {
		t.Error("expected insertInDoc to return true when overwriting")
	}
	out := marshalDoc(t, doc)
	flat, _ := readFlatMap([]byte(out))
	if flat["app.ui.name"] != "Updated" {
		t.Errorf("expected value 'Updated', got %q", flat["app.ui.name"])
	}
}

func TestInsertInDoc_conflictDissolve(t *testing.T) {
	doc := emptyDoc()
	insertInDoc(doc, "app.ui.chat_agents.gemini", "Gemini Pro 3.1", false)
	insertInDoc(doc, "app.ui.chat_agents.gemini.help", "Help text", false)

	out := marshalDoc(t, doc)

	// app, ui must still be nested mappings.
	if !strings.Contains(out, "app:") {
		t.Errorf("expected 'app:' key:\n%s", out)
	}
	if !strings.Contains(out, "ui:") {
		t.Errorf("expected 'ui:' key:\n%s", out)
	}

	// The conflict should be resolved as flat keys inside chat_agents.
	if !strings.Contains(out, "gemini:") {
		t.Errorf("expected flat 'gemini:' key in chat_agents:\n%s", out)
	}
	if !strings.Contains(out, "gemini.help:") {
		t.Errorf("expected flat 'gemini.help:' key in chat_agents:\n%s", out)
	}

	// Roundtrip must preserve the values.
	flat, err := readFlatMap([]byte(out))
	if err != nil {
		t.Fatal(err)
	}
	assertFlatMap(t, flat, map[string]string{
		"app.ui.chat_agents.gemini":      "Gemini Pro 3.1",
		"app.ui.chat_agents.gemini.help": "Help text",
	})
}

func TestDeleteFromDoc_basic(t *testing.T) {
	doc := emptyDoc()
	insertInDoc(doc, "app.ui.name", "Name", false)
	insertInDoc(doc, "app.ui.email", "Email", false)

	ok := deleteFromDoc(doc, "app.ui.name")
	if !ok {
		t.Error("expected deleteFromDoc to return true")
	}
	out := marshalDoc(t, doc)
	flat, _ := readFlatMap([]byte(out))
	if _, exists := flat["app.ui.name"]; exists {
		t.Error("expected 'app.ui.name' to be deleted")
	}
	if flat["app.ui.email"] != "Email" {
		t.Errorf("expected 'app.ui.email' = 'Email', got %q", flat["app.ui.email"])
	}
}

func TestDeleteFromDoc_missingKey(t *testing.T) {
	doc := emptyDoc()
	insertInDoc(doc, "app.ui.name", "Name", false)
	ok := deleteFromDoc(doc, "app.ui.nonexistent")
	if ok {
		t.Error("expected deleteFromDoc to return false for non-existent key")
	}
}

func TestInsertDeletePreservesStructure(t *testing.T) {
	doc := emptyDoc()
	insertInDoc(doc, "app.ui.chat_agents.gemini", "Gemini Pro 3.1", false)
	insertInDoc(doc, "app.ui.chat_agents.gemini.help", "Help text", false)
	deleteFromDoc(doc, "app.ui.chat_agents.gemini.help")

	out := marshalDoc(t, doc)

	flat, err := readFlatMap([]byte(out))
	if err != nil {
		t.Fatal(err)
	}
	assertFlatMap(t, flat, map[string]string{
		"app.ui.chat_agents.gemini": "Gemini Pro 3.1",
	})
	if _, exists := flat["app.ui.chat_agents.gemini.help"]; exists {
		t.Error("deleted key 'app.ui.chat_agents.gemini.help' still present")
	}
}

func TestInsertInDoc_preservesExistingContent(t *testing.T) {
	src := `app:
    ui:
        name: Name
        existing: Value
`
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(src), &doc); err != nil {
		t.Fatal(err)
	}

	insertInDoc(&doc, "app.ui.new_key", "New", false)

	out := marshalDoc(t, &doc)
	flat, err := readFlatMap([]byte(out))
	if err != nil {
		t.Fatal(err)
	}
	assertFlatMap(t, flat, map[string]string{
		"app.ui.name":     "Name",
		"app.ui.existing": "Value",
		"app.ui.new_key":  "New",
	})
}

func emptyDoc() *yaml.Node {
	return &yaml.Node{
		Kind:    yaml.DocumentNode,
		Content: []*yaml.Node{{Kind: yaml.MappingNode}},
	}
}

func marshalDoc(t *testing.T, doc *yaml.Node) string {
	t.Helper()
	data, err := yaml.Marshal(doc)
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}
	return string(data)
}

func assertFlatMap(t *testing.T, got, want map[string]string) {
	t.Helper()
	for k, wv := range want {
		if gv, ok := got[k]; !ok {
			t.Errorf("missing key %q", k)
		} else if gv != wv {
			t.Errorf("key %q: got %q, want %q", k, gv, wv)
		}
	}
	for k := range got {
		if _, ok := want[k]; !ok {
			t.Errorf("unexpected key %q = %q", k, got[k])
		}
	}
}
