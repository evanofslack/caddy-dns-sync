package state

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dgraph-io/badger/v3"
	"github.com/evanofslack/caddy-dns-sync/internal/metrics"
)

const domainPrefix = "domain:"

type Manager interface {
	LoadState(ctx context.Context) (State, error)
	SaveState(ctx context.Context, state State) error
	Close() error
}

type badgerManager struct {
	db      *badger.DB
	metrics *metrics.Metrics
}

func New(path string, metrics *metrics.Metrics) (Manager, error) {
	opts := badger.DefaultOptions(path)
	opts.Logger = nil // Disable Badger's internal logger

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("open badger db: %w", err)
	}
	m := &badgerManager{db: db, metrics: metrics}
	return m, nil
}

func (m *badgerManager) LoadState(ctx context.Context) (State, error) {
	state := State{
		Domains: make(map[string]DomainState),
	}

	err := m.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()

		prefix := []byte(domainPrefix)
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			key := string(item.Key())
			host := key[len(domainPrefix):]

			err := item.Value(func(val []byte) error {
				var domain DomainState
				if err := json.Unmarshal(val, &domain); err != nil {
					return err
				}
				state.Domains[host] = domain
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	m.metrics.IncBadgerRequest("read", err == nil)
	return state, err
}

func (m *badgerManager) SaveState(ctx context.Context, state State) error {
	txn := m.db.NewTransaction(true)
	defer txn.Discard()

	// First, get all existing keys to handle deletions
	existingHosts := make(map[string]bool)

	it := txn.NewIterator(badger.DefaultIteratorOptions)
	prefix := []byte(domainPrefix)
	for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
		key := string(it.Item().Key())
		host := key[len(domainPrefix):]
		existingHosts[host] = true
	}
	it.Close()

	// Store current domains
	for host, domain := range state.Domains {
		data, err := json.Marshal(domain)
		if err != nil {
			m.metrics.IncBadgerRequest("update", false)
			return err
		}
		key := domainPrefix + host
		if err := txn.Set([]byte(key), data); err != nil {
			m.metrics.IncBadgerRequest("update", false)
			return err
		}
		// Remove from existingHosts to track what's been kept
		delete(existingHosts, host)
	}

	// Delete hosts that are no longer present
	for host := range existingHosts {
		key := domainPrefix + host
		if err := txn.Delete([]byte(key)); err != nil {
			m.metrics.IncBadgerRequest("delete", false)
			return err
		}
	}
	err := txn.Commit()
	m.metrics.IncBadgerRequest("update", err == nil)
	return err
}

func (m *badgerManager) Close() error {
	return m.db.Close()
}
