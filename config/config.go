package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	SyncInterval    time.Duration   `yaml:"syncInterval"`
	StatePath       string          `yaml:"statePath"`
	CaddyConfig     CaddyConfig     `yaml:"caddy"`
	DNSConfig       DNSConfig       `yaml:"dns"`
	ReconcileConfig ReconcileConfig `yaml:"reconcile"`
}

type CaddyConfig struct {
	AdminURL string `yaml:"adminUrl"`
}

type DNSConfig struct {
	Provider    string            `yaml:"provider"`
	Zones       []string          `yaml:"zones"`
	Credentials map[string]string `yaml:"credentials"`
	TTL         int               `yaml:"ttl"`
}

type ReconcileConfig struct {
	DryRun           bool     `yaml:"dryRun"`
	ProtectedRecords []string `yaml:"protectedRecords"`
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
		cfg.SyncInterval = 5 * time.Minute
	}

	if cfg.StatePath == "" {
		cfg.StatePath = "caddy-dns-sync.db"
	}
	return &cfg, nil
}
