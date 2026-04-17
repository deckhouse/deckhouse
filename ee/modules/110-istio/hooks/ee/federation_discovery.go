/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package ee

import (
	"context"
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	eeCrd "github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/ee/lib/crd"
	metadataExporter "github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/ee/lib/metadata-exporter"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: lib.Queue("federation"),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "federations",
			ApiVersion:                   "deckhouse.io/v1alpha1",
			Kind:                         string(metadataExporter.AllianceKindFederation),
			ExecuteHookOnEvents:          ptr.To(true),
			ExecuteHookOnSynchronization: ptr.To(true),
			FilterFunc:                   applyFederationFilter,
		},
	},
	Schedule: []go_hook.ScheduleConfig{
		{Name: "cron", Crontab: "* * * * *"},
	},
}, dependency.WithExternalDependencies(federationDiscovery))

func applyFederationFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var federation eeCrd.IstioFederation

	err := sdk.FromUnstructured(obj, &federation)
	if err != nil {
		return nil, err
	}

	clusterUUID := ""
	if federation.Status.MetadataCache.Public != nil {
		clusterUUID = federation.Status.MetadataCache.Public.ClusterUUID
	}

	me := federation.Spec.MetadataEndpoint
	me = strings.TrimSuffix(me, "/")

	return metadataExporter.FederationCrdInfo{
		CommonInfo: &metadataExporter.CommonInfo{
			AllianceKind:             string(metadataExporter.AllianceKindFederation),
			Name:                     federation.GetName(),
			ClusterCA:                federation.Spec.Metadata.ClusterCA,
			EnableInsecureConnection: federation.Spec.Metadata.EnableInsecureConnection,
			ClusterUUID:              clusterUUID,
			PublicMetadataEndpoint:   me + "/public/public.json",
			PrivateMetadataEndpoint:  me + "/private/federation.json",
		},
		TrustDomain: federation.Spec.TrustDomain,
	}, nil
}

func federationDiscovery(_ context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	discovery, err := metadataExporter.New(input, metadataExporter.TypeFederation, dc)
	if err != nil {
		return nil
	}

	for federationInfo, err := range sdkobjectpatch.SnapshotIter[metadataExporter.FederationCrdInfo](input.Snapshots.Get("federations")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over federations: %w", err)
		}

		if skip, err := discovery.RunDiscoveryOf(&federationInfo); skip {
			continue
		} else if err != nil {
			return err
		}
	}
	return nil
}
