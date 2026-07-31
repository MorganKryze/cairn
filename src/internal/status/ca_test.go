package status_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MorganKryze/cairn/src/internal/config"
	"github.com/MorganKryze/cairn/src/internal/status"
)

// gatus is a TLS server holding a certificate of its very own, which is the
// shape of an internal Gatus, plus that certificate in PEM.
//
// The certificate is minted here rather than left to httptest.NewTLSServer,
// which hands every server the same built-in one. Two of those look like two
// authorities and are one, so a bundle carrying the first verifies the second
// and a test asserting otherwise passes on a client that checks nothing.
func gatus(t *testing.T) (*httptest.Server, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "gatus.internal"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"name":"pad","results":[{"success":true}]}]`))
	}))
	srv.TLS = &tls.Config{
		Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}},
		MinVersion:   tls.VersionTLS12,
	}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return srv, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

// status.ca is the answer status.insecure is not: it verifies rather than stops
// checking. Both halves have to be asserted, or a client that verified nothing
// would pass the half that matters.
func TestCABundleVerifiesWhatTheSystemRootsCannot(t *testing.T) {
	srv, ca := gatus(t)

	if _, err := status.Fetch(status.Source{URL: srv.URL}); err == nil {
		t.Fatal("a self-signed gatus verified against the system roots alone")
	}

	dir := t.TempDir()
	config.AssetsPath = dir
	if err := os.WriteFile(filepath.Join(dir, "ca.crt"), ca, 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := status.Fetch(status.Source{URL: srv.URL, CA: "/assets/ca.crt"})
	if err != nil {
		t.Fatalf("a bundle holding the server's own authority still failed: %v", err)
	}
	if !st["pad"].Up {
		t.Errorf("statuses = %v, want pad up: the body is read the same either side of the bundle", st)
	}
}

// The same bundle, served rather than mounted. This is the case the key exists
// for: a CDN instead of a ConfigMap.
func TestCABundleCanBeFetched(t *testing.T) {
	srv, ca := gatus(t)
	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(ca)
	}))
	defer cdn.Close()

	st, err := status.Fetch(status.Source{URL: srv.URL, CA: cdn.URL + "/ca.crt"})
	if err != nil {
		t.Fatalf("a fetched bundle failed: %v", err)
	}
	if !st["pad"].Up {
		t.Errorf("statuses = %v, want pad up", st)
	}
}

// Adding an authority is not the same as trusting everything. A second server
// with an authority of its own has to stay refused, or the key would be
// status.insecure wearing a better name.
func TestCABundleStillRefusesEverythingElse(t *testing.T) {
	_, ca := gatus(t)
	other, _ := gatus(t)

	dir := t.TempDir()
	config.AssetsPath = dir
	if err := os.WriteFile(filepath.Join(dir, "ca.crt"), ca, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := status.Fetch(status.Source{URL: other.URL, CA: "/assets/ca.crt"}); err == nil {
		t.Fatal("a server the bundle says nothing about verified anyway")
	}
}

// A CDN that is not up yet must not poison the rest of the run. cairn starting
// before the thing it depends on is the exact bug this release fixed once
// already; caching a failure would reintroduce it one layer down.
func TestAFailedBundleIsRetriedRatherThanRemembered(t *testing.T) {
	srv, ca := gatus(t)
	var up atomic.Bool
	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if !up.Load() {
			http.Error(w, "not yet", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write(ca)
	}))
	defer cdn.Close()

	src := status.Source{URL: srv.URL, CA: cdn.URL + "/ca.crt"}
	if _, err := status.Fetch(src); err == nil {
		t.Fatal("a bundle the server refused to serve was accepted")
	}
	up.Store(true)
	if _, err := status.Fetch(src); err != nil {
		t.Fatalf("the bundle never recovered once its host came up: %v", err)
	}
}

// What an operator gets wrong here is the file, not the URL: a DER, a key, a
// README. Every one of them reads as bytes and none of them is a certificate.
func TestABundleWithNoCertificateSaysSo(t *testing.T) {
	srv, _ := gatus(t)
	dir := t.TempDir()
	config.AssetsPath = dir
	if err := os.WriteFile(filepath.Join(dir, "ca.crt"), []byte("this is not a certificate\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := status.Fetch(status.Source{URL: srv.URL, CA: "/assets/ca.crt"})
	if err == nil {
		t.Fatal("a file holding no certificate was accepted as a trust anchor")
	}
	for _, want := range []string{"status.ca", "certificate"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not carry %q, so nobody knows which key to look at", err, want)
		}
	}
}

// The client is built once per bundle, not once per poll: a fresh transport
// every interval was the connection-pool leak fixed in 1.13.1, and a fetched
// bundle would add an outbound request every interval on top of it.
func TestTheTrustingClientIsBuiltOnce(t *testing.T) {
	srv, ca := gatus(t)
	var fetches atomic.Int64
	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fetches.Add(1)
		_, _ = w.Write(ca)
	}))
	defer cdn.Close()

	src := status.Source{URL: srv.URL, CA: cdn.URL + "/ca.crt"}
	for range 3 {
		if _, err := status.Fetch(src); err != nil {
			t.Fatalf("poll refused: %v", err)
		}
	}
	if n := fetches.Load(); n != 1 {
		t.Errorf("the bundle was fetched %d times for 3 polls, want 1", n)
	}
}
