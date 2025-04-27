package state

import (
	"github.com/evanofslack/caddy-dns-sync/internal/source"
)

type State struct {
	Domains map[string]DomainState
}

type DomainState struct {
	ServerName string `json:"serverName"`
	LastSeen   int64  `json:"lastSeen"`
}

type StateChanges struct {
	Added   []source.DomainConfig
	Removed []string
}

func (st StateChanges) IsEmpty() bool {
	return len(st.Added) == 0 && len(st.Removed) == 0
}
