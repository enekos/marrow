package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSiteByHost(t *testing.T) {
	cfg := &Config{
		Sites: []SiteConfig{
			{Name: "blog", Hosts: []string{"search.blog.com", "blog.local"}},
			{Name: "docs", Hosts: []string{"search.docs.com"}},
		},
	}

	tests := []struct {
		host string
		want string
	}{
		{"search.blog.com", "blog"},
		{"blog.local", "blog"},
		{"search.docs.com", "docs"},
		{"unknown.com", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			site := cfg.SiteByHost(tt.host)
			var got string
			if site != nil {
				got = site.Name
			}
			if got != tt.want {
				t.Errorf("SiteByHost(%q) = %q, want %q", tt.host, got, tt.want)
			}
		})
	}
}

func TestSiteByName(t *testing.T) {
	cfg := &Config{
		Sites: []SiteConfig{
			{Name: "blog"},
			{Name: "docs"},
		},
	}

	tests := []struct {
		name string
		want string
	}{
		{"blog", "blog"},
		{"docs", "docs"},
		{"missing", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			site := cfg.SiteByName(tt.name)
			var got string
			if site != nil {
				got = site.Name
			}
			if got != tt.want {
				t.Errorf("SiteByName(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestSourcesForSite(t *testing.T) {
	cfg := &Config{
		Sources: []SourceConfig{
			{Name: "blog-md", Type: "local"},
			{Name: "docs-md", Type: "git"},
			{Name: "other", Type: "local"},
		},
		Sites: []SiteConfig{
			{Name: "blog", Sources: []string{"blog-md"}},
			{Name: "docs", Sources: []string{"docs-md", "other"}},
		},
	}

	tests := []struct {
		siteName string
		wantLen  int
		wantNames []string
	}{
		{"blog", 1, []string{"blog-md"}},
		{"docs", 2, []string{"docs-md", "other"}},
		{"empty", 0, nil},
	}

	for _, tt := range tests {
		t.Run(tt.siteName, func(t *testing.T) {
			site := cfg.SiteByName(tt.siteName)
			if site == nil {
				if tt.wantLen > 0 {
					t.Fatalf("site %q not found", tt.siteName)
				}
				return
			}
			got := cfg.SourcesForSite(site)
			if len(got) != tt.wantLen {
				t.Errorf("SourcesForSite(%q) len = %d, want %d", tt.siteName, len(got), tt.wantLen)
			}
			for i, want := range tt.wantNames {
				if i >= len(got) || got[i].Name != want {
					t.Errorf("SourcesForSite(%q)[%d] = %q, want %q", tt.siteName, i, got[i].Name, want)
				}
			}
		})
	}
}

func TestHasSites(t *testing.T) {
	if (&Config{}).HasSites() {
		t.Error("empty config should not have sites")
	}
	if !(&Config{Sites: []SiteConfig{{Name: "x"}}}).HasSites() {
		t.Error("config with sites should return true")
	}
}

func TestLoad_SitesFromConfig(t *testing.T) {
	base := t.TempDir()
	content := `
[[sources]]
name = "blog-md"
type = "local"
dir = "/data/blog"

[[sites]]
name = "blog"
hosts = ["search.blog.com"]
cors_origins = ["https://blog.com"]
api_key = "secret"
sources = ["blog-md"]
rate_limit_rps = 20
`
	_ = os.WriteFile(filepath.Join(base, ".marrow.toml"), []byte(content), 0644)

	cfg, err := Load(base)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(cfg.Sites) != 1 {
		t.Fatalf("expected 1 site, got %d", len(cfg.Sites))
	}

	site := cfg.Sites[0]
	if site.Name != "blog" {
		t.Errorf("site.Name = %q, want %q", site.Name, "blog")
	}
	if len(site.Hosts) != 1 || site.Hosts[0] != "search.blog.com" {
		t.Errorf("site.Hosts = %v, want [search.blog.com]", site.Hosts)
	}
	if len(site.CORSOrigins) != 1 || site.CORSOrigins[0] != "https://blog.com" {
		t.Errorf("site.CORSOrigins = %v, want [https://blog.com]", site.CORSOrigins)
	}
	if site.APIKey != "secret" {
		t.Errorf("site.APIKey = %q, want %q", site.APIKey, "secret")
	}
	if site.RateLimitRPS != 20 {
		t.Errorf("site.RateLimitRPS = %v, want 20", site.RateLimitRPS)
	}
	if len(site.Sources) != 1 || site.Sources[0] != "blog-md" {
		t.Errorf("site.Sources = %v, want [blog-md]", site.Sources)
	}
}
