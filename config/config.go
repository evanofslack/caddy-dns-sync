package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultSyncInterval = time.Minute
	defaultStatePath    = "caddy-sync-dns.db"
	defaultOwner        = "default"
	defaultLogLevel = "info"
	defaultLogEnv = "prod"
)

type Config struct {
	SyncInterval time.Duration `yaml:"syncInterval"`
	StatePath    string        `yaml:"statePath"`
	Log          Log           `yaml:"log"`
	Caddy        Caddy         `yaml:"caddy"`
	DNS          DNS           `yaml:"dns"`
	Reconcile    Reconcile     `yaml:"reconcile"`
}

type Caddy struct {
	AdminURL string `yaml:"adminUrl"`
}

type DNS struct {
	Provider string   `yaml:"provider"`
	Zones    []string `yaml:"zones"`
	Token    string   `yaml:"token"` // Value will be overridden by environment variable
	TTL      int      `yaml:"ttl"`
}

type Log struct {
	Level string `yaml:"level"`
	Env   string `yaml:"env"`
}

type Reconcile struct {
	DryRun           bool     `yaml:"dryRun"`
	ProtectedRecords []string `yaml:"protectedRecords"`
	Owner            string   `yaml:"owner"`
}

func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cfg Config
	decoder := yaml.NewDecoder(f)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, err
	}

	if cfg.SyncInterval == 0 {
		cfg.SyncInterval = defaultSyncInterval
	}

	if cfg.StatePath == "" {
		cfg.StatePath = defaultStatePath
	}

	if cfg.Reconcile.Owner == "" {
		cfg.Reconcile.Owner = defaultOwner
	}

	// Set log defaults
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}
	if cfg.Log.Env == "" {
		cfg.Log.Env = "prod"
	}


	// Override token from environment if set
	if token := os.Getenv("CLOUDFLARE_API_TOKEN"); token != "" {
		cfg.DNS.Token = token
	}
	return &cfg, nil
}
