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
// dashboard. All three keys are needed: they hide the URL, the hostname and the
// port separately. hide-url also redacts the address inside the error text,
// which only appears when a check fails.
type hidden struct {
	URL      bool `yaml:"hide-url"`
	Hostname bool `yaml:"hide-hostname"`
	Port     bool `yaml:"hide-port"`
}

// Emit derives a Gatus endpoints config from the services. Endpoint names are
// service ids, which is what the status poller matches on.
//
// hide adds the ui block above to every endpoint. It is off by default: what
// this writes is the service's own public URL, already printed on its card. It
// earns its keep once the emitted file is edited to probe an address that is
// not the published one.
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
			if !monitorable(s) {
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

// An endpoint for a service that is not there yet or not there any more is a
// red dashboard row its operator can never fix.
func monitorable(s config.Service) bool {
	return strings.HasPrefix(s.URL, "http") && !s.State.Disables()
}

// Level is what a source says about one service. Up and down are universal,
// degraded and maintenance are each drawn by four monitors of the eight
// surveyed. Anything else a source says reads as down, the safe direction: a
// word added to a vendor's API next year cannot make a broken service look
// green.
//
// Gatus reaches only two of them. Its results carry a success bool and nothing
// else, so giving the other sources more states cannot move what a Gatus site
// renders.
const (
	LevelUp          = "up"
	LevelDegraded    = "degraded"
	LevelMaintenance = "maintenance"
	LevelDown        = "down"
)

// State is what a source says about one service: the level, and the key of the
// source's own page for it, empty when it has none. The key is reported rather
// than derived: only the operator's Gatus config decides how endpoints are
// grouped, and cairn's categories are not that config.
type State struct {
	Level string
	Key   string
}

// Key builds the key Gatus uses in its endpoint page URLs
// (/endpoints/{group}_{name}), mirroring its own sanitization. It is the
// fallback for a Gatus too old to report one, and for the pills drawn before
// Gatus has answered at all.
func Key(group, name string) string {
	sanitize := func(s string) string {
		s = strings.ToLower(s)
		return strings.NewReplacer("/", "-", "_", "-", ",", "-", ".", "-", "#", "-").Replace(s)
	}
	return sanitize(group) + "_" + sanitize(name)
}

// Unmonitored names the services the monitor knows nothing about, so an id
// mismatch is one log line instead of a silent missing pill.
func Unmonitored(cfg *config.Config, statuses map[string]State) string {
	var ids []string
	claimed := make(map[string]bool, len(statuses))
	for _, c := range cfg.Categories {
		for _, s := range c.Services {
			if _, ok := statuses[s.ID]; ok {
				claimed[s.ID] = true
			} else {
				ids = append(ids, s.ID)
			}
		}
	}
	if len(ids) == 0 {
		return ""
	}
	subject, cards := "services", "their cards show"
	if len(ids) == 1 {
		subject, cards = "service", "its card shows"
	}
	line := fmt.Sprintf("%d %s the %s at %s says nothing about, %s no pill: %s",
		len(ids), subject, cfg.Site.StatusProvider(), cfg.Site.StatusAddress(), cards, strings.Join(ids, ", "))
	// Listing what is missing tells an operator nothing they did not already
	// see on the page. Naming what the monitor offered and no service claimed
	// puts the two side by side, so a mismatch case-folding cannot reach, a
	// domain name or a name with a space in it, is obvious at a glance.
	var unclaimed []string
	for k := range statuses {
		if !claimed[k] {
			unclaimed = append(unclaimed, k)
		}
	}
	if len(unclaimed) == 0 {
		return line
	}
	slices.Sort(unclaimed)
	names := "names"
	if len(unclaimed) == 1 {
		names = "name"
	}
	return fmt.Sprintf("%s. It reports %d %s no service id matches: %s",
		line, len(unclaimed), names, strings.Join(unclaimed, ", "))
}

// verifying and skipping are built once and reused. A client per poll would
// build a fresh connection pool every interval and leave the old one to be
// collected with its idle sockets still open.
//
// skipping exists for an internal Gatus answering with a certificate no public
// authority signed, where mounting a CA bundle is not always the operator's to
// arrange. It is a real hole and theirs to open: with verification off,
// anything that can answer on that address decides what the pills say, and the
// day the certificate changes stops being visible. cairn says so at startup and
// on every -check rather than only here.
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

// Source is where the pills come from: the monitor cairn polls, and how much it
// is willing to trust on the way there. It mirrors the status block in
// site.yaml, so the trust settings travel with the address they apply to.
type Source struct {
	URL      string
	Provider string // status.provider: empty means gatus, which is what every config written before providers existed says
	Slug     string // status.slug: kuma only, the published status page to read
	Insecure bool   // status.insecure: verify nothing
	CA       string // status.ca: verify against this too, a URL or an /assets path
	// TokenFile names a file holding a bearer token, never the token itself: a
	// secret written in site.yaml is a secret in a config repository. Every
	// platform that has secrets delivers them as a mounted file.
	TokenFile   string
	TokenScheme string  // default Bearer; Statuspage wants OAuth
	Map         Mapping // json only: how to read somebody else's document
}

// trusted caches the client built from a bundle: building one means reading or
// fetching that bundle, and a poll happens every interval.
//
// Only a success is cached. A bundle served from an address that is not up yet
// is tried again on the next poll, so a cairn started before what it depends on
// still recovers.
var trusted struct {
	sync.Mutex
	ref    string
	client *http.Client
}

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
// authority did sign keeps working, and adding a bundle narrows nothing else
// cairn would accept.
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

// maxBundle: a CA bundle is kilobytes, the whole public root store a few
// hundred. Unbounded, an endless stream would eat the process.
const maxBundle = 1 << 20

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
	return io.ReadAll(io.LimitReader(resp.Body, maxBundle))
}

// fetcher is one way of asking a monitor what it knows. Fetch chooses the
// client once, so every provider inherits the same timeout and the same answer
// to status.insecure and status.ca.
type fetcher func(*http.Client, Source) (map[string]State, error)

// maxBody bounds the document a provider reads: the timeout alone would not
// stop a fast endless stream from eating the process. A few thousand endpoints
// fit well under it.
const maxBody = 8 << 20

// The error in Fetch reads this map, so a provider registered here is one that
// message already names.
var providers = map[string]fetcher{"gatus": fetchGatus, "json": fetchJSON, "kuma": fetchKuma}

func providerNames() string {
	return strings.Join(slices.Sorted(maps.Keys(providers)), ", ")
}

// Fetch asks the source's monitor what it knows, keyed by service id.
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
	got, err := f(client, src)
	if err != nil {
		return nil, err
	}
	return folded(got), nil
}

// folded lowercases what a monitor reported. A cairn service id can only be
// lowercase, idRe sees to that, but a monitor name is typed into a form by a
// person who writes "Immich": issue #33 was a setup otherwise entirely correct,
// published page, right slug, monitor green, and no pill on the card.
//
// Folding can add a match and cannot invent a wrong one, the other side being
// lowercase by construction. It happens here rather than in each provider so a
// provider added later cannot forget it. Two monitor names differing only in
// case collapse into one, which is a directory nobody could read anyway.
func folded(in map[string]State) map[string]State {
	out := make(map[string]State, len(in))
	for k, v := range in {
		out[strings.ToLower(k)] = v
	}
	return out
}

// fetchGatus keys the statuses by endpoint name, which is the service id.
func fetchGatus(client *http.Client, src Source) (map[string]State, error) {
	resp, err := get(client, src, strings.TrimSuffix(src.URL, "/")+"/api/v1/endpoints/statuses")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gatus answered %s", resp.Status)
	}
	body := io.LimitReader(resp.Body, maxBody)
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
