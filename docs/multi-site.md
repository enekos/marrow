# Multi-Site Configuration

Marrow can serve search for multiple static sites from a single process. Each site is isolated by domain / subdomain and can have its own CORS rules, API key, rate limits, and source mapping.

## How It Works

When the `vps` build tag is enabled, Marrow reads the `[[sites]]` array from your config. On every request it resolves the site using:

1. The `Host` header (matched against `hosts`)
2. The `X-Site` header (matched against `site.name`)

If a site is resolved, the `/search` endpoint automatically filters results to only documents from that site's configured `sources`. Clients do not need to (and cannot) escape their site's boundary.

## Example: Two Sites

```toml
[[sources]]
name = "blog-md"
type = "local"
dir = "/data/blog"

[[sources]]
name = "docs-md"
type = "git"
repo_url = "https://github.com/acme/docs"
local_path = "/data/repos/docs"

[[sites]]
name = "blog"
hosts = ["search.blog.com"]
cors_origins = ["https://blog.com", "https://www.blog.com"]
api_key = "sk_blog_xxx"
sources = ["blog-md"]
rate_limit_rps = 20

[[sites]]
name = "docs"
hosts = ["search.docs.com"]
cors_origins = ["https://docs.com"]
api_key = "sk_docs_xxx"
sources = ["docs-md"]
rate_limit_rps = 10
```

## Client Usage

No `source` parameter is required in the search request:

```bash
curl -X POST https://search.blog.com/search \
  -H "Authorization: Bearer sk_blog_xxx" \
  -H "Content-Type: application/json" \
  -d '{"q": "hello world", "limit": 10}'
```

## Without Sites (Backward Compatible)

If `[[sites]]` is empty or omitted, Marrow behaves exactly as before:
- `/search` accepts an explicit `source` filter
- No CORS enforcement
- No API key requirement
- No rate limiting

This lets the same binary serve local development and production VPS deployments.
