package reconcile

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/evanofslack/caddy-dns-sync/internal/config"
	"github.com/evanofslack/caddy-dns-sync/internal/metrics"
	"github.com/evanofslack/caddy-dns-sync/internal/provider"
	"github.com/evanofslack/caddy-dns-sync/internal/source"
	"github.com/evanofslack/caddy-dns-sync/internal/state"
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
	slog.Debug("State comparison", "added", len(changes.Added), "removed", len(changes.Removed))
	if changes.IsEmpty() {
		slog.Info("No state changes, ending reconciliation")
		return Results{}, nil
	}

	// Generate and execute plan
	plan, err := e.generatePlan(ctx, changes)
	if err != nil {
		return Results{}, fmt.Errorf("generate plan: %w", err)
	}

	results, err := e.executePlan(ctx, plan, currentState)
	if err != nil {
		return results, fmt.Errorf("execute plan: %w", err)
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
			slog.Debug("Got record", "name", r.Name, "type", r.Type, "data", r.Data)
			recordName := getRecordName(r.Name, zone)
			switch r.Type {
			case "A", "CNAME":
				recordMap[recordName] = r
			case "TXT":
				if strings.Contains(r.Data, "heritage=caddy-dns-sync") && strings.Contains(r.Data, "caddy-dns-sync/owner="+e.cfg.Reconcile.Owner) {
					managedTXTRecords[recordName] = r
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
			desiredData := host

			// Check if existing records need to be updated
			existingMainRecord, mainExists := recordMap[recordName]
			existingTXTRecord, txtExists := managedTXTRecords[recordName]

			// If existing records match desired state, skip creation
			if mainExists && txtExists &&
				existingMainRecord.Data == desiredData &&
				existingTXTRecord.Data == txtIdentifier(e.cfg.Reconcile.Owner) {
				continue
			}

			// If existing records don't match, plan to delete them first
			if mainExists {
				plan.Delete = append(plan.Delete, existingMainRecord)
				e.metrics.IncDNSOperation("delete", zone, existingMainRecord.Type)
			}
			if txtExists {
				plan.Delete = append(plan.Delete, existingTXTRecord)
				e.metrics.IncDNSOperation("delete", zone, "TXT")
			}

			// Create new records
			mainRecord := provider.Record{
				Name: recordName,
				Type: recordType,
				Data: desiredData,
				TTL:  3600, // TODO: This should be configurable
				Zone: zone,
			}
			plan.Create = append(plan.Create, mainRecord)
			e.metrics.IncDNSOperation("create", zone, recordType)

			txtRecord := provider.Record{
				Name: recordName,
				Type: "TXT",
				Data: txtIdentifier(e.cfg.Reconcile.Owner),
				TTL:  3600,
				Zone: zone,
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
					slog.Debug("TXT record check", "recordName", recordName, "exists", txtExists, "managedRecords", managedTXTRecords)
					e.metrics.IncDNSOperation("skip", zone, recordType)
					continue
				}
				plan.Delete = append(plan.Delete, record)
				e.metrics.IncDNSOperation("delete", zone, recordType)
			}

			// Delete associated TXT record if managed
			if txtRecord, exists := managedTXTRecords[recordName]; exists {
                // txtRecord.Data = txtIdentifier(e.cfg.Reconcile.Owner) // cf check
			    // Set data to empty to match all data, we already know its correct
				plan.Delete = append(plan.Delete, txtRecord)
				e.metrics.IncDNSOperation("delete", zone, "TXT")
			}
		}
	}
	return plan, nil
}

func (e *engine) executePlan(ctx context.Context, plan Plan, newState state.State) (Results, error) {
	results := Results{}
	slog.Info("Execution mode", "dryRun", e.dryRun)

	if e.dryRun {
		slog.Info("Dry run mode - would create records", "count", len(plan.Create))
		slog.Info("Dry run mode - would delete records", "count", len(plan.Delete))

		results.Created = make([]provider.Record, len(plan.Create))
		copy(results.Created, plan.Create)

		results.Deleted = make([]provider.Record, len(plan.Delete))
		copy(results.Deleted, plan.Delete)
		// In dry-run mode, return early without saving state
		results.Created = make([]provider.Record, len(plan.Create))
		copy(results.Created, plan.Create)
		results.Deleted = make([]provider.Record, len(plan.Delete))
		copy(results.Deleted, plan.Delete)
		return results, nil
	}

	// Execute creates
	for _, record := range plan.Create {
		slog.Debug("Start execute create from plan", "name", record.Name, "type", record.Type, "data", record.Data, "zone", record.Zone)
		if err := e.dnsProvider.CreateRecord(ctx, record.Zone, record); err != nil {
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
		slog.Debug("Start execute delete from plan", "name", record.Name, "type", record.Type, "data", record.Data, "zone", record.Zone)
		if err := e.dnsProvider.DeleteRecord(ctx, record.Zone, record); err != nil {
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

	// Only persist state if all operations succeeded
	if len(results.Failures) == 0 {
		if err := e.stateManager.SaveState(ctx, newState); err != nil {
			return results, fmt.Errorf("save state: %w", err)
		}
	} else {
		slog.Warn("Not persisting state due to failed operations", "failures", len(results.Failures))
	}

	return results, nil
}

func (e *engine) isProtected(name string) bool {
	return e.protected[name]
}

func belongsToZone(host, zone string) bool {
	// Match exact zone or subdomains with dot separator
	slog.Debug("Zone check", "host", host, "zone", zone, "matches", host == zone || strings.HasSuffix(host, "."+zone))
	return host == zone || strings.HasSuffix(host, "."+zone)
}

func getRecordName(host, zone string) string {
	name := strings.TrimSuffix(host, "."+zone)
	slog.Debug("Record name extraction", "host", host, "zone", zone, "result", name)
	if host == zone {
		return "@"
	}
	return name
}

func getRecordType(host string) string {
	// Handle IPv6 in brackets with or without port
	if strings.HasPrefix(host, "[") {
		// Try to handle as host:port format first
		if strings.Contains(host, "]:") {
			if ipstr, _, err := net.SplitHostPort(host); err == nil {
				host = ipstr
			}
		} else {
			// Just brackets without port
			host = strings.TrimPrefix(host, "[")
			host = strings.TrimSuffix(host, "]")
		}
	}

	// Then try parsing as pure IP
	if ip := net.ParseIP(host); ip != nil {
		if ip.To4() != nil {
			return "A"
		}
		return "AAAA"
	}

	// Then try parsing as host:port format
	if ipstr, _, err := net.SplitHostPort(host); err == nil {
		if ip := net.ParseIP(ipstr); ip != nil {
			if ip.To4() != nil {
				return "A"
			}
			return "AAAA"
		}
	}
	
	// Fallback to CNAME
	return "CNAME"
}

func extractHostFromUpstream(upstream string) string {
	if upstream == "" {
		return ""
	}

	// Handle IPv6 addresses with port first
	if strings.HasPrefix(upstream, "[") && strings.Contains(upstream, "]:") {
		if host, _, err := net.SplitHostPort(upstream); err == nil {
			return strings.Trim(host, "[]")
		}
	}

	// Split on first colon to handle invalid formats like "host:port:extra"
	if host, _, ok := strings.Cut(upstream, ":"); ok {
		return host
	}
	return upstream
}

// TXT record used to identify managed records
func txtIdentifier(owner string) string {
	return fmt.Sprintf("heritage=caddy-dns-sync,caddy-dns-sync/owner=%s", owner)
}
