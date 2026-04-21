/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

var publicJSONDepricatedSubdomainRequests prometheus.Counter

func registerMetadataExporterMetrics(reg prometheus.Registerer) {
	c := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "d8_istio_metadata_exporter_accessed_via_deprecated_subdomain",
		Help: "Requests to `/metadata/private/<federation|multicluster>.json` whose Host equals this cluster depricated istio public hostname (istio.<publicDomainTemplate>).",
	})
	reg.MustRegister(c)
	publicJSONDepricatedSubdomainRequests = c
}

func prepareEndpointString(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("empty metadataEndpoint")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	host := u.Hostname()
	if host == "" {
		return "", fmt.Errorf("no host in metadataEndpoint")
	}
	return strings.ToLower(host), nil
}

func checkIfAccessedViaDepricatedSubdomain(r *http.Request) {
	dontWant := strings.TrimSpace(os.Getenv("ISTIO_METADATA_OLD_PUBLIC_HOST"))

	if dontWant == "" {
		return
	}
	got, err := prepareEndpointString(r.Host)
	if err != nil {
		return
	}
	if got == "" || got != dontWant {
		logger.Printf("Log: dontWant=%s got=%s", dontWant, got)
		return
	}
	publicJSONDepricatedSubdomainRequests.Inc()
	logger.Printf("Depricated subdomain usage: dontWant=%s got=%s", dontWant, got)
}
