package cloudflare

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/evanofslack/caddy-dns-sync/config"
	"github.com/evanofslack/caddy-dns-sync/provider"
	"github.com/libdns/cloudflare"
	"github.com/libdns/libdns"
)

type CloudflareProvider struct {
	provider string
	ttl      int
	cf       *cloudflare.Provider
}

func New(cfg config.DNS) (*CloudflareProvider, error) {
	p := &CloudflareProvider{
		provider: cfg.Provider,
		ttl:      cfg.TTL,
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

func (p *CloudflareProvider) GetRecords(zone string) ([]provider.Record, error) {
	ctx := context.Background()
	slog.Info("Getting DNS records", "zone", zone)

	records, err := p.cf.GetRecords(ctx, zone)
	if err != nil {
		return nil, err
	}

	var result []provider.Record
	for _, r := range records {
		result = append(result, provider.FromLibdns(r))
	}
	return result, nil
}

func (p *CloudflareProvider) CreateRecord(zone string, record provider.Record) error {
	ctx := context.Background()
	slog.Info("Creating DNS record", "zone", zone, "name", record.Name, "type", record.Type, "data", record.Data)

	r, err := provider.ToLibdns(record)
	if err != nil {
		return err
	}
	recs := []libdns.Record{r}

	switch p.provider {
	case "cloudflare":
		_, err = p.cf.AppendRecords(ctx, zone, recs)
		return err
	default:
		return fmt.Errorf("unsupported provider: %s", p.provider)
	}
}

func (p *CloudflareProvider) UpdateRecord(zone string, record provider.Record) error {
	ctx := context.Background()
	slog.Info("Updating DNS record", "zone", zone, "name", record.Name, "type", record.Type, "data", record.Data)

	r, err := provider.ToLibdns(record)
	if err != nil {
		return err
	}
	recs := []libdns.Record{r}

	switch p.provider {
	case "cloudflare":
		_, err := p.cf.SetRecords(ctx, zone, recs)
		return err
	default:
		return fmt.Errorf("unsupported provider: %s", p.provider)
	}
}

func (p *CloudflareProvider) DeleteRecord(zone string, record provider.Record) error {
	ctx := context.Background()
	slog.Info("Deleting DNS record", "zone", zone, "name", record.Name)

	r, err := provider.ToLibdns(record)
	if err != nil {
		return err
	}
	recs := []libdns.Record{r}

	switch p.provider {
	case "cloudflare":
		_, err := p.cf.DeleteRecords(ctx, zone, recs)
		return err
	default:
		return fmt.Errorf("unsupported provider: %s", p.provider)
	}
}
