/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package metadataExporter

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"

	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	DeprecatedSubdomainMetricName = "d8_istio_metadata_exporter_endpoint_uses_deprecated_subdomain"
	DeprecatedLeftmostSubdomain   = "istio"
)

func (c *CommonInfo) ifHasDeprecatedSubdomainAlert(d *Discovery) {
	isDeprecated, err := hasDeprecatedSubdomainInURL(c.PublicMetadataEndpoint)
	if err != nil {
		d.input.Logger.Warn(
			"failed to validate metadataEndpoint subdomain",
			log.Err(err),
		)
	}
	if !isDeprecated {
		return
	}
	hostname, err := retrieveHostname(c.PublicMetadataEndpoint)
	if err != nil {
		d.input.Logger.Warn(
			"failed to retrieveHostname",
			log.Err(err),
		)
	}
	labels := map[string]string{
		"alliance_kind": c.AllianceKind,
		"name":          c.Name,
		"hostname":      hostname,
	}
	d.input.MetricsCollector.Set(DeprecatedSubdomainMetricName, 1, labels, metrics.WithGroup(string(d.metricsGroup)))
}

func hasDeprecatedSubdomainInURL(url string) (bool, error) {
	usingSubdomain, err := retrieveLeftmostSubdomain(url)
	if err != nil {
		return false, err
	}
	return usingSubdomain == DeprecatedLeftmostSubdomain, nil
}

func retrieveLeftmostSubdomain(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	h, err := retrieveHostname(raw)
	if err != nil {
		return "", err
	}
	if h == "" {
		return "", fmt.Errorf("no host in metadataEndpoint")
	}
	h = strings.TrimSpace(strings.ToLower(h))
	sub, _, _ := strings.Cut(strings.TrimSuffix(h, "."), ".")
	return sub, nil
}

func retrieveHostname(raw string) (string, error) {
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
	return u.Hostname(), nil
}
