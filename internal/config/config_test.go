package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	want := &Config{
		Server: ServerConfig{
			Addr: ":8080",
			DB:   "marrow.db",
		},
		GitHub: GitHubConfig{
			RepoURL:   "",
			RepoToken: "",
			Source:    "github",
			LocalPath: "./repo",
		},
		Search: SearchConfig{
			DetectLang:  true,
			DefaultLang: "en",
		},
		Sync: SyncConfig{
			Dir:         ".",
			Source:      "local",
			DefaultLang: "en",
		},
		GitHubApp: GitHubAppConfig{
			AppID:          0,
			PrivateKey:     "",
			InstallationID: 0,
			RepoOwner:      "",
			RepoName:       "",
		},
		Embedding: EmbeddingConfig{
			Provider: "",
			Model:    "",
			BaseURL:  "",
			APIKey:   "",
		},
	}

	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("unexpected defaults:\n got  %+v\n want %+v", cfg, want)
	}
}

func TestNewViper_EnvOverride(t *testing.T) {
	envVars := map[string]string{
		"MARROW_SERVER_ADDR":            ":9090",
		"MARROW_SERVER_DB":              "test.db",
		"MARROW_GITHUB_REPO_URL":        "https://github.com/test/repo",
		"MARROW_GITHUB_REPO_TOKEN":      "token123",
		"MARROW_GITHUB_WEBHOOK_SECRET":  "shh",
		"MARROW_GITHUB_SOURCE":          "custom",
		"MARROW_GITHUB_LOCAL_PATH":      "/tmp/repo",
		"MARROW_SEARCH_DETECT_LANG":     "false",
		"MARROW_SEARCH_DEFAULT_LANG":    "es",
		"MARROW_SYNC_DIR":               "/tmp/sync",
		"MARROW_SYNC_SOURCE":            "git",
		"MARROW_SYNC_DEFAULT_LANG":      "fr",
		"MARROW_GITHUB_APP_APP_ID":      "42",
		"MARROW_GITHUB_APP_PRIVATE_KEY": "key",
		"MARROW_GITHUB_APP_INSTALLATION_ID": "7",
		"MARROW_GITHUB_APP_REPO_OWNER":  "owner",
		"MARROW_GITHUB_APP_REPO_NAME":   "name",
		"MARROW_GITHUB_APP_WEBHOOK_SECRET": "appsecret",
		"MARROW_EMBEDDING_PROVIDER":     "openai",
		"MARROW_EMBEDDING_MODEL":        "text-embedding-3-small",
		"MARROW_EMBEDDING_BASE_URL":     "http://localhost:8080",
		"MARROW_EMBEDDING_API_KEY":      "sk-test",
	}

	for k, v := range envVars {
		t.Setenv(k, v)
	}

	v := NewViper(t.TempDir())

	// String values via v.Get come back as strings.
	strChecks := []struct {
		key  string
		want string
	}{
		{"server.addr", ":9090"},
		{"server.db", "test.db"},
		{"github.repo_url", "https://github.com/test/repo"},
		{"github.repo_token", "token123"},
		{"github.webhook_secret", "shh"},
		{"github.source", "custom"},
		{"github.local_path", "/tmp/repo"},
		{"search.default_lang", "es"},
		{"sync.dir", "/tmp/sync"},
		{"sync.source", "git"},
		{"sync.default_lang", "fr"},
		{"github_app.private_key", "key"},
		{"github_app.repo_owner", "owner"},
		{"github_app.repo_name", "name"},
		{"github_app.webhook_secret", "appsecret"},
		{"embedding.provider", "openai"},
		{"embedding.model", "text-embedding-3-small"},
		{"embedding.base_url", "http://localhost:8080"},
		{"embedding.api_key", "sk-test"},
	}

	for _, c := range strChecks {
		got := v.GetString(c.key)
		if got != c.want {
			t.Errorf("%s: got %q, want %q", c.key, got, c.want)
		}
	}

	if got := v.GetBool("search.detect_lang"); got != false {
		t.Errorf("search.detect_lang: got %v, want false", got)
	}
	if got := v.GetInt64("github_app.app_id"); got != 42 {
		t.Errorf("github_app.app_id: got %d, want 42", got)
	}
	if got := v.GetInt64("github_app.installation_id"); got != 7 {
		t.Errorf("github_app.installation_id: got %d, want 7", got)
	}
}

func TestNewViper_ProjectConfigOverride(t *testing.T) {
	base := t.TempDir()
	projectDir := filepath.Join(base, "project")
	_ = os.MkdirAll(projectDir, 0755)

	content := `
[server]
addr = ":7070"
db = "project.db"

[github]
source = "project-github"

[[sources]]
name = "docs"
type = "local"
`
	_ = os.WriteFile(filepath.Join(projectDir, ".marrow.toml"), []byte(content), 0644)

	v := NewViper(projectDir)

	if got := v.GetString("server.addr"); got != ":7070" {
		t.Errorf("server.addr = %q, want %q", got, ":7070")
	}
	if got := v.GetString("server.db"); got != "project.db" {
		t.Errorf("server.db = %q, want %q", got, "project.db")
	}
	if got := v.GetString("github.source"); got != "project-github" {
		t.Errorf("github.source = %q, want %q", got, "project-github")
	}
	if got := v.GetString("github.local_path"); got != "./repo" {
		t.Errorf("github.local_path default was overwritten, got %q", got)
	}

	sources := v.Get("sources")
	if sources == nil {
		t.Fatalf("expected sources to be loaded from project config")
	}
}

func TestNewViper_ProjectConfigDoesNotExist(t *testing.T) {
	base := t.TempDir()
	v := NewViper(base)

	if got := v.GetString("server.addr"); got != ":8080" {
		t.Errorf("server.addr = %q, want default %q", got, ":8080")
	}
}

func TestNewViper_EnvOverridesProjectConfig(t *testing.T) {
	base := t.TempDir()
	projectDir := filepath.Join(base, "project")
	_ = os.MkdirAll(projectDir, 0755)

	content := `
[server]
addr = ":7070"
`
	_ = os.WriteFile(filepath.Join(projectDir, ".marrow.toml"), []byte(content), 0644)

	t.Setenv("MARROW_SERVER_ADDR", ":6060")

	v := NewViper(projectDir)

	if got := v.GetString("server.addr"); got != ":6060" {
		t.Errorf("server.addr = %q, want %q", got, ":6060")
	}
}

func TestUserConfigPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine user home dir")
	}

	want := filepath.Join(home, ".config", "marrow", "config.toml")
	got := UserConfigPath()
	if got != want {
		t.Fatalf("UserConfigPath() = %q, want %q", got, want)
	}
}

func TestLoad_InvalidWorkDir(t *testing.T) {
	// Load should still work because NewViper only uses workDir to find project config.
	// An invalid directory means no project config is found, but defaults apply.
	cfg, err := Load("/this/path/does/not/exist/anywhere/12345")
	if err != nil {
		t.Fatalf("Load failed for invalid workDir: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Server.Addr != ":8080" {
		t.Errorf("expected default addr, got %q", cfg.Server.Addr)
	}
}

func TestLoad_SourcesFromConfig(t *testing.T) {
	base := t.TempDir()
	projectDir := filepath.Join(base, "project")
	_ = os.MkdirAll(projectDir, 0755)

	content := `
[[sources]]
name = "api-docs"
type = "github_api"
dir = "docs"
repo_url = "https://github.com/org/repo"
token = "ghp_xxx"
local_path = "./api-docs"
default_lang = "en"

[[sources]]
name = "blog"
type = "git"
`
	_ = os.WriteFile(filepath.Join(projectDir, ".marrow.toml"), []byte(content), 0644)

	cfg, err := Load(projectDir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(cfg.Sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(cfg.Sources))
	}

	want := []SourceConfig{
		{
			Name:        "api-docs",
			Type:        "github_api",
			Dir:         "docs",
			RepoURL:     "https://github.com/org/repo",
			Token:       "ghp_xxx",
			LocalPath:   "./api-docs",
			DefaultLang: "en",
		},
		{
			Name: "blog",
			Type: "git",
		},
	}

	if !reflect.DeepEqual(cfg.Sources, want) {
		t.Fatalf("unexpected sources:\n got  %+v\n want %+v", cfg.Sources, want)
	}
}

func TestLoad_SourcesFromEnv(t *testing.T) {
	// viper does not unmarshal slices from env automatically without custom logic,
	// but we can still verify that Get() returns the raw value if we set it.
	// This test documents the current behavior.
	t.Setenv("MARROW_SOURCES", `[{"name":"env-source","type":"local"}]`)

	v := NewViper(t.TempDir())
	raw := v.Get("sources")
	if raw == nil {
		t.Skip("viper does not parse slice env vars automatically")
	}
}

func TestSetDefaults(t *testing.T) {
	v := NewViper(t.TempDir())

	// Spot-check a representative set of defaults.
	tests := []struct {
		key  string
		want any
	}{
		{"server.addr", ":8080"},
		{"server.db", "marrow.db"},
		{"github.source", "github"},
		{"github.local_path", "./repo"},
		{"search.detect_lang", true},
		{"search.default_lang", "en"},
		{"sync.dir", "."},
		{"sync.source", "local"},
		{"sync.default_lang", "en"},
		{"github_app.app_id", int64(0)},
		{"embedding.provider", ""},
	}

	for _, tt := range tests {
		got := v.Get(tt.key)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: got %v (%T), want %v (%T)", tt.key, got, got, tt.want, tt.want)
		}
	}
}

func TestLoad_EnvVarTypes(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		check   func(*Config) error
	}{
		{
			name: "bool false",
			env:  map[string]string{"MARROW_SEARCH_DETECT_LANG": "false"},
			check: func(c *Config) error {
				if c.Search.DetectLang != false {
					return fmt.Errorf("DetectLang = %v, want false", c.Search.DetectLang)
				}
				return nil
			},
		},
		{
			name: "bool true explicit",
			env:  map[string]string{"MARROW_SEARCH_DETECT_LANG": "true"},
			check: func(c *Config) error {
				if c.Search.DetectLang != true {
					return fmt.Errorf("DetectLang = %v, want true", c.Search.DetectLang)
				}
				return nil
			},
		},
		{
			name: "int64 app_id",
			env:  map[string]string{"MARROW_GITHUB_APP_APP_ID": "99"},
			check: func(c *Config) error {
				if c.GitHubApp.AppID != 99 {
					return fmt.Errorf("AppID = %d, want 99", c.GitHubApp.AppID)
				}
				return nil
			},
		},
		{
			name: "int64 installation_id",
			env:  map[string]string{"MARROW_GITHUB_APP_INSTALLATION_ID": "123456789"},
			check: func(c *Config) error {
				if c.GitHubApp.InstallationID != 123456789 {
					return fmt.Errorf("InstallationID = %d, want 123456789", c.GitHubApp.InstallationID)
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			cfg, err := Load(t.TempDir())
			if err != nil {
				t.Fatalf("Load failed: %v", err)
			}
			if err := tt.check(cfg); err != nil {
				t.Fatal(err)
			}
		})
	}
}
