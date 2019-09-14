package proxy

import (
	"net/http"

	"github.com/coreos/pkg/capnslog"
)

var logger = capnslog.NewPackageLogger("crowd-auth-proxy", "proxy")

func healthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func RunProxy(listenAddress, apiServerUrl, login, password, certPath, crowdBaseUrl string, cacheTTL int) {
	logger.Printf("-- Listening on: %s", listenAddress)
	logger.Printf("-- Atlassian Crowd URL: %s", crowdBaseUrl)
	logger.Printf("-- Kubernetes API URL: %s", apiServerUrl)
	logger.Printf("-- Cache TTL: %v", cacheTTL)

	h := newHandler(crowdBaseUrl, apiServerUrl, login, password, certPath, cacheTTL)
	http.Handle("/", h)
	http.HandleFunc("/healthz", healthz)
	logger.Fatal(http.ListenAndServe(listenAddress, nil))
}
