package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// emitGatus derives a Gatus endpoints config from the services. Endpoint
// names are service ids: that is also what the status poller matches on.
func emitGatus(cfg *Config) ([]byte, error) {
	type endpoint struct {
		Name       string   `yaml:"name"`
		Group      string   `yaml:"group,omitempty"`
		URL        string   `yaml:"url"`
		Interval   string   `yaml:"interval"`
		Conditions []string `yaml:"conditions"`
	}
	var eps []endpoint
	for _, c := range cfg.Categories {
		for _, s := range c.Services {
			if !strings.HasPrefix(s.URL, "http") {
				continue
			}
			eps = append(eps, endpoint{
				Name:       s.ID,
				Group:      s.Category,
				URL:        s.URL,
				Interval:   "5m",
				Conditions: []string{"[STATUS] == 200"},
			})
		}
	}
	return yaml.Marshal(map[string]any{"endpoints": eps})
}

// gatusKey builds the key Gatus uses in its endpoint page URLs
// (/endpoints/{group}_{name}), mirroring its own sanitization.
func gatusKey(group, name string) string {
	sanitize := func(s string) string {
		s = strings.ToLower(s)
		return strings.NewReplacer("/", "-", "_", "-", ",", "-", ".", "-", "#", "-").Replace(s)
	}
	return sanitize(group) + "_" + sanitize(name)
}

// unmonitored names the services Gatus knows nothing about, so an id
// mismatch is one log line instead of a silent missing pill.
func unmonitored(cfg *Config, statuses map[string]bool) string {
	var ids []string
	for _, c := range cfg.Categories {
		for _, s := range c.Services {
			if _, ok := statuses[s.ID]; !ok {
				ids = append(ids, s.ID)
			}
		}
	}
	if len(ids) == 0 {
		return ""
	}
	return fmt.Sprintf("%d services have no gatus endpoint, their cards show no pill: %s", len(ids), strings.Join(ids, ", "))
}

// fetchStatuses asks a Gatus instance for its endpoint statuses and returns
// service-id -> up, keyed by endpoint name.
func fetchStatuses(base string) (map[string]bool, error) {
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(strings.TrimSuffix(base, "/") + "/api/v1/endpoints/statuses")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gatus answered %s", resp.Status)
	}
	var list []struct {
		Name    string `json:"name"`
		Results []struct {
			Success bool `json:"success"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(list))
	for _, e := range list {
		if len(e.Results) > 0 {
			out[e.Name] = e.Results[len(e.Results)-1].Success
		}
	}
	return out, nil
}
