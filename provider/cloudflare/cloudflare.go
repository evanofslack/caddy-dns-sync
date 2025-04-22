package cloudflare

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/evanofslack/caddy-dns-sync/config"
	"github.com/evanofslack/caddy-dns-sync/metrics"
	"github.com/evanofslack/caddy-dns-sync/provider"
	"github.com/libdns/cloudflare"
	"github.com/libdns/libdns"
)

type CloudflareProvider struct {
	provider string
	ttl      int
	cf       *cloudflare.Provider
	metrics  *metrics.Metrics
}

func New(cfg config.DNS, metrics *metrics.Metrics) (*CloudflareProvider, error) {
	p := &CloudflareProvider{
		provider: cfg.Provider,
		ttl:      cfg.TTL,
		metrics:  metrics,
	}

	token := cfg.Token
	if token == "" {
		return nil, fmt.Errorf("cloudflare api token empty")
	}

	p.cf = &cloudflare.Provider{
		APIToken: token,
	}
	return p, nil
}

func (p *CloudflareProvider) GetRecords(ctx context.Context, zone string) ([]provider.Record, error) {
	slog.Info("Getting DNS records", "zone", zone)

	records, err := p.cf.GetRecords(ctx, zone)
	if err != nil {
		p.metrics.IncDNSRequest("read", zone, false)
		return nil, err
	}

	var result []provider.Record
	for _, r := range records {
		result = append(result, provider.FromLibdns(r))
	}
	p.metrics.IncDNSRequest("read", zone, true)
	return result, nil
}

func (p *CloudflareProvider) CreateRecord(ctx context.Context, zone string, record provider.Record) error {
	slog.Info("Creating DNS record", "zone", zone, "name", record.Name, "type", record.Type, "data", record.Data)

	r, err := provider.ToLibdns(record)
	if err != nil {
		p.metrics.IncDNSRequest("create", zone, false)
		return err
	}
	recs := []libdns.Record{r}

	if _, err = p.cf.AppendRecords(ctx, zone, recs); err != nil {
		p.metrics.IncDNSRequest("create", zone, false)
		return err

	}
	p.metrics.IncDNSRequest("create", zone, true)
	return nil
}

func (p *CloudflareProvider) UpdateRecord(ctx context.Context, zone string, record provider.Record) error {
	slog.Info("Updating DNS record", "zone", zone, "name", record.Name, "type", record.Type, "data", record.Data)

	r, err := provider.ToLibdns(record)
	if err != nil {
		p.metrics.IncDNSRequest("update", zone, false)
		return err
	}
	recs := []libdns.Record{r}

	if _, err := p.cf.SetRecords(ctx, zone, recs); err != nil {
		p.metrics.IncDNSRequest("update", zone, false)
		return err
	}

	p.metrics.IncDNSRequest("update", zone, true)
	return nil
}

func (p *CloudflareProvider) DeleteRecord(ctx context.Context, zone string, record provider.Record) error {
	slog.Info("Deleting DNS record", "zone", zone, "name", record.Name)

	r, err := provider.ToLibdns(record)
	if err != nil {
		p.metrics.IncDNSRequest("delete", zone, false)
		return err
	}
	recs := []libdns.Record{r}

	if _, err := p.cf.DeleteRecords(ctx, zone, recs); err != nil {
		p.metrics.IncDNSRequest("delete", zone, false)
		return err
	}

	p.metrics.IncDNSRequest("delete", zone, true)
	return nil
}
