package markdown

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"gopkg.in/yaml.v3"
)

// Document holds extracted metadata and plain text.
type Document struct {
	Title string
	Lang  string
	Text  string
}

// Parse extracts frontmatter and plain text from Markdown content.
func Parse(source []byte) (Document, error) {
	var doc Document
	doc.Lang = "en" // default

	body, err := extractFrontmatter(source, &doc)
	if err != nil {
		return doc, err
	}

	parser := goldmark.DefaultParser()
	reader := text.NewReader(body)
	root := parser.Parse(reader)

	var sb strings.Builder
	extractText(root, body, &sb)
	doc.Text = cleanWhitespace(sb.String())

	if doc.Title == "" {
		doc.Title = extractFirstH1(root, body)
	}

	return doc, nil
}

func extractFrontmatter(source []byte, doc *Document) ([]byte, error) {
	trimmed := bytes.TrimSpace(source)
	if !bytes.HasPrefix(trimmed, []byte("---")) {
		return source, nil
	}
	rest := trimmed[3:]
	idx := bytes.Index(rest, []byte("---"))
	if idx == -1 {
		return source, nil
	}
	fm := rest[:idx]
	body := rest[idx+3:]

	var meta map[string]interface{}
	if err := yaml.Unmarshal(fm, &meta); err != nil {
		return source, fmt.Errorf("invalid frontmatter: %w", err)
	}
	if v, ok := meta["title"]; ok {
		doc.Title = fmt.Sprintf("%v", v)
	}
	if v, ok := meta["lang"]; ok {
		doc.Lang = fmt.Sprintf("%v", v)
	}
	return body, nil
}

func extractText(n ast.Node, source []byte, sb *strings.Builder) {
	ast.Walk(n, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch node := n.(type) {
		case *ast.Text:
			v := node.Value(source)
			if len(v) > 0 {
				sb.Write(v)
			}
		case *ast.AutoLink:
			sb.Write(node.Label(source))
		case *ast.CodeSpan:
			// skip inline code values to reduce noise in embeddings/fts
			return ast.WalkSkipChildren, nil
		case *ast.CodeBlock, *ast.FencedCodeBlock:
			return ast.WalkSkipChildren, nil
		case *ast.Heading, *ast.Paragraph, *ast.ListItem, *ast.Blockquote:
			ensureSpace(sb)
		}
		return ast.WalkContinue, nil
	})
}

func extractFirstH1(n ast.Node, source []byte) string {
	var title string
	ast.Walk(n, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering || title != "" {
			return ast.WalkContinue, nil
		}
		if h, ok := n.(*ast.Heading); ok && h.Level == 1 {
			var sb strings.Builder
			extractText(h, source, &sb)
			title = cleanWhitespace(sb.String())
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})
	return title
}

func ensureSpace(sb *strings.Builder) {
	if sb.Len() == 0 {
		return
	}
	b := sb.String()
	if b[len(b)-1] != ' ' {
		sb.WriteByte(' ')
	}
}

func cleanWhitespace(s string) string {
	return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
}
