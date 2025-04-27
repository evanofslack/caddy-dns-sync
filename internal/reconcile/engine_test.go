package reconcile

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/evanofslack/caddy-dns-sync/internal/config"
	"github.com/evanofslack/caddy-dns-sync/internal/metrics"
	"github.com/evanofslack/caddy-dns-sync/internal/provider"
	"github.com/evanofslack/caddy-dns-sync/internal/source"
	"github.com/evanofslack/caddy-dns-sync/internal/state"
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
	records      map[string][]provider.Record
	createErr    error
	deleteErr    error
	getRecordsErr error
}

func (m *MockProvider) GetRecords(ctx context.Context, zone string) ([]provider.Record, error) {
	return m.records[zone], m.getRecordsErr
}

func (m *MockProvider) CreateRecord(ctx context.Context, zone string, r provider.Record) error {
	return m.createErr
}

func (m *MockProvider) UpdateRecord(ctx context.Context, zone string, r provider.Record) error {
	return nil // Not used in current tests
}

func (m *MockProvider) DeleteRecord(ctx context.Context, zone string, r provider.Record) error {
	return m.deleteErr
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
		// Existing test cases...
		{
			name: "multi-zone handling",
			initialState: state.State{
				Domains: map[string]state.DomainState{},
			},
			currentDomains: []source.DomainConfig{
				{Host: "a.example.com", Upstream: "192.168.1.1:8080"},
				{Host: "b.example.org", Upstream: "192.168.1.2:8080"},
			},
			providerSetup: map[string][]provider.Record{
				"example.com": {},
				"example.org": {},
			},
			config: &config.Config{
				Reconcile: config.Reconcile{
					DryRun:           false,
					ProtectedRecords: []string{},
					Owner:            "test-owner",
				},
				DNS: config.DNS{
					Zones: []string{"example.com", "example.org"},
				},
			},
			expected: Results{
				Created: []provider.Record{
					{Name: "a", Type: "A", Data: "192.168.1.1", TTL: 3600},
					{Name: "a", Type: "TXT", Data: "heritage=caddy-dns-sync,caddy-dns-sync/owner=test-owner", TTL: 3600},
					{Name: "b", Type: "A", Data: "192.168.1.2", TTL: 3600},
					{Name: "b", Type: "TXT", Data: "heritage=caddy-dns-sync,caddy-dns-sync/owner=test-owner", TTL: 3600},
				},
			},
		},
		{
			name: "modified domain with same host",
			initialState: state.State{
				Domains: map[string]state.DomainState{
					"changed.example.com": {ServerName: "old.upstream:8080", LastSeen: now - 100},
				},
			},
			currentDomains: []source.DomainConfig{
				{Host: "changed.example.com", Upstream: "new.upstream:8080"},
			},
			providerSetup: map[string][]provider.Record{
				"example.com": {
					{Name: "changed", Type: "A", Data: "old.upstream"},
					{Name: "changed", Type: "TXT", Data: "heritage=caddy-dns-sync,caddy-dns-sync/owner=test-owner"},
				},
			},
			config: testConfig,
			expected: Results{
				Created: []provider.Record{
					{Name: "changed", Type: "A", Data: "new.upstream", TTL: 3600},
					{Name: "changed", Type: "TXT", Data: "heritage=caddy-dns-sync,caddy-dns-sync/owner=test-owner", TTL: 3600},
				},
				Deleted: []provider.Record{
					{Name: "changed", Type: "A", Data: "old.upstream"},
					{Name: "changed", Type: "TXT", Data: "heritage=caddy-dns-sync,caddy-dns-sync/owner=test-owner"},
				},
			},
		},
		{
			name: "ipv6 address handling",
			initialState: state.State{
				Domains: map[string]state.DomainState{},
			},
			currentDomains: []source.DomainConfig{
				{Host: "ipv6.example.com", Upstream: "[2001:db8::1]:8080"},
			},
			providerSetup: map[string][]provider.Record{
				"example.com": {},
			},
			config: testConfig,
			expected: Results{
				Created: []provider.Record{
					{Name: "ipv6", Type: "AAAA", Data: "2001:db8::1", TTL: 3600},
					{Name: "ipv6", Type: "TXT", Data: "heritage=caddy-dns-sync,caddy-dns-sync/owner=test-owner", TTL: 3600},
				},
			},
		},
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
			name: "failed create skips state save",
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
				Reconcile: config.Reconcile{
					DryRun: false,
					Owner:  "test-owner",
				},
				DNS: config.DNS{
					Zones: []string{"example.com"},
				},
			},
			providerError: errors.New("dns failure"),
			expected: Results{
				Failures: []OperationResult{
					{
						Record: provider.Record{Name: "new", Type: "A", Data: "192.168.1.1", TTL: 3600},
						Op:     "create",
						Error:  "dns failure",
					},
					{
						Record: provider.Record{Name: "new", Type: "TXT", Data: "heritage=caddy-dns-sync,caddy-dns-sync/owner=test-owner", TTL: 3600},
						Op:     "create",
						Error:  "dns failure",
					},
				},
				Created: []provider.Record{},
			},
		},
		{
			name: "failed delete skips state save",
			initialState: state.State{
				Domains: map[string]state.DomainState{
					"old.example.com": {ServerName: "10.0.0.1:8080", LastSeen: time.Now().Unix() - 100},
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
				Reconcile: config.Reconcile{
					DryRun: false,
					Owner:  "test-owner",
				},
				DNS: config.DNS{
					Zones: []string{"example.com"},
				},
			},
			providerError: errors.New("dns failure"),
			expected: Results{
				Failures: []OperationResult{
					{
						Record: provider.Record{Name: "old", Type: "A", Data: "10.0.0.1"},
						Op:     "delete",
						Error:  "dns failure",
					},
					{
						Record: provider.Record{Name: "old", Type: "TXT", Data: "heritage=caddy-dns-sync,caddy-dns-sync/owner=test-owner"},
						Op:     "delete",
						Error:  "dns failure",
					},
				},
				Deleted: []provider.Record{},
			},
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
				records:      tt.providerSetup,
				getRecordsErr: nil, // Allow GetRecords to succeed
				createErr:    tt.providerError,
				deleteErr:    tt.providerError,
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

func TestGetRecordType(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"1.1.1.1", "A"},
		{"2606:4700:4700::1111", "AAAA"},
		{"localhost:443", "CNAME"},
		{"[2001:db8::1]:8080", "AAAA"},
		{"example.com", "CNAME"},
		{"", "CNAME"},
		// New test cases
		{"192.168.1.1", "A"},
		{"[2001:db8::1]", "AAAA"},
		{"mixedcase.EXAMPLE.com", "CNAME"},
		{"with.port:1234", "CNAME"},
		{"invalid.ip:123:456", "CNAME"},
	}

    for _, tt := range tests {
        t.Run(tt.input, func(t *testing.T) {
            got := getRecordType(tt.input)
            if got != tt.want {
                t.Errorf("getRecordType(%q) = %q, want %q", tt.input, got, tt.want)
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
		// New test cases
		{
			name:     "ipv6 address with port",
			input:    "[2001:db8::1]:8080",
			expected: "2001:db8::1",
		},
		{
			name:     "invalid hostport format",
			input:    "invalid-host-port",
			expected: "invalid-host-port",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "multiple colons",
			input:    "host:port:extra",
			expected: "host",
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
