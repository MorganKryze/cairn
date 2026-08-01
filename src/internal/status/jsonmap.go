package status

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sort"
	"strings"
)

// Mapping reads a flat array of objects out of any JSON document. Six keys and
// no expression language: a path is a dotted walk and nothing else. Anything
// cleverer would be a query language, and a query language in YAML is a second
// product to document, secure and support.
//
// That is enough for every status API the audit found bar two: Atlassian
// Statuspage, Instatus, Upptime, StatusCake, Better Stack and whatever is
// written next all answer with a list of objects carrying a name and a state.
type Mapping struct {
	List        string   // path to the array, empty when the document is the array
	Key         string   // per element: the field holding the service name
	State       string   // per element: the field holding the state
	Up          []string // values that mean up
	Degraded    []string // values that mean up but not well
	Maintenance []string // values that mean off on purpose
	// Unknown are the values that mean nobody is checking: a monitor paused,
	// or one waiting for its first verdict. Those services get no pill at all
	// rather than a red one, which is what cairn's neutral state has always
	// meant and what the kuma provider does with its own pending.
	Unknown []string
}

// level reads one state through the three allow-lists, checked maintenance,
// degraded, up, then down. A value in none of them reads as down, which is the
// safe direction twice over: a vendor that adds a word next year cannot make a
// broken service look green, and an operator who forgot to list one sees a red
// pill rather than a wrong one.
func (m Mapping) level(state string) string {
	switch {
	case slices.Contains(m.Maintenance, state):
		return LevelMaintenance
	case slices.Contains(m.Degraded, state):
		return LevelDegraded
	case slices.Contains(m.Up, state):
		return LevelUp
	default:
		return LevelDown
	}
}

// fetchJSON reads any status API shaped like a list of {name, state}.
func fetchJSON(client *http.Client, src Source) (map[string]State, error) {
	resp, err := get(client, src, src.URL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if err := answered(resp, src); err != nil {
		return nil, err
	}
	var doc any
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&doc); err != nil {
		return nil, fmt.Errorf("status.url %s answered something that is not JSON: %w", src.URL, err)
	}

	m := src.Map
	found := walk(doc, m.List)
	rows, ok := found.([]any)
	if !ok {
		return nil, fmt.Errorf("status.map.list %q found %s in the document at %s (expected an array of objects; the document has %s)",
			m.List, describe(found), src.URL, keysOf(doc))
	}

	out := make(map[string]State, len(rows))
	named, stated := 0, 0
	for _, row := range rows {
		// A row the mapping cannot read is skipped rather than fatal. Real
		// status pages carry group headers and marketing rows, and cairn only
		// draws a pill for a name matching a service id, so junk is already
		// harmless: one bad row must not cost the whole poll.
		name, ok := walk(row, m.Key).(string)
		if !ok || name == "" {
			continue
		}
		named++
		// A row with a name and no state is left out rather than called down,
		// which is what Gatus does with an endpoint that has no result and
		// Kuma with a monitor that has no heartbeat. Nothing has been said
		// about it, and the unknown pill is what says that.
		state, ok := walk(row, m.State).(string)
		if !ok {
			continue
		}
		stated++
		// Checked before the levels, so a value listed twice by mistake gets
		// the quiet answer rather than whichever list is read first.
		if slices.Contains(m.Unknown, state) {
			continue
		}
		out[name] = State{Level: m.level(state)}
	}
	// Nothing read at all is a mapping that is wrong rather than a status page
	// that is empty, and the two possible typos get separate messages: the
	// operator is reading one log line and has to know which key to open.
	//
	// The counters are what was found, not what was drawn: a page whose every
	// monitor is paused reads fine and simply produces no pill, so it must not
	// look like a mapping that missed.
	switch {
	case len(rows) == 0:
	case named == 0:
		return nil, fmt.Errorf("status.map.key %q found nothing in any of the %d rows at %s (a row there has %s)",
			m.Key, len(rows), src.URL, keysOf(rows[0]))
	case stated == 0:
		return nil, fmt.Errorf("status.map.state %q found nothing in any of the %d rows at %s (a row there has %s)",
			m.State, len(rows), src.URL, keysOf(rows[0]))
	}
	return out, nil
}

// walk descends a dotted path. An empty path is the document itself, which is
// what a bare array answers with.
func walk(v any, path string) any {
	if path == "" {
		return v
	}
	for _, step := range strings.Split(path, ".") {
		obj, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = obj[step]
	}
	return v
}

// keysOf and describe exist so a mapping that read nothing says what was
// actually there. -check makes no network request, deliberately, so a typo in
// a mapping first shows up as one line in a log: that line has to be enough.
func keysOf(v any) string {
	obj, ok := v.(map[string]any)
	if !ok {
		return describe(v)
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) > 12 {
		keys = keys[:12]
	}
	return "the keys " + strings.Join(keys, ", ")
}

func describe(v any) string {
	switch t := v.(type) {
	case nil:
		return "nothing"
	case []any:
		return fmt.Sprintf("an array of %d", len(t))
	case map[string]any:
		return "an object"
	case string:
		return "a string"
	default:
		return fmt.Sprintf("%T", v)
	}
}
