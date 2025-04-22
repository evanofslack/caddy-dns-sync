package reconcile

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/evanofslack/caddy-dns-sync/config"
	"github.com/evanofslack/caddy-dns-sync/metrics"
	"github.com/evanofslack/caddy-dns-sync/provider"
	"github.com/evanofslack/caddy-dns-sync/source"
	"github.com/evanofslack/caddy-dns-sync/state"
)

type Engine interface {
	Reconcile(ctx context.Context, domains []source.DomainConfig) (Results, error)
}

type engine struct {
	stateManager state.Manager
	dnsProvider  provider.Provider
	dryRun       bool
	protected    map[string]bool
	zones        []string
	metrics      *metrics.Metrics
	cfg          *config.Config
}

func NewEngine(sm state.Manager, dp provider.Provider, cfg *config.Config, metrics *metrics.Metrics) *engine {
	protected := make(map[string]bool)
	for _, r := range cfg.Reconcile.ProtectedRecords {
		protected[r] = true
	}
	return &engine{
		stateManager: sm,
		dnsProvider:  dp,
		dryRun:       cfg.Reconcile.DryRun,
		protected:    protected,
		zones:        cfg.DNS.Zones,
		metrics:      metrics,
		cfg:          cfg,
	}
}

func (e *engine) Reconcile(ctx context.Context, domains []source.DomainConfig) (Results, error) {
	// Load current state
	prevState, err := e.stateManager.LoadState(ctx)
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
	plan, err := e.generatePlan(ctx, changes)
	if err != nil {
		return Results{}, fmt.Errorf("generate plan: %w", err)
	}

	results, err := e.executePlan(ctx, plan)
	if err != nil {
		return results, fmt.Errorf("execute plan: %w", err)
	}

	// Save new state
	if !e.dryRun {
		if err := e.stateManager.SaveState(ctx, currentState); err != nil {
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
				Host:     host,
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

func (e *engine) generatePlan(ctx context.Context, changes state.StateChanges) (Plan, error) {
	plan := Plan{
		Create: []provider.Record{},
		Delete: []provider.Record{},
	}

	for _, zone := range e.zones {
		// Get existing records
		records, err := e.dnsProvider.GetRecords(ctx, zone)
		if err != nil {
			return plan, fmt.Errorf("get records for zone %s: %w", zone, err)
		}
		slog.Info("Got records from dns provider", "count", len(records))

		recordMap := make(map[string]provider.Record)
		managedTXTRecords := make(map[string]provider.Record)
		for _, r := range records {
			switch r.Type {
			case "A", "CNAME":
				recordMap[r.Name] = r
				slog.Debug("Got record", "name", r.Name, "type", r.Type)
			case "TXT":
				if strings.HasPrefix(r.Data, "heritage=caddy-dns-sync") && strings.Contains(r.Data, "caddy-dns-sync/owner="+e.cfg.Reconcile.Owner) {
					managedTXTRecords[r.Name] = r
				}
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

			host := extractHostFromUpstream(domain.Upstream)
			recordType := getRecordType(host)
			mainRecord := provider.Record{
				Name: recordName,
				Type: recordType,
				Data: host,
				TTL:  3600, // TODO: This should be configurable
			}
			plan.Create = append(plan.Create, mainRecord)
			e.metrics.IncDNSOperation("create", zone, recordType)

			// Add managed TXT record
			txtRecord := provider.Record{
				Name: recordName,
				Type: "TXT",
				Data: fmt.Sprintf("heritage=caddy-dns-sync,caddy-dns-sync/owner=%s", e.cfg.Reconcile.Owner),
				TTL:  3600,
			}
			plan.Create = append(plan.Create, txtRecord)
			e.metrics.IncDNSOperation("create", zone, "TXT")
		}

		// Process removals
		for _, host := range changes.Removed {
			if !belongsToZone(host, zone) {
				continue
			}

			recordName := getRecordName(host, zone)
			recordType := getRecordType(host)
			if e.isProtected(recordName) {
				slog.Info("Skipping delete protected record", "name", recordName, "zone", zone, "record_type", recordType)
				continue
			}

			// If entry has been removed and associated DNS record exists, plan to delete it
			if record, exists := recordMap[recordName]; exists {
				// But only delete if we manage it, confirmed by checking existance of txt record
				if _, txtExists := managedTXTRecords[recordName]; !txtExists {
					slog.Warn("Skipping delete record without associated owned TXT record", "name", recordName, "zone", zone, "record_type", recordType)
					e.metrics.IncDNSOperation("skip", zone, recordType)
					continue
				}
				plan.Delete = append(plan.Delete, record)
				e.metrics.IncDNSOperation("delete", zone, recordType)
			}

			// Delete associated TXT record if managed
			if txtRecord, exists := managedTXTRecords[recordName]; exists {
				plan.Delete = append(plan.Delete, txtRecord)
				e.metrics.IncDNSOperation("delete", zone, "TXT")
			}
		}
	}
	return plan, nil
}

func (e *engine) executePlan(ctx context.Context, plan Plan) (Results, error) {
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

		if err := e.dnsProvider.CreateRecord(ctx, zone, record); err != nil {
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

		if err := e.dnsProvider.DeleteRecord(ctx, zone, record); err != nil {
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
	// Match exact zone or subdomains with dot separator
	return host == zone || strings.HasSuffix(host, "."+zone)
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
