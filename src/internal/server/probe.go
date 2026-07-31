package server

import (
	"io"
	"net"
	"net/http"
	"time"
)

// healthz answers while the process serves, whatever the config says. It is
// the liveness signal: a broken config is not a reason to kill a container
// that is happily serving the getting-started page.
func healthz(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, "ok\n") }

// readyz is the honest one: 503 while no valid config has ever loaded, so a
// monitor does not sit green on a site that only says "Almost there". A
// reload that fails later keeps the last good pages, which is degraded but
// genuinely serving, so it stays ready.
func readyz(w http.ResponseWriter, _ *http.Request) {
	if !Current().Ready {
		http.Error(w, "no valid config yet\n", http.StatusServiceUnavailable)
		return
	}
	io.WriteString(w, "ready\n")
}

// Probe asks the running server whether it is alive, for a container
// healthcheck that has no shell to do it with. It dials the address cairn was
// told to listen on, not loopback: keeping only the port meant an instance
// bound to one interface was probed somewhere else entirely, which either
// restart-loops a healthy container or, worse, reports green because something
// unrelated answers on that port.
func Probe(addr string) int {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 1
	}
	// An empty host is "every interface"; loopback is the one we can reach.
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://" + net.JoinHostPort(host, port) + "/healthz")
	if err != nil || resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}
