/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	storage "k8s.io/api/storage/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/lib-dhctl/pkg/yaml/validation"
	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	cloudDataV1 "github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "cloud_provider_discovery_data",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-cloud-provider-discovery-data"},
			},
			FilterFunc: applyCloudProviderDiscoveryDataSecretFilter,
		},
		{
			Name:       "storage_classes",
			ApiVersion: "storage.k8s.io/v1",
			Kind:       "StorageClass",
			FilterFunc: applyStorageClassFilter,
			LabelSelector: &meta.LabelSelector{
				MatchLabels: map[string]string{
					"heritage": "deckhouse",
					"module":   "cloud-provider-dynamix",
				},
			},
		},
	},
}, handleCloudProviderDiscoveryDataSecret)

func applyCloudProviderDiscoveryDataSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to convert kubernetes object: %v", err)
	}

	return secret, nil
}

func applyStorageClassFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	storageClass := &storage.StorageClass{}
	err := sdk.FromUnstructured(obj, storageClass)
	if err != nil {
		return nil, fmt.Errorf("failed to convert kubernetes object: %v", err)
	}

	return storageClass, nil
}

func handleCloudProviderDiscoveryDataSecret(_ context.Context, input *go_hook.HookInput) error {
	if len(input.Snapshots.Get("cloud_provider_discovery_data")) == 0 {
		input.Logger.Warn("failed to find secret 'd8-cloud-provider-discovery-data' in namespace 'kube-system'")

		if len(input.Snapshots.Get("storage_classes")) == 0 {
			input.Logger.Warn("failed to find storage classes for dynamix provisioner")

			return nil
		}

		storageClassesSnapshots := input.Snapshots.Get("storage_classes")

		storageClasses := make([]storageClass, 0, len(storageClassesSnapshots))

		for sc, err := range sdkobjectpatch.SnapshotIter[storage.StorageClass](storageClassesSnapshots) {
			if err != nil {
				return fmt.Errorf("failed to iterate over storage classes: %v", err)
			}

			storageClasses = append(storageClasses, storageClass{
				Name:          sc.Name,
				StoragePolicy: sc.Parameters["storagePolicy"],
			})
		}

		orderStorageClasses(input, storageClasses)
		setStorageClassesValues(input, storageClasses)

		return nil
	}

	secret := new(v1.Secret)

	snaps := input.Snapshots.Get("cloud_provider_discovery_data")
	if len(snaps) == 0 {
		return fmt.Errorf("cloud_provider_discovery_data snapshot is empty")
	}

	err := snaps[0].UnmarshalTo(secret)
	if err != nil {
		return fmt.Errorf("failed to unmarshal secret: %w", err)
	}

	discoveryDataJSON := secret.Data["discovery-data.json"]

	// The first path is where the schemas live in the source tree (hook tests read them
	// from there), the second is where werf puts them in the deckhouse image. A missing
	// directory is silently skipped, so both are always passed.
	if err := validation.ValidateData([]string{"/deckhouse/ee/modules/030-cloud-provider-dynamix/candi/openapi", "/deckhouse/candi/cloud-providers/dynamix/openapi"}, &discoveryDataJSON); err != nil {
		return fmt.Errorf("failed to validate 'discovery-data.json' from 'd8-cloud-provider-discovery-data' secret: %w", err)
	}

	var discoveryData cloudDataV1.DynamixCloudProviderDiscoveryData
	err = json.Unmarshal(discoveryDataJSON, &discoveryData)
	if err != nil {
		return fmt.Errorf("failed to unmarshal 'discovery-data.json' from 'd8-cloud-provider-discovery-data' secret: %v", err)
	}

	input.Values.Set("cloudProviderDynamix.internal.providerDiscoveryData", discoveryData)

	storageClasses := storageClassesFromPolicies(input, discoveryData.StoragePolicies)
	orderStorageClasses(input, storageClasses)
	setStorageClassesValues(input, storageClasses)

	return deleteStorageClassesWithOutdatedParameters(input, storageClasses)
}

// storageClassesFromPolicies builds exactly one StorageClass per storage policy
// available to the account. The discoverer publishes only ENABLED policies, and the
// platform picks the storage endpoint and pool inside a policy on its own, so there
// is nothing left to unpack here.
func storageClassesFromPolicies(input *go_hook.HookInput, policies []cloudDataV1.DynamixStoragePolicy) []storageClass {
	// Policy names are normalized into StorageClass names, so two different policies
	// can collide on one name. Walking them in name order makes the winner of such a
	// collision independent of the order the discoverer happened to publish them in.
	policyNames := make([]string, 0, len(policies))
	for _, policy := range policies {
		policyNames = append(policyNames, policy.Name)
	}
	sort.Strings(policyNames)

	storageClassesMap := make(map[string]storageClass, len(policyNames))

	for _, policyName := range policyNames {
		name := getStorageClassName(policyName)
		if name == "" {
			input.Logger.Warn("skipping storage policy: its name cannot be converted into a StorageClass name", slog.String("storage_policy", policyName))

			continue
		}

		if kept, taken := storageClassesMap[name]; taken {
			input.Logger.Warn("skipping storage policy: another one already claims the same StorageClass name",
				slog.String("storage_class", name),
				slog.String("storage_policy", policyName),
				slog.String("kept_storage_policy", kept.StoragePolicy))

			continue
		}

		storageClassesMap[name] = storageClass{
			Name:          name,
			StoragePolicy: policyName,
		}
	}

	excludes, ok := input.Values.GetOk("cloudProviderDynamix.storageClass.exclude")
	if ok {
		for _, esc := range excludes.Array() {
			rg := regexp.MustCompile("^(" + esc.String() + ")$")
			for name := range storageClassesMap {
				if rg.MatchString(name) {
					delete(storageClassesMap, name)
				}
			}
		}
	}

	storageClasses := make([]storageClass, 0, len(storageClassesMap))
	for _, sc := range storageClassesMap {
		storageClasses = append(storageClasses, sc)
	}

	return storageClasses
}

// deleteStorageClassesWithOutdatedParameters removes StorageClasses the chart is about
// to render with different parameters. StorageClass.parameters are immutable in
// Kubernetes, so Helm cannot patch such an object in place and the whole release
// upgrade fails. The hook runs before Helm, so the very same upgrade recreates them.
//
// StorageClasses that are no longer rendered at all need no help: they simply leave the
// release and Helm deletes them.
func deleteStorageClassesWithOutdatedParameters(input *go_hook.HookInput, storageClasses []storageClass) error {
	renderedPolicies := make(map[string]string, len(storageClasses))
	for _, sc := range storageClasses {
		renderedPolicies[sc.Name] = sc.StoragePolicy
	}

	for sc, err := range sdkobjectpatch.SnapshotIter[storage.StorageClass](input.Snapshots.Get("storage_classes")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over storage classes: %v", err)
		}

		storagePolicy, rendered := renderedPolicies[sc.Name]
		if !rendered || sc.Parameters["storagePolicy"] == storagePolicy {
			continue
		}

		input.Logger.Info("deleting storage class because its parameters have changed",
			slog.String("storage_class", sc.Name),
			slog.String("storage_policy", storagePolicy))

		input.PatchCollector.Delete("storage.k8s.io/v1", "StorageClass", "", sc.Name)
	}

	return nil
}

// Get StorageClass name from a storage policy name to match Kubernetes restrictions from https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names
func getStorageClassName(value string) string {
	mapFn := func(r rune) rune {
		if r >= 'a' && r <= 'z' ||
			r >= 'A' && r <= 'Z' ||
			r >= '0' && r <= '9' ||
			r == '-' || r == '.' {
			return unicode.ToLower(r)
		} else if r == ' ' {
			return '-'
		}
		return rune(-1)
	}

	// a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.'
	value = strings.Map(mapFn, value)

	// must start and end with an alphanumeric character
	return strings.Trim(value, "-.")
}

// orderStorageClasses sorts storageClasses in place, by name, except that the one named
// in storageClass.default comes first.
//
// helm_lib_module_storage_class_annotations marks the StorageClass at index 0 as the
// cluster default, which is how storageClass.default takes effect. It only does so while
// nothing else claims the default: a StorageClass already carrying the default annotation
// wins (global.discovery.defaultStorageClass), and global.defaultClusterStorageClass wins
// over both. So this picks the default of a cluster that has none yet, not a switch that
// moves the annotation later on -- which is why the parameter is deprecated in favour of
// global.defaultClusterStorageClass.
func orderStorageClasses(input *go_hook.HookInput, storageClasses []storageClass) {
	defaultName := input.Values.Get("cloudProviderDynamix.storageClass.default").String()

	sort.SliceStable(storageClasses, func(i, j int) bool {
		iIsDefault := storageClasses[i].Name == defaultName
		jIsDefault := storageClasses[j].Name == defaultName
		if iIsDefault != jIsDefault {
			return iIsDefault
		}

		return storageClasses[i].Name < storageClasses[j].Name
	})
}

func setStorageClassesValues(input *go_hook.HookInput, storageClasses []storageClass) {
	input.Values.Set("cloudProviderDynamix.internal.storageClasses", storageClasses)
}

type storageClass struct {
	Name          string `json:"name"`
	StoragePolicy string `json:"storagePolicy"`
}
