package server

import (
	"fmt"
	"log"
	"maps"
	"os"
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
	if cfg := Current().Cfg; cfg.Site.Status.Gatus != "" {
		log.Printf("status: polling gatus at %s every %s", cfg.Site.Status.Gatus, cfg.StatusInterval())
	}
	var seen pollLog
	for {
		m := Current()
		pollOnce(m.Cfg.Site.Status.Gatus, &seen)
		time.Sleep(m.Cfg.StatusInterval())
	}
}

// pollLog remembers the last messages printed, so a Gatus that stays down
// says so once instead of every interval.
type pollLog struct{ err, missing string }

// pollOnce runs one status poll and swaps the pages in when the dots changed.
// An empty url is a no-op: a site without Gatus shows no pill at all.
func pollOnce(url string, seen *pollLog) {
	if url == "" {
		return
	}
	st, err := status.Fetch(url)
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

func fingerprint(dir string) string {
	var b strings.Builder
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if info, err := e.Info(); err == nil {
			fmt.Fprintf(&b, "%s|%d|%d;", e.Name(), info.Size(), info.ModTime().UnixNano())
		}
	}
	return b.String()
}
