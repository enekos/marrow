package service

import (
	"context"

	"github.com/enekos/marrow/internal/config"
	"github.com/enekos/marrow/internal/search"
)

// Searcher wraps the search engine with a service-level API.
type Searcher struct {
	Engine *search.Engine
}

// Search runs a query and returns ranked results.
// If site is provided and no explicit source filter is given, the search is
// constrained to the sources belonging to that site.
func (s *Searcher) Search(ctx context.Context, query string, limit int, source, docType, lang string, site *config.SiteConfig) ([]search.Result, error) {
	filter := search.Filter{
		DocType: docType,
		Lang:    lang,
	}
	// When a site is resolved and the caller didn't specify a source,
	// automatically restrict to the site's configured sources.
	if source != "" {
		filter.Sources = []string{source}
	} else if site != nil {
		filter.Sources = site.Sources
		// If site has no sources, return empty results rather than leaking all docs.
		if len(site.Sources) == 0 {
			return []search.Result{}, nil
		}
	}
	return s.Engine.Search(ctx, query, lang, limit, filter)
}
