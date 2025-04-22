package caddy

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/evanofslack/caddy-dns-sync/metrics"
	"github.com/evanofslack/caddy-dns-sync/source"
)

type Client interface {
	Domains(ctx context.Context) ([]source.DomainConfig, error)
}

type Httper interface {
	Do(req *http.Request) (*http.Response, error)
}

type client struct {
	adminURL string
	http     Httper
	metrics  *metrics.Metrics
}

func New(adminURL string, metrics *metrics.Metrics) Client {
	return &client{
		adminURL: adminURL,
		http:     &http.Client{},
		metrics:  metrics,
	}
}

func (c *client) Domains(ctx context.Context) ([]source.DomainConfig, error) {
	domains := []source.DomainConfig{}
	config, err := c.getConfiguration(ctx)
	if err != nil {
		return domains, err
	}
	domains, err = c.extractDomains(config)
	if err != nil {
		return domains, err
	}
	return domains, nil
}

func (c *client) getConfiguration(ctx context.Context) (Config, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.adminURL+"/config/", nil)
	if err != nil {
		c.metrics.IncCaddyRequest(false, 0)
		return Config{}, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.metrics.IncCaddyRequest(false, 0)
		return Config{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.metrics.IncCaddyRequest(false, resp.StatusCode)
		return Config{}, fmt.Errorf("caddy api request, status=%d", resp.StatusCode)
	}

	var config Config
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		c.metrics.IncCaddyRequest(false, 0)
		return Config{}, fmt.Errorf("parse caddy config, err=%w", err)
	}
	c.metrics.IncCaddyRequest(true, resp.StatusCode)
	return config, nil
}

func (c *client) extractDomains(config Config) ([]source.DomainConfig, error) {
	domains := []source.DomainConfig{}
	entries := 0
	for _, server := range config.Apps.HTTP.Servers {
		for _, route := range server.Routes {
			for _, match := range route.Match {
				for _, host := range match.Host {
					entries++
					c.processHandlers(host, route.Handle, &domains)
				}
			}
		}
	}

	// Count reverse proxies
	c.metrics.SetCaddyEntries(len(domains), true)
	// Count non reverse proxies
	norp := entries - len(domains)
	if norp > 0 {
		c.metrics.SetCaddyEntries(norp, false)
	}
	return domains, nil
}

func (c *client) processHandlers(parentHost string, handlers []Handler, domains *[]source.DomainConfig) {
	for _, handler := range handlers {
		slog.Default().Debug("Processing handler", "handler", handler.Handler, "upstreams", handler.Upstreams)

		// Track current host context through nested routes
		currentHost := parentHost
		if handler.Handler == "subroute" {
			for _, nestedRoute := range handler.Routes {
				// Update host context if route has host matches
				for _, match := range nestedRoute.Match {
					if len(match.Host) > 0 {
						currentHost = match.Host[0]
					}
				}
				c.processHandlers(currentHost, nestedRoute.Handle, domains)
			}
		}

		if handler.Handler == "reverse_proxy" && len(handler.Upstreams) > 0 {
			*domains = append(*domains, source.DomainConfig{
				Host:     currentHost, // Use most specific host context
				Upstream: handler.Upstreams[0].Dial,
			})
		}
	}
}
