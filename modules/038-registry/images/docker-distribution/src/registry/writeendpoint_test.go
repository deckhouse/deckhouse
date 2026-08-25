package registry

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"syscall"
	"testing"
	"time"

	"github.com/docker/distribution/configuration"
)

// A cache with a write endpoint, as the storage of this module runs it: proxying on one address,
// accepting pushes on another, one process over one store.
func cachingRegistry(t *testing.T, upstream, addr, writeAddr string) *Registry {
	t.Helper()

	config := &configuration.Configuration{}
	config.HTTP.Addr = addr
	config.HTTP.DrainTimeout = 10 * time.Second
	config.Storage = map[string]configuration.Parameters{"inmemory": map[string]interface{}{}}
	config.Proxy.RemoteURL = upstream
	config.Proxy.SkipModeCleanup = true
	config.WriteEndpoint.Addr = writeAddr

	registry, err := NewRegistry(context.Background(), config)
	if err != nil {
		t.Fatalf("building the registry: %v", err)
	}
	return registry
}

// The two halves of one process: the address the cluster pulls through refuses a push, because it is
// a pull-through cache and answering a push there would skip every layer the upstream already has.
// The write address accepts it.
func TestTheWriteEndpointAcceptsWhatTheCacheRefuses(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	registry := cachingRegistry(t, upstream.URL, "127.0.0.1:15001", "127.0.0.1:15003")

	// The signal channel is shared by every registry in this process, and the tests before this one
	// leave values in it. One left behind would stop this registry the moment it starts serving.
	drainQuit()

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- registry.ListenAndServe()
	}()
	defer stop(t, serveErr)

	waitForListener(t, "127.0.0.1:15001")
	waitForListener(t, "127.0.0.1:15003")

	select {
	case err := <-serveErr:
		t.Fatalf("the registry stopped: %v", err)
	default:
	}

	// A push against the serving half: the cache has no write path, and says so.
	refused := post(t, "http://127.0.0.1:15001/v2/system/deckhouse/test/blobs/uploads/")
	if refused == http.StatusAccepted {
		t.Fatalf("the serving half accepted an upload (%d); a push there skips layers the upstream holds", refused)
	}

	// And against the write half, which is the same store with no proxy in front of it.
	accepted := post(t, "http://127.0.0.1:15003/v2/system/deckhouse/test/blobs/uploads/")
	if accepted != http.StatusAccepted {
		t.Fatalf("the write endpoint answered %d, want %d", accepted, http.StatusAccepted)
	}
}

// Two listeners in one process cannot share an address, and the failure would otherwise be one half
// silently missing after the other won the port.
func TestOneAddressForBothHalvesIsRefused(t *testing.T) {
	config := &configuration.Configuration{}
	config.HTTP.Addr = "127.0.0.1:15001"
	config.Storage = map[string]configuration.Parameters{"inmemory": map[string]interface{}{}}
	config.WriteEndpoint.Addr = config.HTTP.Addr

	if _, err := NewRegistry(context.Background(), config); err == nil {
		t.Fatal("a write endpoint on the serving address was accepted")
	}
}

// What the write endpoint inherits, and what it must not.
func TestTheWriteEndpointIsDerivedFromTheServingConfiguration(t *testing.T) {
	config := &configuration.Configuration{}
	config.HTTP.Addr = "0.0.0.0:5001"
	config.HTTP.Debug.Addr = "127.0.0.1:5002"
	config.HTTP.Debug.Prometheus.Enabled = true
	config.HTTP.TLS.Certificate = "/pki/distribution.crt"
	config.Storage = map[string]configuration.Parameters{
		"filesystem": map[string]interface{}{"rootdirectory": "/data"},
	}
	config.Proxy.RemoteURL = "https://registry.example.com"
	config.Proxy.SkipModeCleanup = true
	config.WriteEndpoint.Addr = "0.0.0.0:5003"
	config.WriteEndpoint.ClientCertCA = "/pki/ingress-client-ca.crt"

	writeConfig := writeEndpointConfiguration(config)

	if writeConfig.Proxy.RemoteURL != "" {
		t.Fatalf("the write endpoint proxies to %q; a push must be answered by the store", writeConfig.Proxy.RemoteURL)
	}
	if !writeConfig.Proxy.SkipModeCleanup {
		t.Fatal("the write endpoint would clean up the store it shares, having decided the mode changed")
	}
	if writeConfig.HTTP.Addr != "0.0.0.0:5003" {
		t.Fatalf("the write endpoint listens on %q, want the configured address", writeConfig.HTTP.Addr)
	}
	if writeConfig.HTTP.Debug.Prometheus.Enabled || writeConfig.HTTP.Debug.Addr != "" {
		t.Fatal("the write endpoint keeps a debug listener; registering its metrics again panics the process")
	}
	if !writeConfig.HTTP.RealIP.Enabled || writeConfig.HTTP.RealIP.ClientCert.CA != "/pki/ingress-client-ca.crt" {
		t.Fatal("the write endpoint does not trust the ingress for the real client address")
	}

	// Inherited: the same storage, the same serving material, so the two halves cannot drift.
	if writeConfig.Storage["filesystem"]["rootdirectory"] != "/data" {
		t.Fatalf("the write endpoint stores elsewhere: %v", writeConfig.Storage)
	}
	if writeConfig.HTTP.TLS.Certificate != config.HTTP.TLS.Certificate {
		t.Fatal("the write endpoint serves a different certificate")
	}

	// And nothing of this reached the half that serves reads.
	if config.Proxy.RemoteURL == "" || !config.HTTP.Debug.Prometheus.Enabled {
		t.Fatal("deriving the write endpoint changed the serving configuration")
	}
	writeConfig.Storage["filesystem"]["rootdirectory"] = "/elsewhere"
	if config.Storage["filesystem"]["rootdirectory"] != "/data" {
		t.Fatal("the two halves share their storage parameters")
	}
}

// stop shuts both halves down and waits, so that nothing of this test is still listening — or still
// sitting in the signal channel — when the next one runs.
func stop(t *testing.T, serveErr <-chan error) {
	t.Helper()

	quit <- syscall.SIGTERM

	select {
	case <-serveErr:
	case <-time.After(30 * time.Second):
		t.Fatal("the registry did not stop")
	}

	drainQuit()
}

func drainQuit() {
	for {
		select {
		case <-quit:
		default:
			return
		}
	}
}

func post(t *testing.T, url string) int {
	t.Helper()

	response, err := http.Post(url, "application/octet-stream", nil)
	if err != nil {
		t.Fatalf("POST %v: %v", url, err)
	}
	defer response.Body.Close()
	return response.StatusCode
}

func waitForListener(t *testing.T, addr string) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		response, err := http.Get(fmt.Sprintf("http://%s/v2/", addr))
		if err == nil {
			response.Body.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("%v never answered", addr)
}
