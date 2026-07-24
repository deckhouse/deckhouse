/*
Copyright 2026 Flant JSC

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

package hooks

import (
	"context"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

/*
Description:
	Resolves the operator-declared desired Kubernetes version (ModuleConfig control-plane-manager
	kubernetesVersion, falling back to the deprecated ClusterConfiguration field) and writes it
	into spec.desiredVersion / spec.updateMode of the kube-system/d8-cluster-kubernetes ConfigMap.

	The update-observer controller (running inside the control-plane-manager DaemonSet) treats
	that spec block as external input: it reconciles status.* against it, but never computes
	spec.* itself. This hook is the single writer of spec.* on that ConfigMap — it never touches
	status.* or the k8s-version/max-k8s-version labels update-observer owns, and it never writes
	anything into the d8-cluster-configuration Secret (only reads it, to recover the raw/
	unresolved ClusterConfiguration value needed for the updateMode decision below).

	Soft validation: in Automatic mode (no explicit kubernetesVersion pinned anywhere), never
	silently resolve to a version more than 1 minor below what's actually running — that would be
	an unattended, unconfirmed downgrade of the control plane. Skip the write and raise
	D8ControlPlaneDefaultVersionDrift instead; the operator can downgrade explicitly if that's
	really wanted (hard validation then guards that explicit downgrade the usual way).
*/

const (
	desiredVersionConfigMapName      = "d8-cluster-kubernetes"
	desiredVersionConfigMapNamespace = "kube-system"

	defaultVersionDriftMetricGroup = "D8ControlPlaneDefaultVersionDrift"
	defaultVersionDriftMetricName  = "d8_control_plane_default_version_drift"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        moduleQueue,
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 15},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "cluster_configuration_secret",
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-cluster-configuration"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			FilterFunc: sdkvFilterRawClusterConfigurationVersion,
		},
		{
			Name:       "cluster_kubernetes_configmap",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NameSelector: &types.NameSelector{
				MatchNames: []string{desiredVersionConfigMapName},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{desiredVersionConfigMapNamespace},
				},
			},
			FilterFunc: sdkvFilterConfigMapSnapshot,
		},
	},
}, syncDesiredKubernetesVersion)

// rawClusterConfiguration captures only the raw (possibly literal "Automatic") kubernetesVersion
// field from the embedded ClusterConfiguration YAML — unlike
// global.clusterConfiguration.kubernetesVersion in Values, which the global discovery hook has
// already resolved to a concrete version by the time module hooks run.
type rawClusterConfiguration struct {
	KubernetesVersion string `json:"kubernetesVersion"`
}

func sdkvFilterRawClusterConfigurationVersion(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret corev1.Secret
	if err := sdk.FromUnstructured(unstructured, &secret); err != nil {
		return nil, err
	}

	var cc rawClusterConfiguration
	if err := yaml.Unmarshal(secret.Data["cluster-configuration.yaml"], &cc); err != nil {
		// Malformed/absent ClusterConfiguration is unrelated to this hook's job — ignore.
		return "", nil
	}

	return cc.KubernetesVersion, nil
}

// configMapSnapshot carries both the ConfigMap's current data["spec"] (raw YAML, used to detect
// a no-op write) and its data["status"].currentVersion (the actually-running version, as last
// confirmed by update-observer — used for the drift check below). Both are read-only inputs to
// this hook; only "spec" is ever written back.
type configMapSnapshot struct {
	Spec           string
	CurrentVersion string
}

func sdkvFilterConfigMapSnapshot(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var cm corev1.ConfigMap
	if err := sdk.FromUnstructured(unstructured, &cm); err != nil {
		return nil, err
	}

	snapshot := configMapSnapshot{Spec: cm.Data["spec"]}

	var status struct {
		CurrentVersion string `json:"currentVersion"`
	}
	if err := yaml.Unmarshal([]byte(cm.Data["status"]), &status); err == nil {
		snapshot.CurrentVersion = status.CurrentVersion
	}

	return snapshot, nil
}

// configMapSpec mirrors the Spec struct update-observer's controller/configmap.go writes/reads
// for the ConfigMap's "spec" data key.
type configMapSpec struct {
	DesiredVersion string `json:"desiredVersion"`
	UpdateMode     string `json:"updateMode"`
}

func syncDesiredKubernetesVersion(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(defaultVersionDriftMetricGroup)

	desired := resolveDeclaredKubernetesVersion(input)
	if desired == "" {
		// Nothing declared anywhere yet (e.g. the very first reconcile before
		// global.clusterConfiguration is populated) — nothing to write.
		return nil
	}

	mcVersion := input.Values.Get("controlPlaneManager.kubernetesVersion").String()

	ccRaw := ""
	if snaps, err := sdkobjectpatch.UnmarshalToStruct[string](input.Snapshots, "cluster_configuration_secret"); err == nil && len(snaps) > 0 {
		ccRaw = snaps[0]
	}

	// The declared source is whichever of MC/CC is authoritative (MC wins if set at all, even to
	// the literal "Automatic"); updateMode reflects whether that source is untracked ("Automatic"
	// or unset) or an explicit pin.
	declaredSource := mcVersion
	if declaredSource == "" {
		declaredSource = ccRaw
	}
	updateMode := "Manual"
	if declaredSource == "" || declaredSource == "Automatic" {
		updateMode = "Automatic"
	}

	var cm configMapSnapshot
	if snaps, err := sdkobjectpatch.UnmarshalToStruct[configMapSnapshot](input.Snapshots, "cluster_kubernetes_configmap"); err == nil && len(snaps) > 0 {
		cm = snaps[0]
	}

	if updateMode == "Automatic" && cm.CurrentVersion != "" {
		if drifted, err := isMoreThanOneMinorBelow(desired, cm.CurrentVersion); err == nil && drifted {
			input.MetricsCollector.Set(defaultVersionDriftMetricName, 1, map[string]string{}, metrics.WithGroup(defaultVersionDriftMetricGroup))
			return nil
		}
	}

	specBytes, err := yaml.Marshal(configMapSpec{DesiredVersion: desired, UpdateMode: updateMode})
	if err != nil {
		return err
	}
	specYAML := string(specBytes)

	if cm.Spec == specYAML {
		return nil
	}

	// ConfigMap may not exist yet (e.g. before update-observer's DaemonSet pods have ever run) —
	// create it with just this one data key, then merge-patch the same key in case it already
	// existed with other keys/labels (owned by update-observer) that must not be touched.
	input.PatchCollector.CreateIfNotExists(&corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      desiredVersionConfigMapName,
			Namespace: desiredVersionConfigMapNamespace,
			Labels:    map[string]string{"heritage": "deckhouse"},
		},
		Data: map[string]string{"spec": specYAML},
	})

	patch := map[string]interface{}{
		"data": map[string]interface{}{
			"spec": specYAML,
		},
	}
	input.PatchCollector.PatchWithMerge(patch, "v1", "ConfigMap", desiredVersionConfigMapNamespace, desiredVersionConfigMapName, object_patch.WithIgnoreMissingObject())

	return nil
}

// isMoreThanOneMinorBelow reports whether candidate is more than 1 minor version below current.
func isMoreThanOneMinorBelow(candidate, current string) (bool, error) {
	candidateV, err := semver.NewVersion(candidate)
	if err != nil {
		return false, err
	}
	currentV, err := semver.NewVersion(current)
	if err != nil {
		return false, err
	}
	if candidateV.Major() != currentV.Major() {
		return candidateV.Major() < currentV.Major(), nil
	}
	return candidateV.Minor() < currentV.Minor()-1, nil
}
