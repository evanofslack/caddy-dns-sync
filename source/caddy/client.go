package caddy

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/evanofslack/caddy-dns-sync/source"
)

type Client interface {
	Domains() ([]source.DomainConfig, error)
}

type Httper interface {
	Get(url string) (*http.Response, error)
}

type client struct {
	adminURL string
	http     Httper
}

func New(adminURL string) Client {
	return &client{
		adminURL: adminURL,
		http:     &http.Client{},
	}
}

func (c *client) Domains() ([]source.DomainConfig, error) {
	domains := []source.DomainConfig{}
	config, err := c.getConfiguration()
	if err != nil {
		return domains, err
	}
	domains, err = c.extractDomains(config)
	if err != nil {
		return domains, err
	}
	return domains, nil
}

func (c *client) getConfiguration() (Config, error) {
	resp, err := c.http.Get(c.adminURL + "/config/")
	if err != nil {
		return Config{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Config{}, fmt.Errorf("caddy api request, status=%d", resp.StatusCode)
	}

	var config Config
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return Config{}, fmt.Errorf("parse caddy config, err=%w", err)
	}
	return config, nil
}

func (c *client) extractDomains(config Config) ([]source.DomainConfig, error) {
	domains := []source.DomainConfig{}

	for _, server := range config.Apps.HTTP.Servers {
		for _, route := range server.Routes {
			for _, match := range route.Match {
				for _, host := range match.Host {
					// Find reverse_proxy handlers
					for _, handle := range route.Handle {
					    slog.Default().Info("Got handler", "handler", handle.Handler, "upstreams", handle.Upstreams)
						if handle.Handler == "reverse_proxy" && len(handle.Upstreams) > 0 {
							domains = append(domains, source.DomainConfig{
								Host:       host,
								Upstream: handle.Upstreams[0].Dial,
							})
						}
					}
				}
			}
		}
	}

	return domains, nil
}
