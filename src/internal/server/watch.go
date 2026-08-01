package server

import (
	"fmt"
	"io/fs"
	"log"
	"maps"
	"path/filepath"
	"strings"
	"time"

	"github.com/MorganKryze/cairn/src/internal/config"
	"github.com/MorganKryze/cairn/src/internal/render"
	"github.com/MorganKryze/cairn/src/internal/status"
)

// watch polls instead of using inotify: polling survives bind mounts and
// configmap symlink swaps that file watchers routinely miss. The loop is only
// the clock; reloadOnce does the work and is what the tests drive.
func Watch(dir string) {
	last := watchPrint(dir)
	for range time.Tick(2 * time.Second) {
		last = reloadOnce(dir, last)
	}
}

// watchPrint fingerprints everything a page is built from: the config dir, and
// the local icons that live outside it but change the pages too.
func watchPrint(dir string) string {
	return fingerprint(dir) + fingerprint(filepath.Join(config.AssetsPath, "icons"))
}

// reloadOnce runs one iteration of the watcher and returns the fingerprint to
// compare against next time. An unchanged directory costs one stat sweep; a
// broken config is logged and the previous pages stay up.
func reloadOnce(dir, last string) string {
	fp := watchPrint(dir)
	if fp == last {
		return last
	}
	cfg, err := config.Load(dir)
	if err != nil {
		log.Printf("reload failed, keeping previous config: %v", err)
		return fp
	}
	reloadMu.Lock()
	m, err := render.BuildModel(cfg, Current().Statuses)
	if err == nil {
		current.Store(m)
	}
	reloadMu.Unlock()
	if err != nil {
		log.Printf("reload failed, keeping previous config: %v", err)
		return fp
	}
	log.Printf("config reloaded: %d services, locales %v", config.CountServices(cfg), cfg.Site.Locales)
	return fp
}

// pollStatus feeds the status dots from the Gatus API, server-side only. On
// any fetch problem the dots disappear rather than go stale.
func Poll() {
	if cfg := Current().Cfg; cfg.Site.StatusAddress() != "" {
		log.Printf("status: polling %s at %s every %s", cfg.Site.StatusProvider(), cfg.Site.StatusAddress(), cfg.StatusInterval())
		switch st := cfg.Site.Status; {
		case st.Insecure:
			log.Print("status: insecure is on, so the certificate gatus presents is not verified at all: anything answering on that address decides what the pills say")
		case strings.HasPrefix(st.CA, "http://"):
			log.Printf("status: ca is %s, fetched over http, so whatever sits on the path to that address decides what cairn trusts for the poll: serve it over https, or mount it as a file, to close that", st.CA)
		case st.CA != "":
			log.Printf("status: verifying gatus against %s on top of the system roots", st.CA)
		}
	}
	var seen pollLog
	for {
		m := Current()
		pollOnce(source(m.Cfg), &seen)
		time.Sleep(m.Cfg.StatusInterval())
	}
}

// source is the status block as the poller needs it.
func source(cfg *config.Config) status.Source {
	return status.Source{
		URL:      cfg.Site.StatusAddress(),
		Provider: cfg.Site.StatusProvider(),
		Slug:     cfg.Site.Status.Slug,
		// The two mapping types are deliberately separate, since config
		// imports nothing of cairn's; this is where one becomes the other.
		TokenFile:   cfg.Site.Status.TokenFile,
		TokenScheme: cfg.Site.Status.TokenScheme,
		Map: status.Mapping{
			List:        cfg.Site.Status.Map.List,
			Key:         cfg.Site.Status.Map.Key,
			State:       cfg.Site.Status.Map.State,
			Up:          cfg.Site.Status.Map.Up,
			Degraded:    cfg.Site.Status.Map.Degraded,
			Maintenance: cfg.Site.Status.Map.Maintenance,
		},
		Insecure: cfg.Site.Status.Insecure,
		CA:       cfg.Site.Status.CA,
	}
}

// pollLog remembers the last messages printed, so a Gatus that stays down
// says so once instead of every interval.
type pollLog struct{ err, missing string }

// pollOnce runs one status poll and swaps the pages in when the dots changed.
// An empty url is a no-op: a site without Gatus shows no pill at all.
func pollOnce(src status.Source, seen *pollLog) {
	if src.URL == "" {
		return
	}
	st, err := status.Fetch(src)
	if err != nil {
		st = nil
		if err.Error() != seen.err {
			log.Printf("status: %v (dots hidden until gatus answers)", err)
			seen.err = err.Error()
		}
	} else {
		seen.err = ""
		if msg := status.Unmonitored(Current().Cfg, st); msg != seen.missing {
			if msg != "" {
				log.Printf("status: %s", msg)
			}
			seen.missing = msg
		}
	}
	reloadMu.Lock()
	defer reloadMu.Unlock()
	// Re-read under the lock so a config reload that landed mid-poll is merged
	// with, not overwritten by, the fresh statuses.
	cur := Current()
	if maps.Equal(st, cur.Statuses) {
		return
	}
	if next, err := render.BuildModel(cur.Cfg, st); err == nil {
		current.Store(next)
	} else {
		log.Printf("status: render failed, keeping previous pages: %v", err)
	}
}

// fingerprint walks the tree rather than listing one level. A directory's own
// mtime does not move when a file inside it is overwritten, so media/shot.png
// replaced in place used to stay invisible: the page kept declaring the old
// image's width and height until something unrelated touched the top level.
//
// The walk is bounded by what a config directory holds, a handful of yaml and
// a media folder, and it runs on the same two-second tick as before.
func fingerprint(dir string) string {
	var b strings.Builder
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			// An unreadable entry still has to change the fingerprint when it
			// appears or goes away, or a permission flip would go unnoticed.
			fmt.Fprintf(&b, "%s|err;", p)
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		fmt.Fprintf(&b, "%s|%d|%d;", p, info.Size(), info.ModTime().UnixNano())
		return nil
	})
	return b.String()
}
