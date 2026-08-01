package status

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Uptime Kuma, read through the two endpoints a published status page answers
// without a login. There is no config file to write and no setup API to call:
// standing an instance up for the audit took a browser driven by a script, so
// there is no -emit-kuma and there cannot be one. What an operator does is
// name each monitor after the cairn service id, by hand.
//
// Kuma's status constants, from its own source: 0 down, 1 up, 2 pending,
// 3 maintenance.
const (
	kumaDown        = 0
	kumaUp          = 1
	kumaPending     = 2
	kumaMaintenance = 3
)

// fetchKuma joins two documents on a numeric monitor id: the names live on the
// status page, the states in the heartbeat feed, and neither carries the
// other. That join is why this is code rather than a mapping.
func fetchKuma(client *http.Client, src Source) (map[string]State, error) {
	if src.Slug == "" {
		return nil, fmt.Errorf("status.slug is empty: kuma serves its statuses per published status page, so cairn needs the slug of yours (the last part of its URL, as in https://kuma.example.org/status/tools)")
	}
	base := strings.TrimSuffix(src.URL, "/") + "/api/status-page/"
	slug := url.PathEscape(src.Slug)

	var page struct {
		PublicGroupList []struct {
			MonitorList []struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"monitorList"`
		} `json:"publicGroupList"`
	}
	if err := kumaGet(client, src, base+slug, &page); err != nil {
		return nil, kumaErr(src, err)
	}
	var beats struct {
		HeartbeatList map[string][]struct {
			Status int `json:"status"`
		} `json:"heartbeatList"`
	}
	if err := kumaGet(client, src, base+"heartbeat/"+slug, &beats); err != nil {
		return nil, kumaErr(src, err)
	}

	out := make(map[string]State)
	monitors := 0
	for _, g := range page.PublicGroupList {
		for _, m := range g.MonitorList {
			monitors++
			// The last beat is the current one, the way the last Gatus result
			// is. A monitor with none is left out rather than called down:
			// nobody has looked at it yet, which is what the unknown pill says.
			hb := beats.HeartbeatList[strconv.Itoa(m.ID)]
			if len(hb) == 0 {
				continue
			}
			switch hb[len(hb)-1].Status {
			case kumaUp:
				out[m.Name] = State{Level: LevelUp}
			case kumaMaintenance:
				out[m.Name] = State{Level: LevelMaintenance}
			case kumaPending:
				// Pending is a monitor waiting for its first verdict, which is
				// unknown by another name. Kuma declares no degraded, so
				// nothing here may invent one either.
			default:
				out[m.Name] = State{Level: LevelDown}
			}
		}
	}
	if monitors == 0 {
		return nil, fmt.Errorf("status.slug %q: that status page has no monitor attached, so there is nothing to draw (add them to a group on the status page, and name each one after the cairn service id)", src.Slug)
	}
	return out, nil
}

// kumaErr says which key to look at. A 404 is the failure an operator will
// actually hit: a status page exists in the admin but was never published, and
// the endpoint answers exactly as it would for a misspelled slug.
func kumaErr(src Source, err error) error {
	if strings.Contains(err.Error(), "404") {
		return fmt.Errorf("status.slug %q: kuma answered %w, so no status page is published there (create it, tick Published, and use the slug from its URL)", src.Slug, err)
	}
	return fmt.Errorf("status.url %s: kuma answered something cairn could not read: %w", src.URL, err)
}

func kumaGet(client *http.Client, src Source, addr string, v any) error {
	resp, err := get(client, src, addr)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s", resp.Status)
	}
	// Bounded like the Gatus body: a timeout alone would not stop a fast
	// endless stream from eating the process.
	return json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(v)
}
