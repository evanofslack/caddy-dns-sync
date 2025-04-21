package provider

import (
	"fmt"
	"net/netip"
	"time"

	"github.com/libdns/libdns"
)

type Provider interface {
	GetRecords(zone string) ([]Record, error)
	CreateRecord(zone string, record Record) error
	UpdateRecord(zone string, record Record) error
	DeleteRecord(zone string, record Record) error
}

type Record struct {
	Name string
	Type string
	Data string
	TTL  time.Duration
}

func FromLibdns(r libdns.Record) Record {
	rr := r.RR()
	record := Record{
		Name: rr.Name,
		Type: rr.Type,
		Data: rr.Data,
		TTL:  rr.TTL,
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
