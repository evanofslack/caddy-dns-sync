package provider

import (
	"context"
	"time"
)

type Provider interface {
	GetRecords(ctx context.Context, zone string) ([]Record, error)
	CreateRecord(ctx context.Context, zone string, record Record) error
	UpdateRecord(ctx context.Context, zone string, record Record) error
	DeleteRecord(ctx context.Context, zone string, record Record) error
}

type Record struct {
	ID   string
	Name string
	Type string
	Data string
	Zone string
	TTL  time.Duration
}

