package provider

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	"github.com/libdns/libdns"
)

type Provider interface {
	GetRecords(ctx context.Context, zone string) ([]Record, error)
	CreateRecord(ctx context.Context, zone string, record Record) error
	UpdateRecord(ctx context.Context, zone string, record Record) error
	DeleteRecord(ctx context.Context, zone string, record Record) error
}

type Record struct {
	Name string
	Type string
	Data string
	Zone string
	TTL  time.Duration
}

func FromLibdns(r libdns.Record, zone string) Record {
	rr := r.RR()
	record := Record{
		Name: rr.Name,
		Type: rr.Type,
		Data: rr.Data,
		TTL:  rr.TTL,
		Zone: zone,
	}
	return record
}

func ToLibdns(r Record) (libdns.Record, error) {
	switch r.Type {
	case "A", "AAAA":
		addr, err := netip.ParseAddr(r.Data)
		if err != nil {
			return nil, fmt.Errorf("fail parse ip addr %s, err=%w", r.Data, err)
		}
		out := &libdns.Address{
			Name: r.Name,
			IP:   addr,
			TTL:  r.TTL,
		}
		return out, nil
	case "CNAME":
		out := &libdns.CNAME{
			Name:   r.Name,
			Target: r.Data,
			TTL:    r.TTL,
		}
		return out, nil
	case "TXT":
		out := &libdns.TXT{
			Name: r.Name,
			Text: r.Data,
			TTL:  r.TTL,
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unknown record type %s", r.Type)
	}
}
