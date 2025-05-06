package config

import (
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultSyncInterval = time.Minute
	defaultStatePath    = "caddydnssync.db"
	defaultOwner        = "default"
	defaultLogLevel     = "info"
	defaultLogEnv       = "prod"
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
	Token    string   `yaml:"token"`
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
	configFile := true
	_, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		slog.Default().Warn("fail find config file, proceeding", "path", path)
		configFile = false
	}

	var cfg Config
	if configFile {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}

		decoder := yaml.NewDecoder(f)
		if err := decoder.Decode(&cfg); err != nil {
			return nil, err
		}
		if err := f.Close(); err != nil {
			slog.Default().Warn("fail close config file", "path", path, "error", err)
		}
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

	// Override from environment if set
	if token := os.Getenv("CADDY_DNS_SYNC_CLOUDFLARE_TOKEN"); token != "" {
		cfg.DNS.Token = token
	}
	if syncInterval := os.Getenv("CADDY_DNS_SYNC_INTERVAL"); syncInterval != "" {
		if interval, err := time.ParseDuration(syncInterval); err != nil {
			cfg.SyncInterval = interval
		} else {
			slog.Default().Warn("fail parse sync interval to duration from string", "interval", interval, "error", err)
		}
	}
	if statePath := os.Getenv("CADDY_DNS_SYNC_STATE_PATH"); statePath != "" {
		cfg.StatePath = statePath
	}
	if caddyUrl := os.Getenv("CADDY_DNS_SYNC_CADDY_URL"); caddyUrl != "" {
		cfg.Caddy.AdminURL = caddyUrl
	}
	if dnsProvider := os.Getenv("CADDY_DNS_SYNC_PROVIDER"); dnsProvider != "" {
		cfg.DNS.Provider = dnsProvider
	}
	if dnsZones := os.Getenv("CADDY_DNS_SYNC_ZONES"); dnsZones != "" {
		zones := strings.Split(dnsZones, ",")
		cfg.DNS.Zones = zones
	}
	if dnsTtl := os.Getenv("CADDY_DNS_SYNC_TTL"); dnsTtl != "" {
		if ttl, err := strconv.Atoi(dnsTtl); err != nil {
			cfg.DNS.TTL = ttl
		} else {
			slog.Default().Warn("fail parse ttl to int from string", "ttl", dnsTtl, "error", err)
		}
	}
	if dryRun := os.Getenv("CADDY_DNS_SYNC_DRYRUN"); dryRun != "" {
		switch strings.ToLower(dryRun) {
		case "true":
			cfg.Reconcile.DryRun = true
		case "false":
			cfg.Reconcile.DryRun = false
		default:
			slog.Default().Warn("fail parse dryrun to bool from string", "dryrun", dryRun)
		}
	}
	if owner := os.Getenv("CADDY_DNS_SYNC_OWNER"); owner != "" {
		cfg.Reconcile.Owner = owner
	}
	if protectedRecords := os.Getenv("CADDY_DNS_SYNC_PROTECTED_RECORDS"); protectedRecords != "" {
		records := strings.Split(protectedRecords, ",")
		cfg.Reconcile.ProtectedRecords = records
	}
	if loglevel := os.Getenv("CADDY_DNS_SYNC_LOG_LEVEL"); loglevel != "" {
		cfg.Log.Level = loglevel
	}
	if logenv := os.Getenv("CADDY_DNS_SYNC_LOG_ENV"); logenv != "" {
		cfg.Log.Env = logenv
	}
	return &cfg, nil
}
