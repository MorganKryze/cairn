package main

import (
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/config"
	"github.com/MorganKryze/cairn/src/internal/testutil"
)

// -emit-gatus and -emit-icons both print a file derived from the config and
// exit. They are the two commands an operator pipes into another file, so what
// they print has to be usable as-is.
func TestOneShotOutputs(t *testing.T) {
	cfg, err := config.Load(testutil.WriteFiles(t, map[string]string{
		"site.yaml":     "locales: [en]\n",
		"services.yaml": "- {id: pad, url: https://pad.example.org, icon: hedgedoc, name: Pad}\n",
	}))
	if err != nil {
		t.Fatal(err)
	}

	gatus, err := oneShot(cfg, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"endpoints:", "name: pad", "https://pad.example.org"} {
		if !strings.Contains(string(gatus), want) {
			t.Errorf("emitted gatus config missing %q:\n%s", want, gatus)
		}
	}

	icons, err := oneShot(cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"#!/bin/sh", "mkdir -p icons", "icons/hedgedoc.svg"} {
		if !strings.Contains(string(icons), want) {
			t.Errorf("emitted icon script missing %q:\n%s", want, icons)
		}
	}
}
