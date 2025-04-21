package reconcile

import (
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/evanofslack/caddy-dns-sync/config"
	"github.com/evanofslack/caddy-dns-sync/provider"
	"github.com/evanofslack/caddy-dns-sync/source"
	"github.com/evanofslack/caddy-dns-sync/state"
)

type Engine interface {
	Reconcile(domains []source.DomainConfig) (Results, error)
}

type engine struct {
	stateManager state.Manager
	dnsProvider  provider.Provider
	dryRun       bool
	protected    map[string]bool
}

func NewEngine(sm state.Manager, dp provider.Provider, cfg *config.Config) Engine {
	protected := make(map[string]bool)
	for _, r := range cfg.Reconcile.ProtectedRecords {
		protected[r] = true
	}
	return &engine{
		stateManager: sm,
		dnsProvider:  dp,
		dryRun:       cfg.Reconcile.DryRun,
		protected:    protected,
	}
}

func (e *engine) Reconcile(domains []source.DomainConfig) (Results, error) {
	// Load current state
	prevState, err := e.stateManager.LoadState()
	if err != nil {
		return Results{}, fmt.Errorf("load state: %w", err)
	}

	// Build new state from current domains
	currentState := state.State{
		Domains: make(map[string]state.DomainState),
	}

	for _, d := range domains {
		currentState.Domains[d.Host] = state.DomainState{
			ServerName: d.Upstream,
			LastSeen:   time.Now().Unix(),
		}
	}

	// Compare states to find changes
	changes := e.compareStates(currentState, prevState)
	slog.Info("State comparison", "added", len(changes.Added), "removed", len(changes.Removed))

	// Generate and execute plan
	plan, err := e.generatePlan(changes)
	if err != nil {
		return Results{}, fmt.Errorf("generate plan: %w", err)
	}

	results, err := e.executePlan(plan)
	if err != nil {
		return results, fmt.Errorf("execute plan: %w", err)
	}

	// Save new state
	if !e.dryRun {
		if err := e.stateManager.SaveState(currentState); err != nil {
			return results, fmt.Errorf("save state: %w", err)
		}
	}

	return results, nil
}

func (e *engine) compareStates(current, previous state.State) state.StateChanges {
	changes := state.StateChanges{
		Added:   []source.DomainConfig{},
		Removed: []string{},
	}

	// Find added or modified domains
	for host, domainCfg := range current.Domains {
		if prev, exists := previous.Domains[host]; !exists || prev.ServerName != domainCfg.ServerName {
			changes.Added = append(changes.Added, source.DomainConfig{
				Host:       host,
				Upstream: domainCfg.ServerName,
			})
		}
	}

	// Find removed domains
	for host := range previous.Domains {
		if _, exists := current.Domains[host]; !exists {
			changes.Removed = append(changes.Removed, host)
		}
	}
	return changes
}

func (e *engine) generatePlan(changes state.StateChanges) (Plan, error) {
	plan := Plan{
		Create: []provider.Record{},
		Delete: []provider.Record{},
	}

	// For each zone we're managing
	// TODO: Get zones from config
	zones := []string{"example.com"}

	for _, zone := range zones {
		// Get existing records
		records, err := e.dnsProvider.GetRecords(zone)
		if err != nil {
			return plan, fmt.Errorf("get records for zone %s: %w", zone, err)
		}

		recordMap := make(map[string]provider.Record)
		for _, r := range records {
			if r.Type == "A" || r.Type == "CNAME" {
				recordMap[r.Name] = r
			}
		}

		// Process additions
		for _, domain := range changes.Added {
			if !belongsToZone(domain.Host, zone) {
				continue
			}

			recordName := getRecordName(domain.Host, zone)
			if e.isProtected(domain.Host) {
				slog.Warn("Skipping protected record", "name", recordName, "zone", zone)
				continue
			}

			plan.Create = append(plan.Create, provider.Record{
				Name: recordName,
				Type: getRecordType(extractHostFromUpstream(domain.Upstream)),
				Data: extractHostFromUpstream(domain.Upstream),
				TTL:  3600,        // This should be configurable
			})
		}

		// Process removals
		for _, host := range changes.Removed {
			if !belongsToZone(host, zone) {
				continue
			}

			recordName := getRecordName(host, zone)
			if e.isProtected(recordName) {
				slog.Warn("Skipping protected record", "name", recordName, "zone", zone)
				continue
			}

			if record, exists := recordMap[recordName]; exists {
				plan.Delete = append(plan.Delete, record)
			}
		}
	}
	return plan, nil
}

func (e *engine) executePlan(plan Plan) (Results, error) {
	results := Results{}

	if e.dryRun {
		slog.Info("Dry run mode - would create records", "count", len(plan.Create))
		slog.Info("Dry run mode - would delete records", "count", len(plan.Delete))

		results.Created = make([]provider.Record, len(plan.Create))
		copy(results.Created, plan.Create)

		results.Deleted = make([]provider.Record, len(plan.Delete))
		copy(results.Deleted, plan.Delete)
		return results, nil
	}

	// Execute creates
	for _, record := range plan.Create {
		// Get zone from record
		zone := extractZone(record.Name)

		if err := e.dnsProvider.CreateRecord(zone, record); err != nil {
			slog.Error("Failed to create record", "name", record.Name, "error", err)
			results.Failures = append(results.Failures, OperationResult{
				Record: record,
				Op:     "create",
				Error:  err.Error(),
			})
		} else {
			results.Created = append(results.Created, record)
		}
	}

	// Execute deletes
	for _, record := range plan.Delete {
		zone := extractZone(record.Name)

		if err := e.dnsProvider.DeleteRecord(zone, record); err != nil {
			slog.Error("Failed to delete record", "name", record.Name, "error", err)
			results.Failures = append(results.Failures, OperationResult{
				Record: record,
				Op:     "delete",
				Error:  err.Error(),
			})
		} else {
			results.Deleted = append(results.Deleted, record)
		}
	}

	return results, nil
}

func (e *engine) isProtected(name string) bool {
	return e.protected[name]
}

func belongsToZone(host, zone string) bool {
	return strings.HasSuffix(host, zone) || host == zone
}

func getRecordName(host, zone string) string {
	if host == zone {
		return "@"
	}
	return strings.TrimSuffix(host, "."+zone)
}

func getRecordType(host string) string {
	if ip := net.ParseIP(host); ip != nil {
		return "A"
	}
	return "CNAME"
}

func extractHostFromUpstream(upstream string) string {
	if upstream == "" {
		return ""
	}
	
	host, _, err := net.SplitHostPort(upstream)
	if err != nil {
		// If no port, use entire string
		return upstream
	}
	return host
}

func extractZone(recordName string) string {
	parts := strings.Split(recordName, ".")
	if len(parts) < 2 {
		return recordName
	}
	return strings.Join(parts[len(parts)-2:], ".")
}
