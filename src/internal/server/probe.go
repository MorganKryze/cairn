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

func Probe(addr string) int {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 1
	}
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/healthz")
	if err != nil || resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}
