package cloudflare

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/evanofslack/caddy-dns-sync/internal/config"
	"github.com/evanofslack/caddy-dns-sync/internal/metrics"
	"github.com/evanofslack/caddy-dns-sync/internal/provider"
)

type CloudflareProvider struct {
	client  *cloudflare.API
	metrics *metrics.Metrics
	ttl     int
	zones   map[string]string // Cache zone name to ID mapping
}

func New(cfg config.DNS, metrics *metrics.Metrics) (*CloudflareProvider, error) {
	token := cfg.Token
	if token == "" {
		return nil, fmt.Errorf("cloudflare API token required")
	}

	client, err := cloudflare.NewWithAPIToken(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create Cloudflare client: %w", err)
	}

	// Pre-cache zone IDs for all configured zones
	zoneCache := make(map[string]string)
	for _, zone := range cfg.Zones {
		id, err := client.ZoneIDByName(zone)
		if err != nil {
			return nil, fmt.Errorf("failed to get zone ID for %s: %w", zone, err)
		}
		zoneCache[zone] = id
	}

	return &CloudflareProvider{
		client:  client,
		metrics: metrics,
		ttl:     cfg.TTL,
		zones:   zoneCache,
	}, nil
}

func (p *CloudflareProvider) GetRecords(ctx context.Context, zone string) ([]provider.Record, error) {
	slog.Info("Getting DNS records", "zone", zone)
	start := time.Now()

	zoneID, ok := p.zones[zone]
	if !ok {
		return nil, fmt.Errorf("zone %s not found in configuration", zone)
	}

	// Get all records for the zone with pagination
	var allRecords []cloudflare.DNSRecord
	page := 1
	for {
		rc := cloudflare.ZoneIdentifier(zoneID)
		params := cloudflare.ListDNSRecordsParams{
			ResultInfo: cloudflare.ResultInfo{
				Page:    page,
				PerPage: 100,
			},
		}

		records, resultInfo, err := p.client.ListDNSRecords(ctx, rc, params)
		if err != nil {
			p.metrics.IncDNSRequest("read", zone, false)
			return nil, fmt.Errorf("failed to list DNS records: %w", err)
		}

		allRecords = append(allRecords, records...)
		if page >= resultInfo.TotalPages {
			break
		}
		page++
	}

	// Convert to provider records
	var result []provider.Record
	for _, r := range allRecords {
		result = append(result, provider.Record{
			ID:   r.ID,
			Name: r.Name,
			Type: r.Type,
			Data: r.Content,
			TTL:  time.Duration(r.TTL) * time.Second,
			Zone: zone,
		})
	}

	p.metrics.IncDNSRequest("read", zone, true)
	slog.Debug("Retrieved DNS records", "zone", zone, "count", len(result), "duration", time.Since(start))
	return result, nil
}

func (p *CloudflareProvider) CreateRecord(ctx context.Context, zone string, record provider.Record) error {
	slog.Info("Creating DNS record", "zone", zone, "name", record.Name, "type", record.Type, "data", record.Data)
	start := time.Now()

	zoneID, ok := p.zones[zone]
	if !ok {
		return fmt.Errorf("zone %s not found in configuration", zone)
	}

	params := cloudflare.CreateDNSRecordParams{
		Type:    record.Type,
		Name:    record.Name,
		Content: record.Data,
		TTL:     int(record.TTL.Seconds()),
	}

	_, err := p.client.CreateDNSRecord(ctx, cloudflare.ZoneIdentifier(zoneID), params)
	if err != nil {
		p.metrics.IncDNSRequest("create", zone, false)
		return fmt.Errorf("failed to create DNS record: %w", err)
	}

	p.metrics.IncDNSRequest("create", zone, true)
	slog.Debug("Created DNS record", "zone", zone, "name", record.Name, "type", record.Type, "duration", time.Since(start))
	return nil
}

func (p *CloudflareProvider) UpdateRecord(ctx context.Context, zone string, record provider.Record) error {
	slog.Info("Updating DNS record", "zone", zone, "name", record.Name, "type", record.Type, "data", record.Data)
	start := time.Now()

	zoneID, ok := p.zones[zone]
	if !ok {
		return fmt.Errorf("zone %s not found in configuration", zone)
	}

	params := cloudflare.UpdateDNSRecordParams{
		ID:      record.ID,
		Type:    record.Type,
		Name:    record.Name,
		Content: record.Data,
		TTL:     int(record.TTL.Seconds()),
	}

	_, err := p.client.UpdateDNSRecord(ctx, cloudflare.ZoneIdentifier(zoneID), params)
	if err != nil {
		p.metrics.IncDNSRequest("update", zone, false)
		return fmt.Errorf("failed to update DNS record: %w", err)
	}

	p.metrics.IncDNSRequest("update", zone, true)
	slog.Debug("Updated DNS record", "zone", zone, "name", record.Name, "type", record.Type, "duration", time.Since(start))
	return nil
}

func (p *CloudflareProvider) DeleteRecord(ctx context.Context, zone string, record provider.Record) error {
	slog.Info("Deleting DNS record", "zone", zone, "name", record.Name, "type", record.Type)
	start := time.Now()

	zoneID, ok := p.zones[zone]
	if !ok {
		return fmt.Errorf("zone %s not found in configuration", zone)
	}

	err := p.client.DeleteDNSRecord(ctx, cloudflare.ZoneIdentifier(zoneID), record.ID)
	if err != nil {
		p.metrics.IncDNSRequest("delete", zone, false)
		return fmt.Errorf("failed to delete DNS record: %w", err)
	}

	p.metrics.IncDNSRequest("delete", zone, true)
	slog.Debug("Deleted DNS record", "zone", zone, "name", record.Name, "type", record.Type, "duration", time.Since(start))
	return nil
}
