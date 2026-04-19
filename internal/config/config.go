package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// DefaultLang is the fallback language code used when no language is detected
// or configured. It is referenced across search, sync, and indexing packages.
const DefaultLang = "en"

// Config is the top-level resolved configuration for Marrow.
type Config struct {
	Server      ServerConfig    `mapstructure:"server"`
	GitHub      GitHubConfig    `mapstructure:"github"`
	Search      SearchConfig    `mapstructure:"search"`
	Sync        SyncConfig      `mapstructure:"sync"`
	GitHubApp   GitHubAppConfig `mapstructure:"github_app"`
	Embedding   EmbeddingConfig `mapstructure:"embedding"`
	Sources     []SourceConfig  `mapstructure:"sources"`
	Sites       []SiteConfig    `mapstructure:"sites"`
}

type EmbeddingConfig struct {
	Provider string `mapstructure:"provider"`
	Model    string `mapstructure:"model"`
	BaseURL  string `mapstructure:"base_url"`
	APIKey   string `mapstructure:"api_key"`
}

type SourceConfig struct {
	Name        string `mapstructure:"name"`
	Type        string `mapstructure:"type"` // local, git, github_api
	Dir         string `mapstructure:"dir"`
	RepoURL     string `mapstructure:"repo_url"`
	Token       string `mapstructure:"token"`
	LocalPath   string `mapstructure:"local_path"`
	DefaultLang string `mapstructure:"default_lang"`
}

type SiteConfig struct {
	Name        string   `mapstructure:"name"`
	Hosts       []string `mapstructure:"hosts"`
	Sources     []string `mapstructure:"sources"`
	CORSOrigins []string `mapstructure:"cors_origins"`
	APIKey      string   `mapstructure:"api_key"`
	RateLimitRPS float64 `mapstructure:"rate_limit_rps"`
}

type ServerConfig struct {
	Addr         string `mapstructure:"addr"`
	DB           string `mapstructure:"db"`
	LogFormat    string `mapstructure:"log_format"`
	SyncInterval string `mapstructure:"sync_interval"`
}

type GitHubConfig struct {
	RepoURL       string `mapstructure:"repo_url"`
	RepoToken     string `mapstructure:"repo_token"`
	WebhookSecret string `mapstructure:"webhook_secret"`
	Source        string `mapstructure:"source"`
	LocalPath     string `mapstructure:"local_path"`
}

type SearchConfig struct {
	DetectLang  bool   `mapstructure:"detect_lang"`
	DefaultLang string `mapstructure:"default_lang"`
}

type SyncConfig struct {
	Dir         string `mapstructure:"dir"`
	Source      string `mapstructure:"source"`
	DefaultLang string `mapstructure:"default_lang"`
}

type GitHubAppConfig struct {
	AppID           int64  `mapstructure:"app_id"`
	PrivateKey      string `mapstructure:"private_key"`
	InstallationID  int64  `mapstructure:"installation_id"`
	RepoOwner       string `mapstructure:"repo_owner"`
	RepoName        string `mapstructure:"repo_name"`
	WebhookSecret   string `mapstructure:"webhook_secret"`
}

// Load reads configuration using a tiered cascade:
//  1. Hardcoded defaults
//  2. User config (~/.config/marrow/config.toml)
//  3. Project config (.marrow.toml, discovered by walking up from workDir)
//  4. Environment variables (MARROW_ prefix)
func Load(workDir string) (*Config, error) {
	v := NewViper(workDir)

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	return &cfg, nil
}

// NewViper creates a configured viper instance for marrow.
func NewViper(workDir string) *viper.Viper {
	v := viper.New()
	setDefaults(v)

	// Layer 2: user config
	home, err := os.UserHomeDir()
	if err == nil {
		v.AddConfigPath(filepath.Join(home, ".config", "marrow"))
	}
	v.SetConfigName("config")
	v.SetConfigType("toml")
	_ = v.MergeInConfig() // no error if file missing

	// Layer 3: project config
	if projectPath := FindProjectConfig(workDir); projectPath != "" {
		pv := viper.New()
		pv.SetConfigFile(projectPath)
		pv.SetConfigType("toml")
		if err := pv.ReadInConfig(); err == nil {
			_ = v.MergeConfigMap(pv.AllSettings())
		}
	}

	// Layer 4: environment variables
	v.SetEnvPrefix("MARROW")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	return v
}

// UserConfigPath returns the path to the user-level config file.
func UserConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "marrow", "config.toml")
}

func setDefaults(v *viper.Viper) {
	// Server
	v.SetDefault("server.addr", ":8080")
	v.SetDefault("server.db", "marrow.db")
	v.SetDefault("server.log_format", "text")
	v.SetDefault("server.sync_interval", "15m")

	// GitHub
	v.SetDefault("github.repo_url", "")
	v.SetDefault("github.repo_token", "")
	v.SetDefault("github.webhook_secret", "")
	v.SetDefault("github.source", "github")
	v.SetDefault("github.local_path", "./repo")

	// Search
	v.SetDefault("search.detect_lang", true)
	v.SetDefault("search.default_lang", DefaultLang)

	// Sync
	v.SetDefault("sync.dir", ".")
	v.SetDefault("sync.source", "local")
	v.SetDefault("sync.default_lang", DefaultLang)

	// GitHub App
	v.SetDefault("github_app.app_id", int64(0))
	v.SetDefault("github_app.private_key", "")
	v.SetDefault("github_app.installation_id", int64(0))
	v.SetDefault("github_app.repo_owner", "")
	v.SetDefault("github_app.repo_name", "")
	v.SetDefault("github_app.webhook_secret", "")

	// Embedding — intentionally unset so users must opt in to mock for tests
	// or configure a real provider (ollama/openai) for production.
	v.SetDefault("embedding.provider", "")
	v.SetDefault("embedding.model", "")
	v.SetDefault("embedding.base_url", "")
	v.SetDefault("embedding.api_key", "")
}

// SiteByHost returns the site configuration matching the given host header.
func (c *Config) SiteByHost(host string) *SiteConfig {
	for i := range c.Sites {
		for _, h := range c.Sites[i].Hosts {
			if h == host {
				return &c.Sites[i]
			}
		}
	}
	return nil
}

// SiteByName returns the site configuration matching the given name.
func (c *Config) SiteByName(name string) *SiteConfig {
	for i := range c.Sites {
		if c.Sites[i].Name == name {
			return &c.Sites[i]
		}
	}
	return nil
}

// SourcesForSite returns the SourceConfigs referenced by a site's sources list.
func (c *Config) SourcesForSite(site *SiteConfig) []SourceConfig {
	var out []SourceConfig
	for _, name := range site.Sources {
		for _, s := range c.Sources {
			if s.Name == name {
				out = append(out, s)
				break
			}
		}
	}
	return out
}

// HasSites returns true if any sites are configured.
func (c *Config) HasSites() bool {
	return len(c.Sites) > 0
}
