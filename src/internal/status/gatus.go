package status

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/MorganKryze/cairn/src/internal/config"
)

// hidden is the Gatus ui block that keeps an endpoint's address off its own
// dashboard. All three keys, because they hide three different things: the
// URL, the hostname, and the port, which outlives the other two. hide-url also
// redacts the address inside the error text, which is the leak that gets
// forgotten because it only appears when a check fails.
type hidden struct {
	URL      bool `yaml:"hide-url"`
	Hostname bool `yaml:"hide-hostname"`
	Port     bool `yaml:"hide-port"`
}

// Emit derives a Gatus endpoints config from the services. Endpoint
// names are service ids: that is also what the status poller matches on.
//
// hide adds the ui block above to every endpoint. It is off by default because
// what this writes is the service's own public URL, already printed on its
// card: hiding that would hide from the status page what the directory shows
// next to it. It earns its keep once the emitted file is edited to probe an
// address that is not the published one.
func Emit(cfg *config.Config, hide bool) ([]byte, error) {
	type endpoint struct {
		Name       string   `yaml:"name"`
		Group      string   `yaml:"group,omitempty"`
		URL        string   `yaml:"url"`
		Interval   string   `yaml:"interval"`
		Conditions []string `yaml:"conditions"`
		UI         *hidden  `yaml:"ui,omitempty"`
	}
	var ui *hidden
	if hide {
		ui = &hidden{URL: true, Hostname: true, Port: true}
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
				UI:         ui,
			})
		}
	}
	return yaml.Marshal(map[string]any{"endpoints": eps})
}

// Level is what a source says about one service. Four values, counted from
// what the sources actually declare rather than chosen: up and down are
// universal, degraded and maintenance are each drawn by four monitors of the
// eight surveyed. Anything else a source says reads as down, which is the safe
// direction: a word added to a vendor's API next year cannot make a broken
// service look green.
//
// Gatus reaches only two of them. Its results carry a success bool and nothing
// else, which is why giving the other sources more states cannot move what a
// Gatus site renders.
const (
	LevelUp          = "up"
	LevelDegraded    = "degraded"
	LevelMaintenance = "maintenance"
	LevelDown        = "down"
)

// State is what a source says about one service: the level, and the key of the
// source's own page for it, empty when it has none. The key is reported rather
// than derived because only the operator's Gatus config decides how endpoints
// are grouped, and cairn's categories are not that config.
type State struct {
	Level string
	Key   string
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

// Source is where the pills come from: the monitor cairn polls, and how much
// it is willing to trust on the way there. It mirrors the status block in
// site.yaml, and exists so the trust settings travel together with the address
// they apply to rather than as loose parameters.
type Source struct {
	URL      string
	Provider string // status.provider: empty means gatus, which is what every config written before providers existed says
	Slug     string // status.slug: kuma only, the published status page to read
	Insecure bool   // status.insecure: verify nothing
	CA       string // status.ca: verify against this too, a URL or an /assets path
	// TokenFile names a file holding a bearer token, never the token itself:
	// a secret written in site.yaml is a secret in a config repository. Every
	// platform that has secrets delivers them as a mounted file.
	TokenFile   string
	TokenScheme string  // default Bearer; Statuspage wants OAuth
	Map         Mapping // json only: how to read somebody else's document
}

// trusted caches the client built from a bundle, because building one means
// reading or fetching that bundle and a poll happens every interval.
//
// Only a success is cached. A bundle served from an address that is not up yet
// has to be tried again on the next poll: caching the failure would recreate,
// one layer down, the bug where cairn started before the thing it depends on
// and never recovered.
var trusted struct {
	sync.Mutex
	ref    string
	client *http.Client
}

// clientFor picks the client for a source: the shared verifying one, the shared
// skipping one, or one carrying the operator's own authority.
func clientFor(src Source) (*http.Client, error) {
	if src.Insecure {
		return skipping, nil
	}
	if src.CA == "" {
		return verifying, nil
	}
	trusted.Lock()
	defer trusted.Unlock()
	if trusted.ref == src.CA && trusted.client != nil {
		return trusted.client, nil
	}
	c, err := trusting(src.CA)
	if err != nil {
		return nil, err
	}
	trusted.ref, trusted.client = src.CA, c
	return c, nil
}

// trusting builds a client that verifies against the system roots plus the
// bundle at ref. Plus, not instead: a Gatus whose certificate a public
// authority did sign keeps working, and an operator who adds a bundle has not
// silently narrowed what else cairn would accept.
func trusting(ref string) (*http.Client, error) {
	body, err := bundle(ref)
	if err != nil {
		return nil, fmt.Errorf("status.ca %s: %w", ref, err)
	}
	pool, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("status.ca %s: the system roots could not be read to add it to: %w", ref, err)
	}
	if !pool.AppendCertsFromPEM(body) {
		return nil, fmt.Errorf("status.ca %s holds no certificate cairn could read (expected PEM, one or more -----BEGIN CERTIFICATE----- blocks; a DER file has to be converted with openssl x509 -inform der -out ca.crt)", ref)
	}
	return &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12},
		},
	}, nil
}

// bundle reads the PEM, from the mounted assets dir or over the network.
func bundle(ref string) ([]byte, error) {
	if p := config.AssetFile(ref); p != "" {
		return os.ReadFile(p)
	}
	resp, err := verifying.Get(ref)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("answered %s", resp.Status)
	}
	// A CA bundle is kilobytes; the whole public root store is a few hundred.
	// The cap is what stops an endless stream from eating the process, the
	// same reason the statuses body has one.
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

// fetcher is one way of asking a monitor what it knows. The client is chosen
// once, by Fetch, so every provider inherits the same timeout and the same
// answer to status.insecure and status.ca.
type fetcher func(*http.Client, Source) (map[string]State, error)

// providers is the whole list, and the error below reads it, so a provider
// registered here is one the message already names without being told.
var providers = map[string]fetcher{"gatus": fetchGatus, "json": fetchJSON, "kuma": fetchKuma}

func providerNames() string {
	return strings.Join(slices.Sorted(maps.Keys(providers)), ", ")
}

// Fetch asks the source's monitor what it knows and returns service-id ->
// state.
func Fetch(src Source) (map[string]State, error) {
	name := src.Provider
	if name == "" {
		name = "gatus"
	}
	f, ok := providers[name]
	if !ok {
		return nil, fmt.Errorf("status.provider %q is not one cairn reads (expected one of %s)", name, providerNames())
	}
	client, err := clientFor(src)
	if err != nil {
		return nil, err
	}
	return f(client, src)
}

// fetchGatus reads the endpoint statuses of a Gatus instance, keyed by
// endpoint name, which is the service id.
func fetchGatus(client *http.Client, src Source) (map[string]State, error) {
	resp, err := get(client, src, strings.TrimSuffix(src.URL, "/")+"/api/v1/endpoints/statuses")
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
			level := LevelDown
			if e.Results[len(e.Results)-1].Success {
				level = LevelUp
			}
			out[e.Name] = State{Level: level, Key: e.Key}
		}
	}
	return out, nil
}
