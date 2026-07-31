package status

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/MorganKryze/cairn/src/internal/config"
)

// Emit derives a Gatus endpoints config from the services. Endpoint
// names are service ids: that is also what the status poller matches on.
func Emit(cfg *config.Config) ([]byte, error) {
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

// State is what Gatus says about one endpoint: whether its last check passed,
// and the key it uses for its own endpoint page. The key is reported rather
// than derived because only the operator's Gatus config decides how endpoints
// are grouped, and cairn's categories are not that config.
type State struct {
	Up  bool
	Key string
}

// Key builds the key Gatus uses in its endpoint page URLs
// (/endpoints/{group}_{name}), mirroring its own sanitization. It is the
// fallback for a Gatus too old to report one, and for the pills that show
// before Gatus has answered at all.
func Key(group, name string) string {
	sanitize := func(s string) string {
		s = strings.ToLower(s)
		return strings.NewReplacer("/", "-", "_", "-", ",", "-", ".", "-", "#", "-").Replace(s)
	}
	return sanitize(group) + "_" + sanitize(name)
}

// Unmonitored names the services Gatus knows nothing about, so an id
// mismatch is one log line instead of a silent missing pill.
func Unmonitored(cfg *config.Config, statuses map[string]State) string {
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

// verifying and skipping are built once and reused. A client per poll would
// build a fresh connection pool every interval and leave the old one to be
// collected with its idle sockets still open.
//
// The second one exists because an internal Gatus usually answers with a
// certificate no public authority signed, and mounting a CA bundle is not
// always the operator's to arrange. It is a real hole and it is theirs to open:
// with verification off, anything that can answer on that address decides what
// the pills say, and the day the certificate changes stops being visible.
// cairn says so at startup and on every -check rather than only here.
var (
	verifying = &http.Client{Timeout: 5 * time.Second}
	skipping  = &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			// #nosec G402 -- the operator asked for this with status.insecure,
			// it is logged at startup and warned about by -check, and it covers
			// the one outbound request cairn makes.
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		},
	}
)

// Fetch asks a Gatus instance for its endpoint statuses and returns
// service-id -> up, keyed by endpoint name. insecure skips certificate
// verification, which is status.insecure in site.yaml.
func Fetch(base string, insecure bool) (map[string]State, error) {
	client := verifying
	if insecure {
		client = skipping
	}
	resp, err := client.Get(strings.TrimSuffix(base, "/") + "/api/v1/endpoints/statuses")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gatus answered %s", resp.Status)
	}
	// Bound the body: the timeout alone would not stop a fast endless stream
	// from eating the process. A few thousand endpoints fit well under this.
	body := io.LimitReader(resp.Body, 8<<20)
	var list []struct {
		Name    string `json:"name"`
		Key     string `json:"key"`
		Results []struct {
			Success bool `json:"success"`
		} `json:"results"`
	}
	if err := json.NewDecoder(body).Decode(&list); err != nil {
		return nil, err
	}
	out := make(map[string]State, len(list))
	for _, e := range list {
		if len(e.Results) > 0 {
			out[e.Name] = State{Up: e.Results[len(e.Results)-1].Success, Key: e.Key}
		}
	}
	return out, nil
}
