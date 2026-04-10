/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package ee

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	eeCrd "github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/ee/lib/crd"
	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib"
)

const (
	metadataExporterDepricatedSubdomainMetricsGroup = "d8_istio_metadata_exporter_endpoint_issues"
	metadataExporterDepricatedSubdomainMetricName   = "d8_istio_metadata_exporter_endpoint_uses_depricated_subdomain"
	depricatedMetadataEndpointSubdomain             = "istio"
)

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

func hasDepricatedSubdomainInHost(endpoint string) bool {
	host, err := prepareEndpointString(endpoint)
	if err != nil {
		return false
	}
	host = strings.TrimSpace(strings.ToLower(host))

	if host == "" {
		return false
	}
	host = strings.TrimSuffix(host, ".")
	subdomain, _, _ := strings.Cut(host, ".")
	return subdomain == depricatedMetadataEndpointSubdomain
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
