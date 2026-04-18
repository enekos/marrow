package service

import (
	"context"

	"marrow/internal/search"
)

// Searcher wraps the search engine with a service-level API.
type Searcher struct {
	Engine *search.Engine
}

// Search runs a query and returns ranked results.
func (s *Searcher) Search(ctx context.Context, query string, limit int, source, docType, lang string) ([]search.Result, error) {
	filter := search.Filter{
		Source:  source,
		DocType: docType,
		Lang:    lang,
	}
	return s.Engine.Search(ctx, query, lang, limit, filter)
}
