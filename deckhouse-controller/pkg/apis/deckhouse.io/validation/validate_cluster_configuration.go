/*
Copyright 2024 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package validation

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	kwhhttp "github.com/slok/kubewebhook/v2/pkg/http"
	"github.com/slok/kubewebhook/v2/pkg/model"
	kwhvalidating "github.com/slok/kubewebhook/v2/pkg/webhook/validating"
	v1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders/kubernetesversion"
	"github.com/deckhouse/deckhouse/modules/040-control-plane-manager/hooks"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	containerdV2UnsupportedLabel        = "node.deckhouse.io/containerd-v2-unsupported"
	customContainerdConfigLabelSelector = "node.deckhouse.io/containerd-config=custom"
	nodeGroupNameLabel                  = "node.deckhouse.io/group"
)

type clusterConfig struct {
	APIVersion              string       `json:"apiVersion"`
	Kind                    string       `json:"kind"`
	ClusterType             string       `json:"clusterType"`
	KubernetesVersion       string       `json:"kubernetesVersion"`
	DefaultCRI              string       `json:"defaultCRI"`
	PodSubnetNodeCIDRPrefix string       `json:"podSubnetNodeCIDRPrefix"`
	PodSubnetCIDR           string       `json:"podSubnetCIDR"`
	ServiceSubnetCIDR       string       `json:"serviceSubnetCIDR"`
	ClusterDomain           string       `json:"clusterDomain"`
	EncryptionAlgorithm     string       `json:"encryptionAlgorithm"`
	Cloud                   *cloudConfig `json:"cloud,omitempty"`
}

type cloudConfig struct {
	Provider string `json:"provider"`
	Prefix   string `json:"prefix,omitempty"`
}

func validateKubernetesVersion(version string, mm moduleManager) (*kwhvalidating.ValidatorResult, error) {
	if version == "Automatic" {
		version = hooks.DefaultKubernetesVersion
	}

	if moduleName, err := kubernetesversion.Instance().ValidateBaseVersion(version); err != nil {
		log.Debug("failed to validate base version", log.Err(err))
		if moduleName == "" {
			return rejectResult(err.Error())
		}
		if mm.IsModuleEnabled(moduleName) {
			log.Debug("module has unsatisfied requirements", slog.String("name", moduleName))
			return rejectResult(err.Error())
		}
	}

	return allowResult(nil)
}

// listNodeGroupCRITypes returns a map of NodeGroup name -> spec.cri.type.
func listNodeGroupCRITypes(ctx context.Context, cli client.Client) (map[string]string, error) {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "deckhouse.io",
		Version: "v1",
		Kind:    "NodeGroupList",
	})
	if err := cli.List(ctx, list); err != nil {
		return nil, fmt.Errorf("list NodeGroups: %w", err)
	}
	out := make(map[string]string, len(list.Items))
	for i := range list.Items {
		t, _, _ := unstructured.NestedString(list.Items[i].Object, "spec", "cri", "type")
		out[list.Items[i].GetName()] = t
	}
	return out, nil
}

// nodeEffectivelyContainerdV2 reports whether the nodes effective CRI is (or will become) ContainerdV2.
// Returns true if the node has no NodeGroup label or the NodeGroup is not in the snapshot(just in case).
func nodeEffectivelyContainerdV2(node *v1.Node, ngCRIType map[string]string) bool {
	ng := node.Labels[nodeGroupNameLabel]
	if ng == "" {
		return true
	}
	t, ok := ngCRIType[ng]
	if !ok {
		return true
	}
	return t == "" || t == "ContainerdV2"
}

func formatNodeWithNG(node *v1.Node) string {
	return fmt.Sprintf("%s (NodeGroup=%s)", node.Name, node.Labels[nodeGroupNameLabel])
}

func checkCntrdV2Support(ctx context.Context, cli client.Client) (*kwhvalidating.ValidatorResult, error) {
	unsupportedSelector, err := labels.Parse(containerdV2UnsupportedLabel)
	if err != nil {
		return nil, fmt.Errorf("failed to parse label selector for unsupported nodes: %w", err)
	}

	unsupportedNodes := &v1.NodeList{}
	if err := cli.List(ctx, unsupportedNodes, &client.ListOptions{LabelSelector: unsupportedSelector}); err != nil {
		return nil, fmt.Errorf("failed to list nodes with label %q: %w", containerdV2UnsupportedLabel, err)
	}

	customConfigSelector, err := labels.Parse(customContainerdConfigLabelSelector)
	if err != nil {
		return nil, fmt.Errorf("failed to parse label selector for custom containerd config: %w", err)
	}

	customConfigNodes := &v1.NodeList{}
	if err := cli.List(ctx, customConfigNodes, &client.ListOptions{LabelSelector: customConfigSelector}); err != nil {
		return nil, fmt.Errorf("failed to list nodes with label %q: %w", customContainerdConfigLabelSelector, err)
	}

	// Nothing flagged any node - early exit before listing NodeGroups
	if len(unsupportedNodes.Items) == 0 && len(customConfigNodes.Items) == 0 {
		return allowResult(nil)
	}

	ngCRI, err := listNodeGroupCRITypes(ctx, cli)
	if err != nil {
		return nil, err
	}

	var blockedNodes []string
	for _, node := range unsupportedNodes.Items {
		if nodeEffectivelyContainerdV2(&node, ngCRI) {
			blockedNodes = append(blockedNodes, formatNodeWithNG(&node))
		}
	}
	if len(blockedNodes) > 0 {
		return rejectResult(fmt.Sprintf(
			"Cluster has nodes that don't support ContainerdV2 and would inherit cluster default: %s. "+
				"Pin their NodeGroups by setting spec.cri.type=Containerd, or resolve the incompatibility with ContainerdV2.",
			strings.Join(blockedNodes, ", ")))
	}

	blockedNodes = blockedNodes[:0]
	for _, node := range customConfigNodes.Items {
		if nodeEffectivelyContainerdV2(&node, ngCRI) {
			blockedNodes = append(blockedNodes, formatNodeWithNG(&node))
		}
	}
	if len(blockedNodes) > 0 {
		return rejectResult(fmt.Sprintf(
			"Cluster has nodes with a custom containerd config, which is incompatible with ContainerdV2 "+
				"that would inherit cluster default: %s. Pin their NodeGroups by setting "+
				"spec.cri.type=Containerd, or remove/migrate the custom config to ContainerdV2.",
			strings.Join(blockedNodes, ", ")))
	}

	return allowResult(nil)
}

func validateDefaultCRI(defaultCRI string, cli client.Client) (*kwhvalidating.ValidatorResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	switch defaultCRI {
	case "Containerd":
		return allowResult(nil)
	case "ContainerdV2":
		return checkCntrdV2Support(ctx, cli)
	case "NotManaged":
		return allowResult(nil)
	default:
		return rejectResult(fmt.Sprintf("Unsupported CRI: %s", defaultCRI))
	}
}

func getKubernetesEndpointsCount(ctx context.Context, cli client.Client) (int, error) {
	endpointslice := &discoveryv1.EndpointSlice{}
	err := cli.Get(ctx, client.ObjectKey{
		Namespace: "default",
		Name:      "kubernetes",
	}, endpointslice)
	if err != nil {
		return 0, fmt.Errorf("failed to get kubernetes endpointslice: %w", err)
	}

	count := 0
	for _, endpoints := range endpointslice.Endpoints {
		count += len(endpoints.Addresses)
	}
	return count, nil
}

func parseVersion(version string) (*semver.Version, error) {
	// Trim whitespace and newlines that might come from secret data
	version = strings.TrimSpace(version)
	if version == "" {
		return nil, fmt.Errorf("version string is empty")
	}
	return semver.NewVersion(version)
}

// kubernetesVersionBelowFloor reports whether target lands more than one minor below floor —
// the single "how far down may we go" rule, shared by the ClusterConfiguration downgrade check
// (minorSubCheck, where the floor is the previous/maxUsed version) and the ModuleConfig guard
// (rejectKubernetesVersionBelowMaxUsed, where the floor is maxUsed).
//
// Neutral (bool) rather than a ValidatorResult on purpose: the two webhooks use opposite result
// conventions — the ClusterConfiguration chain returns a non-nil allowResult for "allowed", while
// validateCommon treats (nil, nil) as "allowed" — so each caller wraps this in its own.
//
// The minor comparison is written as an addition on target so the uint64 minor never underflows
// on a 1.0-style version.
func kubernetesVersionBelowFloor(target, floor *semver.Version) bool {
	switch {
	case target.Major() > floor.Major():
		return false
	case target.Major() == floor.Major() && target.Minor()+1 >= floor.Minor():
		return false
	default:
		return true
	}
}

// TODO(kubernetesVersion-deprecation): T+1 remove — drop CC kubernetesVersion validation path; MC webhook remains the only guard.
// NOTE(kubernetesVersion-deprecation): keep — Secret maxUsed/default baseline keys survive CC field removal.
//
// kubernetesVersionBaseline carries the cluster-level version facts the downgrade check resolves
// "Automatic" against. Both values are Deckhouse's own bookkeeping (written by the
// control-plane-manager effective_kubernetes_version.go hook), not ClusterConfiguration fields.
type kubernetesVersionBaseline struct {
	// MaxUsed is the highest version the cluster has ever converged onto
	// (maxUsedControlPlaneKubernetesVersion).
	MaxUsed    string
	MaxUsedSet bool
	// DeckhouseDefault is the version "Automatic" currently resolves to
	// (deckhouseDefaultKubernetesVersion).
	DeckhouseDefault    string
	DeckhouseDefaultSet bool
	// AvailableVersions is status.availableVersions, the set update-observer publishes as
	// Supported[maxUsed-1:]. Only the ConfigMap ever carried it, so it stays empty when that object
	// is missing — there is no Secret key to fall back to. Carried on the baseline so the
	// ModuleConfig webhook resolves the floor and the membership list from one snapshot of one
	// object instead of reading it twice.
	AvailableVersions []string
}

// kubernetesVersionBaselineFromSecret reads the baseline out of the d8-cluster-configuration Secret.
//
// TODO(kubernetesVersion-deprecation): T+1 remove — the Secret keys are a migration fallback for
// kubernetesVersionBaselineFor below; nothing writes them any more.
func kubernetesVersionBaselineFromSecret(secret *v1.Secret) kubernetesVersionBaseline {
	if secret == nil {
		return kubernetesVersionBaseline{}
	}

	// A present-but-empty key counts as unset. The *Set flags gate a parseVersion call whose
	// failure is fatal to the whole webhook, so treating "" as a value would turn a blank key into
	// a fail-closed ClusterConfiguration — no edit of any field would be accepted.
	maxUsed := strings.TrimSpace(string(secret.Data["maxUsedControlPlaneKubernetesVersion"]))
	deckhouseDefault := strings.TrimSpace(string(secret.Data["deckhouseDefaultKubernetesVersion"]))
	return kubernetesVersionBaseline{
		MaxUsed:             maxUsed,
		MaxUsedSet:          maxUsed != "",
		DeckhouseDefault:    deckhouseDefault,
		DeckhouseDefaultSet: deckhouseDefault != "",
	}
}

// kubernetesVersionBaselineFor resolves both facts from the d8-cluster-kubernetes ConfigMap, which
// update-observer owns, and falls back to the d8-cluster-configuration Secret per field.
//
// The two facts come from different blocks because they are different kinds of thing:
// spec.maxUsedKubernetesVersion is a monotonic record of what the cluster has run, while
// status.automaticVersion is what the running Deckhouse build resolves "Default" to right now.
// Note that the Secret key it replaces was only ever raised, so after a Deckhouse downgrade the
// two disagree — and the ConfigMap is the one telling the truth about the current build.
func kubernetesVersionBaselineFor(ctx context.Context, cli client.Client, secret *v1.Secret) kubernetesVersionBaseline {
	baseline := kubernetesVersionBaselineFromSecret(secret)

	cm := new(v1.ConfigMap)
	if err := cli.Get(ctx, client.ObjectKey{
		Name:      clusterKubernetesConfigMapName,
		Namespace: kubeSystemNamespace,
	}, cm); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Warn("cannot read the d8-cluster-kubernetes ConfigMap, falling back to the Secret baseline", log.Err(err))
		}
		return baseline
	}

	// A block this webhook cannot read leaves the Secret-derived value in place, which is the right
	// degradation — but it is logged, because a guard that quietly falls back to a stale source
	// looks exactly like a guard that found nothing to correct.
	spec := new(clusterKubernetesSpec)
	if err := yaml.Unmarshal([]byte(cm.Data[clusterKubernetesSpecDataKey]), spec); err != nil {
		log.Warn("cannot parse d8-cluster-kubernetes data.spec, keeping the Secret baseline", log.Err(err))
	} else if maxUsed := strings.TrimSpace(spec.MaxUsedVersion); maxUsed != "" {
		baseline.MaxUsed, baseline.MaxUsedSet = maxUsed, true
	}

	status := new(clusterKubernetesStatus)
	if err := yaml.Unmarshal([]byte(cm.Data[clusterKubernetesStatusDataKey]), status); err != nil {
		log.Warn("cannot parse d8-cluster-kubernetes data.status, keeping the Secret baseline", log.Err(err))
	} else {
		if automaticVersion := strings.TrimSpace(status.AutomaticVersion); automaticVersion != "" {
			baseline.DeckhouseDefault, baseline.DeckhouseDefaultSet = automaticVersion, true
		}
		baseline.AvailableVersions = status.AvailableVersions
	}

	return baseline
}

// validateKubernetesVersionDowngrade validates that Kubernetes version downgrade
// does not exceed 1 minor version. It handles "Automatic" version by resolving
// it to an actual version from the supplied baseline.
//
// Rules:
//   - Upgrade is always allowed (no restrictions)
//   - Downgrade is allowed only if it's within 1 minor version
//   - Multiple downgrades are dissalowed. If maxUsedControlPlaneKubernetesVersion > oldVersion
//     use maxUsedControlPlaneKubernetesVersion instead of oldVersion
//   - When oldVersion is "Automatic", uses baseline.MaxUsed
//     (maximum version that was ever used in the cluster)
//   - When newVersion is "Automatic", uses baseline.DeckhouseDefault
//     (default version that Deckhouse will use for Automatic)
//   - Also checks maxUsedControlPlaneKubernetesVersion to prevent downgrade below max used version
func validateKubernetesVersionDowngrade(oldVersion, newVersion string, baseline kubernetesVersionBaseline) (*kwhvalidating.ValidatorResult, error) {
	// oldVersion can be either "Automatic" or semver (e.g., "1.23.4")
	// newVersion can be either "Automatic" or semver (e.g., "1.23.5")
	//
	// kubernetesVersion is optional in ClusterConfiguration since it moved to the
	// control-plane-manager ModuleConfig, so either side can now be absent. An absent field means
	// exactly what "Automatic" means — Deckhouse picks the version — so normalize instead of
	// handing "" to the semver parser.
	if oldVersion == "" {
		oldVersion = automaticKubernetesVersion
	}
	if newVersion == "" {
		newVersion = automaticKubernetesVersion
	}

	if oldVersion == newVersion {
		return allowResult(nil)
	}

	type versionChecker func(oldVersionSemver, newVersionSemver *semver.Version) (*kwhvalidating.ValidatorResult, error)
	var selectedChecker versionChecker

	var nameForOldVersion = "oldKubernetesVersion"
	// minorSubCheck validates that downgrade does not exceed 1 minor version.
	// It allows upgrade without restrictions and only checks downgrade scenarios.
	var minorSubCheck = func(oldVersionSemver, newVersionSemver *semver.Version) (*kwhvalidating.ValidatorResult, error) {
		if !kubernetesVersionBelowFloor(newVersionSemver, oldVersionSemver) {
			return allowResult(nil)
		}

		return rejectResult(
			fmt.Sprintf("can not downgrade kubernetes version more than 1 minor version. %s=%s newKubernetesVersion=%s", nameForOldVersion, oldVersionSemver, newVersionSemver),
		)
	}

	// automaticOnlyGreaterCheck is used when newVersion is "Automatic".
	// It only rejects if oldVersion is greater than Automatic version (downgrade scenario).
	// Upgrade or same version is allowed.
	// This is simpler than minorSubCheck because Automatic will use deckhouseDefaultKubernetesVersion
	// which is always safe, so we only need to check if it's a downgrade.
	var automaticOnlyGreaterCheck = func(oldVersionSemver, newVersionSemver *semver.Version) (*kwhvalidating.ValidatorResult, error) {
		if oldVersionSemver.GreaterThan(newVersionSemver) {
			return rejectResult(
				fmt.Sprintf(
					"can not set Automatic because it will downgrade kubernetes version. "+
						"Automatic=%s oldKubernetesVersion=%s", newVersionSemver, oldVersionSemver,
				),
			)
		}

		return allowResult(nil)
	}

	selectedChecker = minorSubCheck

	var maxUsedVersionSemver *semver.Version
	if baseline.MaxUsedSet {
		var err error
		maxUsedVersionSemver, err = parseVersion(baseline.MaxUsed)

		if err != nil {
			return nil, fmt.Errorf("failed to parse max used version: %w", err)
		}
	}

	// Resolve oldVersion: if it's "Automatic", get an actual version from the baseline
	var oldVersionSemver *semver.Version
	if oldVersion == "Automatic" {
		// Corner case: If maxUsedControlPlaneKubernetesVersion is not available,
		// we cannot determine the actual version that was used, so we allow the change.
		// This can happen during initial cluster setup or if a secret is incomplete.
		if maxUsedVersionSemver == nil {
			return allowResult(nil)
		}

		oldVersionSemver = maxUsedVersionSemver
	} else {
		var err error
		oldVersionSemver, err = parseVersion(oldVersion)
		if err != nil {
			return nil, fmt.Errorf("failed to parse old version: %w", err)
		}
	}

	// Resolve newVersion: if it's "Automatic", get actual version from the baseline
	var newVersionSemver *semver.Version
	if newVersion == "Automatic" {
		// Corner case: If deckhouseDefaultKubernetesVersion is not available,
		// we cannot determine what Automatic will resolve to, so we allow the change.
		// This can happen during initial cluster setup or if the source is incomplete.
		if !baseline.DeckhouseDefaultSet {
			return allowResult(nil)
		}

		var err error
		newVersionSemver, err = parseVersion(baseline.DeckhouseDefault)
		if err != nil {
			return nil, fmt.Errorf("failed to parse automatic version: %w", err)
		}

		// When newVersion is "Automatic", we use simpler checker that only checks
		// if oldVersion > newVersion (downgrade). Upgrade or same version is allowed.
		// We don't need to check minor version restriction because Automatic will use
		// deckhouseDefaultKubernetesVersion which is always safe.
		selectedChecker = automaticOnlyGreaterCheck
	} else {
		var err error
		newVersionSemver, err = parseVersion(newVersion)
		if err != nil {
			return nil, fmt.Errorf("failed to parse new version: %w", err)
		}

		// Switching to an explicit version: guard against downgrading more than
		// 1 minor below the highest version the cluster ever ran. This baseline only
		// makes sense for an explicit target — a switch to "Automatic" is compared
		// against the actual current version, so a no-op like "1.33" -> Automatic(=1.33)
		// stays allowed.
		if maxUsedVersionSemver != nil && maxUsedVersionSemver.GreaterThan(oldVersionSemver) {
			nameForOldVersion = "maxUsedControlPlaneKubernetesVersion"
			oldVersionSemver = maxUsedVersionSemver
		}
	}

	// Run selected checker
	result, err := selectedChecker(oldVersionSemver, newVersionSemver)
	if err != nil {
		return nil, err
	}
	if !result.Valid {
		return result, nil
	}

	return allowResult(nil)
}

func validateCRIChange(oldCRI, newCRI string, cli client.Client) (*kwhvalidating.ValidatorResult, error) {
	if oldCRI == newCRI {
		return allowResult(nil)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	endpointsCount, err := getKubernetesEndpointsCount(ctx, cli)
	if err != nil {
		return nil, fmt.Errorf("failed to get endpoints count: %w", err)
	}

	if endpointsCount < 3 {
		return allowResult([]string{"it is disruptive to change defaultCRI type for cluster with apiserver endpoints < 3"})
	}

	return allowResult(nil)
}

func validateUnsafeConfigChanges(oldConfig, newConfig *clusterConfig, unsafeMode bool) (*kwhvalidating.ValidatorResult, error) {
	if unsafeMode {
		return allowResult(nil)
	}

	if oldConfig.PodSubnetNodeCIDRPrefix != newConfig.PodSubnetNodeCIDRPrefix {
		return rejectResult("it is forbidden to change podSubnetNodeCIDRPrefix in a running cluster")
	}

	if oldConfig.PodSubnetCIDR != newConfig.PodSubnetCIDR {
		return rejectResult("it is forbidden to change podSubnetCIDR in a running cluster")
	}

	if oldConfig.ServiceSubnetCIDR != newConfig.ServiceSubnetCIDR {
		return rejectResult("it is forbidden to change serviceSubnetCIDR in a running cluster")
	}

	return allowResult(nil)
}

func validateClusterConfiguration(ctx context.Context, clusterConfiguration []byte) (*kwhvalidating.ValidatorResult, error) {
	_, err := config.ParseConfigFromData(
		ctx,
		string(clusterConfiguration),
		config.DummyValidatorProvider(),
		nil,
		config.ValidateOptionOmitDocInError(true),
		config.ValidateOptionStrictUnmarshal(true),
	)
	if err != nil {
		result, _ := rejectResult(err.Error())
		return result, nil
	}

	result, _ := allowResult(nil)
	return result, nil
}

func clusterConfigurationHandler(mm moduleManager, cli client.Client, _ *config.SchemaStore) http.Handler {
	validator := kwhvalidating.ValidatorFunc(func(ctx context.Context, ar *model.AdmissionReview, obj metav1.Object) (*kwhvalidating.ValidatorResult, error) {
		if ar.Operation == model.OperationDelete {
			return rejectResult("It is forbidden to delete secret d8-cluster-configuration")
		}

		secret, ok := obj.(*v1.Secret)
		if !ok {
			log.Debug("unexpected type", log.Type("expected", v1.Secret{}), log.Type("got", obj))
			return nil, fmt.Errorf("expect Secret as unstructured, got %T", obj)
		}

		clusterConfigurationRaw, ok := secret.Data["cluster-configuration.yaml"]
		if !ok {
			log.Debug("no cluster-configuration found in secret", slog.String("namespace", obj.GetNamespace()), slog.String("name", obj.GetName()))
			return nil, fmt.Errorf("expected field 'cluster-configuration.yaml' not found in secret %s", secret.Name)
		}

		clusterConfigurationValidator := kwhvalidating.ValidatorFunc(func(ctx context.Context, _ *model.AdmissionReview, _ metav1.Object) (*kwhvalidating.ValidatorResult, error) {
			return validateClusterConfiguration(ctx, clusterConfigurationRaw)
		})

		clusterConf := new(clusterConfig)
		if err := yaml.Unmarshal(clusterConfigurationRaw, clusterConf); err != nil {
			log.Debug("failed to unmarshal cluster configuration", log.Err(err))
			return nil, fmt.Errorf("unmarshal cluster configuration: %w", err)
		}

		k8sVersionValidator := kwhvalidating.ValidatorFunc(func(ctx context.Context, _ *model.AdmissionReview, _ metav1.Object) (*kwhvalidating.ValidatorResult, error) {
			// Not a pin: in *this* document the sentinel is "Automatic", and an absent field means
			// the same thing — Deckhouse picks the version. ModuleConfig spells that "Default"
			// and never accepts "Automatic"; the two dictionaries are not interchangeable.
			if !isClusterConfigurationPinned(clusterConf.KubernetesVersion) {
				return allowResult(nil)
			}
			// ModuleConfig supersedes this field; a leftover pin must not gate the write.
			if moduleConfigOwnsKubernetesVersion(ctx, cli) {
				return allowResult(nil)
			}
			return validateKubernetesVersion(clusterConf.KubernetesVersion, mm)
		})

		criValidator := kwhvalidating.ValidatorFunc(func(_ context.Context, _ *model.AdmissionReview, _ metav1.Object) (*kwhvalidating.ValidatorResult, error) {
			return validateDefaultCRI(clusterConf.DefaultCRI, cli)
		})

		validators := []kwhvalidating.Validator{clusterConfigurationValidator, k8sVersionValidator, criValidator}

		if ar.Operation == model.OperationUpdate && ar.OldObjectRaw != nil {
			oldSecret := &v1.Secret{}
			if err := yaml.Unmarshal(ar.OldObjectRaw, oldSecret); err == nil {
				if oldClusterConfigurationRaw, ok := oldSecret.Data["cluster-configuration.yaml"]; ok {
					oldClusterConf := new(clusterConfig)
					if err := yaml.Unmarshal(oldClusterConfigurationRaw, oldClusterConf); err == nil {
						unsafeMode := false
						if annotations := secret.GetAnnotations(); annotations != nil {
							if annotations["deckhouse.io/allow-unsafe"] != "" && annotations["deckhouse.io/allow-unsafe"] != "null" {
								unsafeMode = true
							}
						}

						unsafeValidator := kwhvalidating.ValidatorFunc(func(_ context.Context, _ *model.AdmissionReview, _ metav1.Object) (*kwhvalidating.ValidatorResult, error) {
							return validateUnsafeConfigChanges(oldClusterConf, clusterConf, unsafeMode)
						})

						k8sDowngradeValidator := kwhvalidating.ValidatorFunc(func(ctx context.Context, _ *model.AdmissionReview, _ metav1.Object) (*kwhvalidating.ValidatorResult, error) {
							// See k8sVersionValidator: when ModuleConfig owns the version this
							// field is inert, so changing (or removing) it cannot downgrade
							// anything and must not be blocked.
							if moduleConfigOwnsKubernetesVersion(ctx, cli) {
								return allowResult(nil)
							}
							return validateKubernetesVersionDowngrade(oldClusterConf.KubernetesVersion, clusterConf.KubernetesVersion, kubernetesVersionBaselineFor(ctx, cli, secret))
						})

						criChangeValidator := kwhvalidating.ValidatorFunc(func(_ context.Context, _ *model.AdmissionReview, _ metav1.Object) (*kwhvalidating.ValidatorResult, error) {
							oldCRI := oldClusterConf.DefaultCRI
							if oldCRI == "" {
								oldCRI = "Containerd"
							}
							newCRI := clusterConf.DefaultCRI
							if newCRI == "" {
								newCRI = "Containerd"
							}
							return validateCRIChange(oldCRI, newCRI, cli)
						})

						validators = append(validators, unsafeValidator, k8sDowngradeValidator, criChangeValidator)
					}
				}
			}
		}

		chain := kwhvalidating.NewChain(nil, validators...)
		return chain.Validate(ctx, ar, obj)
	})

	wh, _ := kwhvalidating.NewWebhook(kwhvalidating.WebhookConfig{
		ID:        "cluster-configuration-validator",
		Validator: validator,
		Logger:    nil,
		Obj:       &v1.Secret{},
	})

	return kwhhttp.MustHandlerFor(kwhhttp.HandlerConfig{Webhook: wh, Logger: nil})
}
