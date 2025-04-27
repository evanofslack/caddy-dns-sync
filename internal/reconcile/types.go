package reconcile

import (
	"github.com/evanofslack/caddy-dns-sync/internal/provider"
)

type Plan struct {
	Create []provider.Record
	Update []provider.Record
	Delete []provider.Record
}

type Results struct {
	Created  []provider.Record
	Updated  []provider.Record
	Deleted  []provider.Record
	Failures []OperationResult
}

type OperationResult struct {
	Record provider.Record
	Op     string
	Error  string
}
