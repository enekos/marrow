package markdown

import "testing"

func TestParse_DefaultLang(t *testing.T) {
	source := []byte("# Hello\nWorld")
	doc, err := Parse(source)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc.Lang != "en" {
		t.Errorf("expected default lang en, got %s", doc.Lang)
	}
	if doc.Title != "Hello" {
		t.Errorf("expected title Hello, got %s", doc.Title)
	}
}

func TestParseWithDefault(t *testing.T) {
	source := []byte("# Hello\nWorld")
	doc, err := ParseWithDefault(source, "eu")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc.Lang != "eu" {
		t.Errorf("expected lang eu, got %s", doc.Lang)
	}
}

func TestParseWithDefault_OverriddenByFrontmatter(t *testing.T) {
	source := []byte("---\nlang: es\n---\n# Hola\nMundo")
	doc, err := ParseWithDefault(source, "eu")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc.Lang != "es" {
		t.Errorf("expected lang es from frontmatter, got %s", doc.Lang)
	}
}
