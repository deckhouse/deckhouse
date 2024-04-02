/*
Copyright 2021 Flant JSC

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
	"encoding/base64"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

/*
Description:
	Hook generates 3 kind of snapshots:
		- control-plane pods with annotation "control-plane-manager.deckhouse.io/kubernetes-version" from kube-system NS
		- all Nodes (filtering .status.modeInfo.kubeletVersion)
		- Secret: d8-cluster-configuration from NS: kube-system - get only maxUsedControlPlaneKubernetesVersion field
	and get desired k8s version from `global.clusterConfiguration.kubernetesVersion`

	Then process following logic:
	```
	if global.clusterConfiguration.kubernetesVersion > maxNodeVersion
		if minNodeVersion < minControlPlaneVersion:
			effectiveKubernetesVersion = minControlPlaneVersion
		else:
			effectiveKubernetesVersion =  minControlPlaneVersion.IncMinor() // bumped minor version
	else if global.clusterConfiguration.kubernetesVersion < maxNodeVersion:
		if maxNodeVersion < maxControlPlaneVersion && maxControlPlaneVersion == maxUsedControlPlaneVersion:
			unbumped := fmt.Sprintf("%d.%d.%d", maxControlPlaneVersion.Major(), maxControlPlaneVersion.Minor()-1, maxControlPlaneVersion.Patch())
			effectiveKubernetesVersion = semver.MustParse(unbumped) // minor version-1
		else:
			effectiveKubernetesVersion = maxControlPlaneVersion
	else:
		effectiveKubernetesVersion = global.clusterConfiguration.kubernetesVersion
	```

	then save effectiveKubernetesVersion to Values (`global.clusterConfiguration.kubernetesVersion`)
	and if effectiveKubernetesVersion >= maxUsedControlPlaneVersion:
		update maxUsedControlPlaneKubernetesVersion in Secret: d8-cluster-configuration

     For deckhouse upgrade requirements we are using minimal version of whole cluster.
*/

const minK8sVersionRequirementKey = "controlPlaneManager:minUsedControlPlaneKubernetesVersion"

const maxUsedK8sVersionSecretKey = "maxUsedControlPlaneKubernetesVersion"
const deckhouseDefaultK8sVersionSecretKey = "deckhouseDefaultKubernetesVersion"

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        moduleQueue,
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 50},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "control_plane_versions",
			ApiVersion: "v1",
			Kind:       "Pod",
			FilterFunc: ekvFilterControlPlaneAnnotations,
			LabelSelector: &v1.LabelSelector{
				MatchExpressions: []v1.LabelSelectorRequirement{
					{
						Key:      "component",
						Operator: v1.LabelSelectorOpIn,
						Values:   []string{"kube-apiserver", "kube-controller-manager", "kube-scheduler"},
					},
					{
						Key:      "tier",
						Operator: v1.LabelSelectorOpIn,
						Values:   []string{"control-plane"},
					},
				},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
		},
		{
			Name:       "node_versions",
			ApiVersion: "v1",
			Kind:       "Node",
			FilterFunc: ekvFilterNode,
		},
		{
			Name:       "max_used_control_plane_version",
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
			FilterFunc: ekvFilterSecret,
		},
	},
}, dependency.WithExternalDependencies(handleEffectiveK8sVersion))

type controlPlanePod struct {
	Name       string
	K8sVersion string
}

func ekvFilterControlPlaneAnnotations(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var pod corev1.Pod
	err := sdk.FromUnstructured(unstructured, &pod)
	if err != nil {
		return nil, err
	}
	annotations := pod.GetAnnotations()

	k8sVersion, ok := annotations["control-plane-manager.deckhouse.io/kubernetes-version"]
	if !ok {
		return nil, nil
	}

	return controlPlanePod{Name: pod.Name, K8sVersion: k8sVersion}, nil
}

func ekvFilterNode(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node corev1.Node

	err := sdk.FromUnstructured(unstructured, &node)
	if err != nil {
		return nil, err
	}

	rawV := node.Status.NodeInfo.KubeletVersion
	rawV = strings.TrimPrefix(rawV, "v")

	return semver.NewVersion(rawV)
}

type kubernetesVersionsInSecret struct {
	MaxUsed          *semver.Version
	DeckhouseDefault *semver.Version
}

func ekvFilterSecret(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret corev1.Secret

	err := sdk.FromUnstructured(unstructured, &secret)
	if err != nil {
		return nil, err
	}

	versions := kubernetesVersionsInSecret{}

	rawMaxUsed, ok := secret.Data[maxUsedK8sVersionSecretKey]
	if ok {
		maxUsed, err := semver.NewVersion(string(rawMaxUsed))
		if err != nil {
			return nil, err
		}
		versions.MaxUsed = maxUsed
	}

	rawDeckhouseDefault, ok := secret.Data[deckhouseDefaultK8sVersionSecretKey]
	if ok {
		deckhouseDefault, err := semver.NewVersion(string(rawDeckhouseDefault))
		if err != nil {
			return nil, err
		}
		versions.DeckhouseDefault = deckhouseDefault
	}

	return versions, nil
}

func handleEffectiveK8sVersion(input *go_hook.HookInput, dc dependency.Container) error {
	prevEffectiveVersion := input.Values.Get("controlPlaneManager.internal.effectiveKubernetesVersion").String()

	configVersionRaw, ok := input.Values.GetOk("global.clusterConfiguration.kubernetesVersion")
	if !ok {
		return fmt.Errorf("global.clusterConfiguration.kubernetesVersion required")
	}

	configVersion, err := semver.NewVersion(configVersionRaw.String())
	if err != nil {
		return fmt.Errorf("global.clusterConfiguration.kubernetesVersion is not valid semver: %s", configVersionRaw.String())
	}

	// process pods snapshot
	minControlPlaneVersion, maxControlPlaneVersion, err := ekvProcessPodsSnapshot(input, dc)
	if err != nil {
		return err
	}

	// process nodes snapshot
	minNodeVersion, maxNodeVersion, err := ekvProcessNodeSnapshot(input)
	if err != nil {
		return err
	}

	requirements.SaveValue(minK8sVersionRequirementKey, minNodeVersion.String())

	// process secret snapshot
	versionsInSecret := ekvProcessSecretSnapshot(input)
	maxUsedControlPlaneVersion := versionsInSecret.MaxUsed
	if maxUsedControlPlaneVersion == nil {
		input.LogEntry.Warn("deckhouse-managed control plane Pods are not yet deployed, setting max_used_control_plane_version to config_version")
		maxUsedControlPlaneVersion = configVersion
	}

	var effectiveKubernetesVersion *semver.Version

	// getting version logic here
	switch {
	case configVersion.GreaterThan(maxNodeVersion):
		if minNodeVersion.LessThan(minControlPlaneVersion) {
			effectiveKubernetesVersion = minControlPlaneVersion
		} else {
			bumped := minControlPlaneVersion.IncMinor()
			effectiveKubernetesVersion = &bumped
		}

	case configVersion.LessThan(maxNodeVersion):
		if maxNodeVersion.LessThan(maxControlPlaneVersion) && maxControlPlaneVersion.Equal(maxUsedControlPlaneVersion) {
			unbumped := fmt.Sprintf("%d.%d.%d", maxControlPlaneVersion.Major(), maxControlPlaneVersion.Minor()-1, maxControlPlaneVersion.Patch())
			effectiveKubernetesVersion = semver.MustParse(unbumped)
		} else {
			effectiveKubernetesVersion = maxControlPlaneVersion
		}

	default:
		effectiveKubernetesVersion = configVersion
	}

	// result semver should me in form X.Y, not X.Y.Z
	resultStr := fmt.Sprintf("%d.%d", effectiveKubernetesVersion.Major(), effectiveKubernetesVersion.Minor())

	input.Values.Set("controlPlaneManager.internal.effectiveKubernetesVersion", resultStr)
	input.MetricsCollector.Set("d8_kubernetes_version", 1, map[string]string{"k8s_version": resultStr})

	var patch map[string]interface{}

	addToPatch := func(key, value string) {
		if patch == nil {
			patch = map[string]interface{}{
				"data": map[string]interface{}{},
			}
		}

		data := patch["data"].(map[string]interface{})
		data[key] = value
	}

	if !effectiveKubernetesVersion.LessThan(maxUsedControlPlaneVersion) {
		encoded := base64.StdEncoding.EncodeToString([]byte(resultStr))
		addToPatch(maxUsedK8sVersionSecretKey, encoded)
	}

	currentDeckhouseDefault, err := semver.NewVersion(config.DefaultKubernetesVersion)
	if err != nil {
		return fmt.Errorf("incorrect default kubernetes version %s: %v", config.DefaultKubernetesVersion, err)
	}

	if versionsInSecret.DeckhouseDefault == nil || currentDeckhouseDefault.GreaterThan(versionsInSecret.DeckhouseDefault) {
		resultStr := fmt.Sprintf("%d.%d", currentDeckhouseDefault.Major(), currentDeckhouseDefault.Minor())
		encoded := base64.StdEncoding.EncodeToString([]byte(resultStr))
		addToPatch(deckhouseDefaultK8sVersionSecretKey, encoded)
	}

	if patch != nil {
		input.PatchCollector.MergePatch(patch, "v1", "Secret", "kube-system", "d8-cluster-configuration")
	}

	if prevEffectiveVersion != "" && prevEffectiveVersion != resultStr {
		_ = os.RemoveAll("~/.kube/http-cache")
		_ = os.RemoveAll("~/.kube/cache")
	}

	return nil
}

// process control plane pods snapshot with annotation, gettings minimum and maximum from control-plane-pods
func ekvProcessPodsSnapshot(input *go_hook.HookInput, dc dependency.Container) (minControlPlaneVersion, maxControlPlaneVersion *semver.Version, err error) {
	snap := input.Snapshots["control_plane_versions"]

	var controlPlaneVersions []controlPlanePod
	var apiserverExists bool

	for _, res := range snap {
		if res == nil {
			continue // filtered pod could be nil, if it doesnt have necessary annotation
		}
		pod := res.(controlPlanePod)
		if strings.Contains(pod.Name, "kube-apiserver") {
			apiserverExists = true
		}
		controlPlaneVersions = append(controlPlaneVersions, pod)
	}

	if !apiserverExists {
		input.LogEntry.Warn("deckhouse-managed control plane Pods are not yet deployed, setting control_plane_version to version acquired from kubectl version")
		k8sClient, err := dc.GetK8sClient()
		if err != nil {
			return nil, nil, err
		}
		verInfo, err := k8sClient.Discovery().ServerVersion()
		if err != nil {
			return nil, nil, err
		}
		controlPlaneVersions = []controlPlanePod{{
			Name:       "server_discovery",
			K8sVersion: fmt.Sprintf("%s.%s.0", verInfo.Major, verInfo.Minor),
		}}
	}

	controlPlaneVs := make([]*semver.Version, len(controlPlaneVersions))
	for i, pod := range controlPlaneVersions {
		v, err := semver.NewVersion(pod.K8sVersion)
		if err != nil {
			return nil, nil, fmt.Errorf("control_plane_version: %s has invalid semver: %s", pod.Name, pod.K8sVersion)
		}
		controlPlaneVs[i] = v
	}
	sort.Sort(semver.Collection(controlPlaneVs))

	minControlPlaneVersion = controlPlaneVs[0]
	maxControlPlaneVersion = controlPlaneVs[len(controlPlaneVs)-1]

	return
}

// determine minimum and maximum node versions
func ekvProcessNodeSnapshot(input *go_hook.HookInput) (minNodeVersion, maxNodeVersion *semver.Version, err error) {
	snap := input.Snapshots["node_versions"]

	var nodeVersions []*semver.Version
	for _, res := range snap {
		nodeVersions = append(nodeVersions, res.(*semver.Version))
	}

	if len(nodeVersions) == 0 {
		return nil, nil, fmt.Errorf("no Nodes? What are you doing here")
	}
	sort.Sort(semver.Collection(nodeVersions))

	minNodeVersion = nodeVersions[0]
	maxNodeVersion = nodeVersions[len(nodeVersions)-1]

	return
}

// get semver from secret
func ekvProcessSecretSnapshot(input *go_hook.HookInput) (maxUsedControlPlaneVersion kubernetesVersionsInSecret) {
	snap := input.Snapshots["max_used_control_plane_version"]

	if len(snap) > 0 && snap[0] != nil {
		return snap[0].(kubernetesVersionsInSecret)
	}

	return kubernetesVersionsInSecret{}
}
