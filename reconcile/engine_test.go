package reconcile

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/evanofslack/caddy-dns-sync/config"
	"github.com/evanofslack/caddy-dns-sync/metrics"
	"github.com/evanofslack/caddy-dns-sync/provider"
	"github.com/evanofslack/caddy-dns-sync/source"
	"github.com/evanofslack/caddy-dns-sync/state"
)

type MockStateManager struct {
	state state.State
	err   error
}

func (m *MockStateManager) LoadState(ctx context.Context) (state.State, error) { return m.state, m.err }
func (m *MockStateManager) SaveState(ctx context.Context, s state.State) error {
	m.state = s
	return m.err
}
func (m *MockStateManager) Close() error { return nil }

type MockProvider struct {
	records map[string][]provider.Record
	err     error
}

func (m *MockProvider) GetRecords(ctx context.Context, zone string) ([]provider.Record, error) {
	return m.records[zone], m.err
}

func (m *MockProvider) CreateRecord(ctx context.Context, zone string, r provider.Record) error {
	return m.err
}

func (m *MockProvider) UpdateRecord(ctx context.Context, zone string, r provider.Record) error {
	return m.err
}

func (m *MockProvider) DeleteRecord(ctx context.Context, zone string, r provider.Record) error {
	return m.err
}

func TestEngine(t *testing.T) {
	now := time.Now().Unix()
	testConfig := &config.Config{
		Reconcile: config.Reconcile{
			DryRun:           false,
			ProtectedRecords: []string{"protected.example.com"},
			Owner:            "test-owner",
		},
		DNS: config.DNS{
			Zones: []string{"example.com"},
		},
	}

	tests := []struct {
		name           string
		initialState   state.State
		currentDomains []source.DomainConfig
		providerSetup  map[string][]provider.Record
		config         *config.Config
		stateError     error
		providerError  error
		expected       Results
		expectError    bool
	}{
		{
			name: "new domain creation",
			initialState: state.State{
				Domains: map[string]state.DomainState{},
			},
			currentDomains: []source.DomainConfig{
				{Host: "new.example.com", Upstream: "192.168.1.1:8080"},
			},
			providerSetup: map[string][]provider.Record{
				"example.com": {},
			},
			config: &config.Config{
				Reconcile: testConfig.Reconcile,
				DNS: config.DNS{
					Zones: []string{"example.com"},
				},
			},
			expected: Results{
				Created: []provider.Record{
					{Name: "new", Type: "A", Data: "192.168.1.1", TTL: 3600},
					{Name: "new", Type: "TXT", Data: "heritage=caddy-dns-sync,caddy-dns-sync/owner=test-owner", TTL: 3600},
				},
			},
		},
		{
			name: "domain removal",
			initialState: state.State{
				Domains: map[string]state.DomainState{
					"old.example.com": {ServerName: "10.0.0.1:8080", LastSeen: now - 100},
				},
			},
			currentDomains: []source.DomainConfig{},
			providerSetup: map[string][]provider.Record{
				"example.com": {
					{Name: "old", Type: "A", Data: "10.0.0.1"},
					{Name: "old", Type: "TXT", Data: "heritage=caddy-dns-sync,caddy-dns-sync/owner=test-owner"},
				},
			},
			config: &config.Config{
				Reconcile: testConfig.Reconcile,
				DNS: config.DNS{
					Zones: []string{"example.com"},
				},
			},
			expected: Results{
				Deleted: []provider.Record{
					{Name: "old", Type: "A", Data: "10.0.0.1"},
					{Name: "old", Type: "TXT", Data: "heritage=caddy-dns-sync,caddy-dns-sync/owner=test-owner"},
				},
			},
		},
		{
			name: "unmanaged record deletion skip",
			initialState: state.State{
				Domains: map[string]state.DomainState{
					"unmanaged.example.com": {ServerName: "10.0.0.1:8080", LastSeen: now - 100},
				},
			},
			currentDomains: []source.DomainConfig{},
			providerSetup: map[string][]provider.Record{
				"example.com": {
					{Name: "unmanaged", Type: "A", Data: "10.0.0.1"},
				},
			},
			config: &config.Config{
				Reconcile: testConfig.Reconcile,
				DNS: config.DNS{
					Zones: []string{"example.com"},
				},
			},
			expected: Results{
				Deleted: []provider.Record{},
			},
		},
		{
			name: "mismatched owner deletion skip",
			initialState: state.State{
				Domains: map[string]state.DomainState{
					"wrongowner.example.com": {ServerName: "10.0.0.1:8080", LastSeen: now - 100},
				},
			},
			currentDomains: []source.DomainConfig{},
			providerSetup: map[string][]provider.Record{
				"example.com": {
					{Name: "wrongowner", Type: "A", Data: "10.0.0.1"},
					{Name: "wrongowner", Type: "TXT", Data: "heritage=caddy-dns-sync,caddy-dns-sync/owner=other-owner"},
				},
			},
			config: &config.Config{
				Reconcile: testConfig.Reconcile,
				DNS: config.DNS{
					Zones: []string{"example.com"},
				},
			},
			expected: Results{
				Deleted: []provider.Record{},
			},
		},
		{
			name: "protected record skip",
			initialState: state.State{
				Domains: map[string]state.DomainState{},
			},
			currentDomains: []source.DomainConfig{
				{Host: "protected.example.com", Upstream: "10.0.0.1:8080"},
			},
			providerSetup: map[string][]provider.Record{
				"example.com": {},
			},
			config: &config.Config{
				Reconcile: testConfig.Reconcile,
				DNS: config.DNS{
					Zones: []string{"example.com"},
				},
			},
			expected: Results{
				Created: []provider.Record{},
			},
		},
		{
			name: "cname creation",
			initialState: state.State{
				Domains: map[string]state.DomainState{},
			},
			currentDomains: []source.DomainConfig{
				{Host: "api.example.com", Upstream: "reroute.com"},
			},
			providerSetup: map[string][]provider.Record{
				"example.com": {},
			},
			config: &config.Config{
				Reconcile: testConfig.Reconcile,
				DNS: config.DNS{
					Zones: []string{"example.com"},
				},
			},
			expected: Results{
				Created: []provider.Record{
					{Name: "api", Type: "CNAME", Data: "reroute.com", TTL: 3600},
					{Name: "api", Type: "TXT", Data: "heritage=caddy-dns-sync,caddy-dns-sync/owner=test-owner"},
				},
			},
		},
		{
			name:         "state load failure",
			initialState: state.State{},
			stateError:   errors.New("state error"),
			config:       testConfig,
			expectError:  true,
		},
		{
			name: "dry run mode",
			initialState: state.State{
				Domains: map[string]state.DomainState{},
			},
			currentDomains: []source.DomainConfig{
				{Host: "dryrun.example.com", Upstream: "192.168.1.1:8080"},
			},
			providerSetup: map[string][]provider.Record{
				"example.com": {},
			},
			config: &config.Config{
				Reconcile: config.Reconcile{
					DryRun: true,
					Owner: "test-owner",
				},
				DNS: config.DNS{
					Zones: []string{"example.com"},
				},
			},
			expected: Results{
				Created: []provider.Record{
					{Name: "dryrun", Type: "A", Data: "192.168.1.1", TTL: 3600},
					{Name: "dryrun", Type: "TXT", Data: "heritage=caddy-dns-sync,caddy-dns-sync/owner=test-owner"},
				},
			},
		},
	}

	for _, tt := range tests {
		ctx := context.Background()
		t.Run(tt.name, func(t *testing.T) {
			stateManager := &MockStateManager{
				state: tt.initialState,
				err:   tt.stateError,
			}

			provider := &MockProvider{
				records: tt.providerSetup,
				err:     tt.providerError,
			}

			metrics := metrics.New(false)
			engine := NewEngine(stateManager, provider, tt.config, metrics)
			results, err := engine.Reconcile(ctx, tt.currentDomains)

			if tt.expectError && err == nil {
				t.Fatal("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(results.Created) != len(tt.expected.Created) {
				t.Errorf("Created records mismatch: got %d, want %d", len(results.Created), len(tt.expected.Created))
			}

			if len(results.Deleted) != len(tt.expected.Deleted) {
				t.Errorf("Deleted records mismatch: got %d, want %d", len(results.Deleted), len(tt.expected.Deleted))
			}

			if tt.config.Reconcile.DryRun && len(stateManager.state.Domains) > 0 {
				t.Error("Dry run mode should not persist state changes")
			}
		})
	}
}

func TestExtractHostname(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "extract host from upstream with port",
			input:    "backend:8080",
			expected: "backend",
		},
		{
			name:     "extract host from upstream without port",
			input:    "backend",
			expected: "backend",
		},
		{
			name:     "extract host from ip with port",
			input:    "192.168.1.1:443",
			expected: "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractHostFromUpstream(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestExtractZone(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "extract zone subdomain",
			input:    "sub.example.com",
			expected: "example.com",
		},
		{
			name:     "extract zone deep subdomain",
			input:    "sub1.sub2.example.com",
			expected: "example.com",
		},
		{
			name:     "extract zone top level",
			input:    "example.com",
			expected: "example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractZone(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}
