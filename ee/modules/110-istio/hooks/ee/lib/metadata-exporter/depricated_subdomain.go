/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package metadataExporter

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
)

const (
	DepricatedSubdomainMetricsGroup = "d8_istio_metadata_exporter_endpoint_issues"
	DepricatedSubdomainMetricName   = "d8_istio_metadata_exporter_endpoint_uses_depricated_subdomain"
	DepricatedLeftmostSubdomain     = "istio"
)

// this file contains just common for both federation and multicluster helpers without implementation
// tests have to be deleted if no functions below using anymore
// const (
//
//	DepricatedSubdomainMetricsGroup = "d8_istio_metadata_exporter_endpoint_issues"
//	DepricatedSubdomainMetricName   = "d8_istio_metadata_exporter_endpoint_uses_depricated_subdomain"
//	DepricatedLeftmostSubdomain     = "istio"
//
// )

func AlertIfHasDeprecatedSubdomain(input *go_hook.HookInput, allianceKind string, clusterName string, endpointURL string) {
	isDeprecated, err := hasDepricatedSubdomainInURL(endpointURL)
	if err != nil {
		input.Logger.Warn("failed to validate metadataEndpoint subdomain: %v", err)
		return
	}
	if !isDeprecated {
		return
	}
	labels := map[string]string{
		"alliance_kind":     allianceKind,
		"multicluster_name": clusterName,
		"endpoint":          endpointURL,
	}
	input.MetricsCollector.Set(DepricatedSubdomainMetricName, 1, labels, metrics.WithGroup(DepricatedSubdomainMetricsGroup))
}

/*
	func (i IstioMulticlusterDiscoveryCrdInfo) AlertIfHasDeprecatedSubdomainNew(input *go_hook.HookInput, allianceKind string, clusterName string, endpointURL string) {
		isDeprecated, err := hasDepricatedSubdomainInURL(endpointURL)
		if err != nil {
			input.Logger.Warn("failed to validate metadataEndpoint subdomain: %v", err)
			return
		}
		if !isDeprecated {
			return
		}
		labels := map[string]string{
			"alliance_kind":     allianceKind,
			"multicluster_name": clusterName,
			"endpoint":          endpointURL,
		}
		input.MetricsCollector.Set(DepricatedSubdomainMetricName, 1, labels, metrics.WithGroup(DepricatedSubdomainMetricsGroup))
	}
*/
func hasDepricatedSubdomainInURL(url string) (bool, error) {
	usingSubdomain, err := retriveLeftmostSubdomain(url)
	if err != nil {
		return false, err
	}
	return usingSubdomain == DepricatedLeftmostSubdomain, nil
}

func retriveLeftmostSubdomain(raw string) (string, error) {
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
	h := u.Hostname()
	if h == "" {
		return "", fmt.Errorf("no host in metadataEndpoint")
	}
	h = strings.TrimSpace(strings.ToLower(h))
	sub, _, _ := strings.Cut(strings.TrimSuffix(h, "."), ".")
	return sub, nil
}

/*
type allianceMetadataEndpointSnapshot struct {
	AllianceKind string
	Name         string
	Endpoint     string
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: lib.Queue("metadata-exporter"),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "federations_metadata_endpoint",
			ApiVersion:                   "deckhouse.io/v1alpha1",
			Kind:                         "IstioFederation",
			ExecuteHookOnEvents:          ptr.To(true),
			ExecuteHookOnSynchronization: ptr.To(true),
			FilterFunc:                   applyFederationMetadataEndpointHostFilter,
		},
		{
			Name:                         "multiclusters_metadata_endpoint",
			ApiVersion:                   "deckhouse.io/v1alpha1",
			Kind:                         "IstioMulticluster",
			ExecuteHookOnEvents:          ptr.To(true),
			ExecuteHookOnSynchronization: ptr.To(true),
			FilterFunc:                   applyMulticlusterMetadataEndpointHostFilter,
		},
	},
	Schedule: []go_hook.ScheduleConfig{
		{Name: "cron", Crontab: "* * * * *"},
	},
}, allianceMetadataEndpointDepricatedSubdomain)

func applyFederationMetadataEndpointHostFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var f eeCrd.IstioFederation
	if err := sdk.FromUnstructured(obj, &f); err != nil {
		return nil, err
	}
	return allianceMetadataEndpointSnapshot{
		AllianceKind: "IstioFederation",
		Name:         f.GetName(),
		Endpoint:     strings.TrimSpace(f.Spec.MetadataEndpoint),
	}, nil
}

func applyMulticlusterMetadataEndpointHostFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var m eeCrd.IstioMulticluster
	if err := sdk.FromUnstructured(obj, &m); err != nil {
		return nil, err
	}
	return allianceMetadataEndpointSnapshot{
		AllianceKind: "IstioMulticluster",
		Name:         m.GetName(),
		Endpoint:     strings.TrimSpace(m.Spec.MetadataEndpoint),
	}, nil
}

func allianceMetadataEndpointDepricatedSubdomain(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(metadataExporterDepricatedSubdomainMetricsGroup)

	federation := input.Values.Get("istio.federation.enabled").Bool()
	multicluster := input.Values.Get("istio.multicluster.enabled").Bool()
	if !federation && !multicluster {
		return nil
	}
	emit := func(snap allianceMetadataEndpointSnapshot) {
		if snap.AllianceKind == "IstioFederation" && !federation {
			return
		}
		if snap.AllianceKind == "IstioMulticluster" && !multicluster {
			return
		}
		if !hasDepricatedSubdomainInHost(snap.Endpoint) {
			return
		}
		input.MetricsCollector.Set(metadataExporterDepricatedSubdomainMetricName, 1, map[string]string{
			"alliance_kind": snap.AllianceKind,
			"name":          snap.Name,
		}, metrics.WithGroup(metadataExporterDepricatedSubdomainMetricsGroup))
	}

	for snap, err := range sdkobjectpatch.SnapshotIter[allianceMetadataEndpointSnapshot](input.Snapshots.Get("federations_metadata_endpoint")) {
		if err != nil {
			return fmt.Errorf("federations_metadata_endpoint snapshot: %w", err)
		}
		emit(snap)
	}

	for snap, err := range sdkobjectpatch.SnapshotIter[allianceMetadataEndpointSnapshot](input.Snapshots.Get("multiclusters_metadata_endpoint")) {
		if err != nil {
			return fmt.Errorf("multiclusters_metadata_endpoint snapshot: %w", err)
		}
		emit(snap)
	}

	return nil
}
*/
