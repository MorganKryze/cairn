package config

// Icon resolution. A bare slug resolves to a local file when the operator
// has one in <assets>/icons/, and to dashboard-icons on jsdelivr otherwise,
// the convention Homepage and Homarr already use. -emit-icons prints the
// downloads needed to go fully self-hosted, so the page makes no
// third-party request at all.

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// AssetsPath mirrors the -assets flag; Load scans it for local icons.
var AssetsPath = "/assets"

const iconCDN = "https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/svg/"

// IconURL resolves a service icon: URLs and absolute paths pass through,
// a slug prefers the operator's own file, then the CDN.
func IconURL(cfg *Config, icon string) string {
	if icon == "" || IsURLOrAbs(icon) {
		return icon
	}
	if p := cfg.LocalIcons[icon]; p != "" {
		return p
	}
	return iconCDN + icon + ".svg"
}

// localIcons maps slug -> served path for every image in <assets>/icons/;
// an svg wins over other formats of the same name.
func localIcons(dir string) map[string]string {
	out := map[string]string{}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if e.IsDir() || (ext != ".svg" && ext != ".png" && ext != ".webp") {
			continue
		}
		slug := strings.TrimSuffix(name, filepath.Ext(name))
		if ext == ".svg" || out[slug] == "" {
			out[slug] = "/assets/icons/" + name
		}
	}
	return out
}

// CdnSlugs lists the icon slugs that would load from the CDN, sorted.
func CdnSlugs(cfg *Config) []string {
	seen := map[string]bool{}
	for _, c := range cfg.Categories {
		for _, s := range c.Services {
			// Both halves: a site self-hosting its icons needs the dark one
			// downloaded too, or half its cards go to the CDN after all.
			for _, ic := range s.Icon.Refs() {
				if IsURLOrAbs(ic) {
					continue
				}
				if cfg.LocalIcons[ic] == "" {
					seen[ic] = true
				}
			}
		}
	}
	slugs := make([]string, 0, len(seen))
	for s := range seen {
		slugs = append(slugs, s)
	}
	sort.Strings(slugs)
	return slugs
}

// shellQuote wraps a value in single quotes, the one form sh treats as fully
// literal, and closes-escapes-reopens for any quote inside it.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func EmitIconsScript(cfg *Config) []byte {
	var b bytes.Buffer
	b.WriteString("#!/bin/sh\n")
	b.WriteString("# Run this inside your assets folder: it downloads your icon slugs\n")
	b.WriteString("# into icons/, and cairn serves them from there automatically.\n")
	b.WriteString("set -e\nmkdir -p icons\n")
	for _, slug := range CdnSlugs(cfg) {
		// slugRe already keeps a shell metacharacter out of here; quoting the
		// value anyway means a future loosening of that rule cannot turn this
		// script, which the docs pipe into sh, into an execution vector.
		fmt.Fprintf(&b, "curl -fsSL -o %s %s\n", shellQuote("icons/"+slug+".svg"), shellQuote(iconCDN+slug+".svg"))
	}
	return b.Bytes()
}
