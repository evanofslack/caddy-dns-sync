package state

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v3"
)

func TestBadgerManager(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "badger-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "badger")

	// Create manager
	manager, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer manager.Close()

	// Test cases
	tests := []struct {
		name       string
		stateToSet State
		expected   State
	}{
		{
			name: "empty state",
			stateToSet: State{
				Domains: map[string]DomainState{},
			},
			expected: State{
				Domains: map[string]DomainState{},
			},
		},
		{
			name: "single domain",
			stateToSet: State{
				Domains: map[string]DomainState{
					"example.com": {
						ServerName: "localhost:8080",
						LastSeen:   time.Now().Unix(),
					},
				},
			},
			expected: State{
				Domains: map[string]DomainState{
					"example.com": {
						ServerName: "localhost:8080",
						LastSeen:   time.Now().Unix(),
					},
				},
			},
		},
		{
			name: "multiple domains",
			stateToSet: State{
				Domains: map[string]DomainState{
					"example.com": {
						ServerName: "localhost:8080",
						LastSeen:   time.Now().Unix(),
					},
					"test.com": {
						ServerName: "localhost:9090",
						LastSeen:   time.Now().Unix(),
					},
				},
			},
			expected: State{
				Domains: map[string]DomainState{
					"example.com": {
						ServerName: "localhost:8080",
						LastSeen:   time.Now().Unix(),
					},
					"test.com": {
						ServerName: "localhost:9090",
						LastSeen:   time.Now().Unix(),
					},
				},
			},
		},
		{
			name: "remove domain",
			stateToSet: State{
				Domains: map[string]DomainState{
					"example.com": {
						ServerName: "localhost:8080",
						LastSeen:   time.Now().Unix(),
					},
				},
			},
			expected: State{
				Domains: map[string]DomainState{
					"example.com": {
						ServerName: "localhost:8080",
						LastSeen:   time.Now().Unix(),
					},
				},
			},
		},
		{
			name: "update domain",
			stateToSet: State{
				Domains: map[string]DomainState{
					"example.com": {
						ServerName: "localhost:8081",
						LastSeen:   time.Now().Unix(),
					},
				},
			},
			expected: State{
				Domains: map[string]DomainState{
					"example.com": {
						ServerName: "localhost:8081",
						LastSeen:   time.Now().Unix(),
					},
				},
			},
		},
		{
			name: "clear all domains",
			stateToSet: State{
				Domains: map[string]DomainState{},
			},
			expected: State{
				Domains: map[string]DomainState{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if err := manager.SaveState(ctx, tt.stateToSet); err != nil {
				t.Fatalf("SaveState failed: %v", err)
			}

			loaded, err := manager.LoadState(ctx)
			if err != nil {
				t.Fatalf("LoadState failed: %v", err)
			}

			if !reflect.DeepEqual(loaded, tt.expected) {
				t.Errorf("Expected %+v but got %+v", tt.expected, loaded)
			}
		})
	}
}

func TestBadgerManagerDirect(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "badger-direct-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "badger")

	// Test direct DB access
	db, err := badger.Open(badger.DefaultOptions(dbPath).WithLogger(nil))
	if err != nil {
		t.Fatalf("failed to open badger db: %v", err)
	}

	testDomain := DomainState{
		ServerName: "localhost:1234",
		LastSeen:   time.Now().Unix(),
	}

	// Manually insert a domain
	txn := db.NewTransaction(true)
	data, _ := json.Marshal(testDomain)
	err = txn.Set([]byte(domainPrefix+"direct.com"), data)
	if err != nil {
		t.Fatalf("failed to set value: %v", err)
	}
	err = txn.Commit()
	if err != nil {
		t.Fatalf("failed to commit: %v", err)
	}
    if err := db.Close(); err != nil {
        t.Fatal(err)
    }

	// Now open with manager and test
	manager, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer manager.Close()

	ctx := context.Background()
	state, err := manager.LoadState(ctx)
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	expected := State{
		Domains: map[string]DomainState{
			"direct.com": testDomain,
		},
	}

	if !reflect.DeepEqual(state, expected) {
		t.Errorf("Expected %+v but got %+v", expected, state)
	}
}

func TestBadgerManagerError(t *testing.T) {
	// Try to create manager with invalid path
	_, err := New("/nonexistent/path/that/cannot/be/created")
	if err == nil {
		t.Fatal("expected error for invalid path but got nil")
	}
}
