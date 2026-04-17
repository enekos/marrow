package markdown

import (
	"strings"
	"testing"
)

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

func TestParseWithoutFrontmatter(t *testing.T) {
	source := []byte("## Subheading\n\nSome content here.\n\n> A quote\n\n- item one\n- item two")
	doc, err := Parse(source)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc.Title != "" {
		t.Errorf("expected empty title, got %q", doc.Title)
	}
	want := "Subheading Some content here. A quote item one item two"
	if doc.Text != want {
		t.Errorf("expected text %q, got %q", want, doc.Text)
	}
}

func TestParseFrontmatterLangOnly(t *testing.T) {
	source := []byte("---\nlang: fr\n---\n# Bonjour\nMonde")
	doc, err := ParseWithDefault(source, "en")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc.Lang != "fr" {
		t.Errorf("expected lang fr, got %s", doc.Lang)
	}
	if doc.Title != "Bonjour" {
		t.Errorf("expected title Bonjour, got %s", doc.Title)
	}
}

func TestParseFrontmatterTitleOnly(t *testing.T) {
	source := []byte("---\ntitle: My Title\n---\n# Hello\nWorld")
	doc, err := Parse(source)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc.Title != "My Title" {
		t.Errorf("expected title My Title, got %s", doc.Title)
	}
	if doc.Lang != "en" {
		t.Errorf("expected lang en, got %s", doc.Lang)
	}
}

func TestParseInvalidFrontmatter(t *testing.T) {
	source := []byte("---\nlang: [unclosed\n---\n# Hello\nWorld")
	_, err := Parse(source)
	if err == nil {
		t.Fatal("expected error for invalid frontmatter")
	}
}

func TestParseEmpty(t *testing.T) {
	doc, err := Parse([]byte(""))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc.Title != "" {
		t.Errorf("expected empty title, got %q", doc.Title)
	}
	if doc.Text != "" {
		t.Errorf("expected empty text, got %q", doc.Text)
	}
}

func TestParseWithHTMLBlock(t *testing.T) {
	source := []byte("<div>\n\nSome text inside HTML\n\n</div>\n")
	doc, err := Parse(source)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !strings.Contains(doc.Text, "Some text inside HTML") {
		t.Errorf("expected text to contain 'Some text inside HTML', got %q", doc.Text)
	}
}

func TestParseFirstH1AfterOtherHeadings(t *testing.T) {
	source := []byte("## H2\n\n### H3\n\n# H1\n\nMore text\n")
	doc, err := Parse(source)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc.Title != "H1" {
		t.Errorf("expected title H1, got %q", doc.Title)
	}
	want := "H2 H3 H1 More text"
	if doc.Text != want {
		t.Errorf("expected text %q, got %q", want, doc.Text)
	}
}

func TestParseExtractTextCodeLinksImages(t *testing.T) {
	source := []byte("# Title\n\nSome `inline code` here.\n\n```go\nfmt.Println(\"hello\")\n```\n\nVisit [link text](https://example.com) and see ![alt text](img.png).\n\nContact <https://example.com>.\n")
	doc, err := Parse(source)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc.Title != "Title" {
		t.Errorf("expected title Title, got %q", doc.Title)
	}
	want := "Title Some here. Visit link text and see alt text. Contact https://example.com."
	if doc.Text != want {
		t.Errorf("expected text %q, got %q", want, doc.Text)
	}
}

func TestParseExtractTextIndentedCodeBlock(t *testing.T) {
	source := []byte("Some text.\n\n    code line\n\nMore text.\n")
	doc, err := Parse(source)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := "Some text. More text."
	if doc.Text != want {
		t.Errorf("expected text %q, got %q", want, doc.Text)
	}
}

func TestExtractFrontmatterNoDelimiter(t *testing.T) {
	source := []byte("# Hello\nWorld")
	var doc Document
	body, err := extractFrontmatter(source, &doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body) != string(source) {
		t.Errorf("expected body to equal source, got %q", string(body))
	}
}

func TestExtractFrontmatterUnclosedDelimiter(t *testing.T) {
	source := []byte("---\nlang: es\n")
	var doc Document
	body, err := extractFrontmatter(source, &doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body) != string(source) {
		t.Errorf("expected body to equal source, got %q", string(body))
	}
}
