// Command cairn serves a directory page for the people you host services for.
//
// This file is wiring only: parse the flags, answer the one-shot commands,
// build the first model, hand everything to the server. The work lives in
// internal/config (read and validate the YAML), internal/render (turn it into
// bytes), internal/status (Gatus) and internal/server (HTTP).
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/MorganKryze/cairn/src/internal/check"
	"github.com/MorganKryze/cairn/src/internal/config"
	"github.com/MorganKryze/cairn/src/internal/render"
	"github.com/MorganKryze/cairn/src/internal/server"
	"github.com/MorganKryze/cairn/src/internal/status"
)

// version is stamped by the build: -ldflags "-X main.version=…"
var version = "dev"

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	cfgDir := flag.String("config", "/config", "config directory")
	assetsDir := flag.String("assets", "/assets", "optional directory served at /assets/")
	base := flag.String("base-path", "", "serve under a sub-path of the domain, e.g. /cairn (default: the domain root)")
	health := flag.Bool("healthcheck", false, "probe the running server and exit (for container healthchecks)")
	validate := flag.Bool("check", false, "validate the config directory, print warnings, and exit (0 ok, 1 error)")
	emit := flag.Bool("emit-gatus", false, "print a Gatus endpoints config derived from the services and exit")
	emitIcons := flag.Bool("emit-icons", false, "print a shell script that downloads your icon slugs for self-hosting and exit")
	initCfg := flag.Bool("init", false, "print a commented starter services.yaml and exit")
	ver := flag.Bool("version", false, "print the version and exit")
	flag.Parse()

	config.AssetsPath = *assetsDir
	render.BasePath = render.NormalizeBase(*base)
	render.Version = version

	if *ver {
		fmt.Println("cairn", version)
		return
	}
	if *initCfg {
		fmt.Print(config.StarterServices)
		return
	}
	if *health {
		os.Exit(server.Probe(*addr))
	}
	if *validate {
		os.Exit(check.RunCheck(*cfgDir))
	}

	cfg, err := config.Load(*cfgDir)
	if *emit || *emitIcons {
		if err != nil {
			log.Fatal(err)
		}
		out, err := oneShot(cfg, *emit)
		if err != nil {
			log.Fatal(err)
		}
		os.Stdout.Write(out)
		return
	}
	if err != nil {
		// A dead container helps nobody: serve the getting-started page and
		// let the watcher swap the real config in the moment it is valid.
		log.Print(err)
		log.Printf("no valid config yet: serving the getting-started page, watching %s", *cfgDir)
		server.Store(render.StarterModel())
	} else {
		m, merr := render.BuildModel(cfg, nil)
		if merr != nil {
			log.Fatal(merr)
		}
		server.Store(m)
	}

	go server.Watch(*cfgDir)
	go server.Poll()

	cfg = server.Current().Cfg
	log.Printf("cairn %s: %d services, locales %v, listening on %s%s",
		version, config.CountServices(cfg), cfg.Site.Locales, *addr, render.BasePath)
	log.Fatal(server.Serve(*addr, *cfgDir, *assetsDir))
}

// oneShot answers -emit-gatus and -emit-icons, which both print a derived file
// and exit without ever serving anything.
func oneShot(cfg *config.Config, gatus bool) ([]byte, error) {
	if gatus {
		return status.Emit(cfg)
	}
	return config.EmitIconsScript(cfg), nil
}
