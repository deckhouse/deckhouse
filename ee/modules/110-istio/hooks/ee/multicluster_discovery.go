/*
Copyright 2022 Flant JSC
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
	Queue: lib.Queue("multicluster"),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "multiclusters",
			ApiVersion:                   "deckhouse.io/v1alpha1",
			Kind:                         string(metadataExporter.AllianceKindMulticluster),
			ExecuteHookOnEvents:          ptr.To(true),
			ExecuteHookOnSynchronization: ptr.To(true),
			FilterFunc:                   applyMulticlusterFilter,
		},
	},
	Schedule: []go_hook.ScheduleConfig{
		{Name: "cron", Crontab: "* * * * *"},
	},
}, dependency.WithExternalDependencies(multiclusterDiscovery))

func applyMulticlusterFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var multicluster eeCrd.IstioMulticluster

	err := sdk.FromUnstructured(obj, &multicluster)
	if err != nil {
		return nil, err
	}

	clusterUUID := ""
	if multicluster.Status.MetadataCache.Public != nil {
		clusterUUID = multicluster.Status.MetadataCache.Public.ClusterUUID
	}

	me := multicluster.Spec.MetadataEndpoint
	me = strings.TrimSuffix(me, "/")

	return metadataExporter.MulticlusterCrdInfo{
		CommonInfo: &metadataExporter.CommonInfo{
			AllianceKind:             string(metadataExporter.AllianceKindMulticluster),
			Name:                     multicluster.GetName(),
			ClusterCA:                multicluster.Spec.Metadata.ClusterCA,
			EnableInsecureConnection: multicluster.Spec.Metadata.EnableInsecureConnection,
			ClusterUUID:              clusterUUID,
			PublicMetadataEndpoint:   me + "/public/public.json",
			PrivateMetadataEndpoint:  me + "/private/multicluster.json",
		},
		EnableIngressGateway: multicluster.Spec.EnableIngressGateway,
	}, nil
}

func multiclusterDiscovery(_ context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	discovery, err := metadataExporter.New(input, metadataExporter.TypeMulticluster, dc)
	if err != nil {
		return nil
	}

	for multiclusterInfo, err := range sdkobjectpatch.SnapshotIter[metadataExporter.MulticlusterCrdInfo](input.Snapshots.Get("multiclusters")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over multiclusters: %w", err)
		}

		if skip, err := discovery.RunDiscoveryOf(&multiclusterInfo); skip {
			continue
		} else if err != nil {
			return err
		}
	}
	return nil
}
