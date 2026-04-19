# VPS Deployment Guide

This guide covers deploying Marrow as a persistent search service on a VPS using Docker Compose + Caddy.

## Prerequisites

- A VPS with Docker and Docker Compose installed
- One or more domains / subdomains pointing to your VPS
- An embedding provider (Ollama recommended for self-hosted; OpenAI works too)

## Quick Start

### 1. Clone and build

```bash
git clone https://github.com/enekos/marrow.git
cd marrow
```

### 2. Create `config.toml`

```toml
[server]
addr = ":8080"
db = "/data/marrow.db"
log_format = "json"
sync_interval = "15m"

[embedding]
provider = "ollama"
model = "nomic-embed-text"
base_url = "http://host.docker.internal:11434"

[[sources]]
name = "blog-docs"
type = "local"
dir = "/data/sites/blog"
default_lang = "en"

[[sources]]
name = "docs-site"
type = "git"
repo_url = "https://github.com/user/docs"
local_path = "/data/repos/docs"
token = "${GITHUB_TOKEN}"

[[sites]]
name = "blog"
hosts = ["search.blog.example.com"]
cors_origins = ["https://blog.example.com"]
api_key = "${BLOG_API_KEY}"
sources = ["blog-docs"]
rate_limit_rps = 20

[[sites]]
name = "docs"
hosts = ["search.docs.example.com"]
cors_origins = ["https://docs.example.com"]
api_key = "${DOCS_API_KEY}"
sources = ["docs-site"]
rate_limit_rps = 10
```

### 3. Update `Caddyfile`

Replace the example hostnames with your actual domains.

### 4. Start services

```bash
docker compose up -d
```

Caddy will automatically provision TLS certificates via Let's Encrypt.

### 5. Verify

```bash
curl https://search.blog.example.com/health
curl -X POST https://search.blog.example.com/search \
  -H "Authorization: Bearer ${BLOG_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"q": "getting started", "limit": 5}'
```

## Updating

```bash
git pull
docker compose up -d --build
```

## Logs

```bash
docker compose logs -f marrow
```

## Backup

```bash
./scripts/backup.sh data/marrow.db
```

Set `S3_BUCKET` and configure the AWS CLI to upload backups automatically.
