package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config is the top-level resolved configuration for Marrow.
type Config struct {
	Server      ServerConfig    `mapstructure:"server"`
	GitHub      GitHubConfig    `mapstructure:"github"`
	Search      SearchConfig    `mapstructure:"search"`
	Sync        SyncConfig      `mapstructure:"sync"`
	GitHubApp   GitHubAppConfig `mapstructure:"github_app"`
	Embedding   EmbeddingConfig `mapstructure:"embedding"`
	Sources     []SourceConfig  `mapstructure:"sources"`
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

type ServerConfig struct {
	Addr string `mapstructure:"addr"`
	DB   string `mapstructure:"db"`
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

	// GitHub
	v.SetDefault("github.repo_url", "")
	v.SetDefault("github.repo_token", "")
	v.SetDefault("github.webhook_secret", "")
	v.SetDefault("github.source", "github")
	v.SetDefault("github.local_path", "./repo")

	// Search
	v.SetDefault("search.detect_lang", true)
	v.SetDefault("search.default_lang", "en")

	// Sync
	v.SetDefault("sync.dir", ".")
	v.SetDefault("sync.source", "local")
	v.SetDefault("sync.default_lang", "en")

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
