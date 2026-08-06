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

package controller

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"go.yaml.in/yaml/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"control-plane-manager/internal/controllers/update-observer/cluster"
	"control-plane-manager/internal/controllers/update-observer/common"
	"control-plane-manager/internal/controllers/update-observer/pkg/version"
)

type ConfigMapData struct {
	*Spec
	*Status
}

type Spec struct {
	DesiredVersion string `yaml:"desiredVersion"`
	UpdateMode     string `yaml:"updateMode"`
	// MaxUsedVersion is the highest Kubernetes minor this cluster has ever run. It is monotonic
	// and derived from the throttled effective version, never from DesiredVersion: an operator may
	// declare a version several minors ahead, and seeding the historical maximum from a declared
	// value would raise the downgrade floor to a version the cluster never actually ran.
	MaxUsedVersion string `yaml:"maxUsedKubernetesVersion,omitempty"`
}

type Status struct {
	CurrentVersion    string             `yaml:"currentVersion"`
	SupportedVersions []string           `yaml:"supportedVersions"`
	AvailableVersions []string           `yaml:"availableVersions"`
	AutomaticVersion  string             `yaml:"automaticVersion"`
	Phase             string             `yaml:"phase"`
	Progress          string             `yaml:"progress,omitempty"`
	ControlPlane      []ControlPlaneNode `yaml:"controlPlane"`
	Nodes             Nodes              `yaml:"nodes"`
}

type ControlPlaneNode struct {
	Name        string            `yaml:"name"`
	Phase       string            `yaml:"phase"`
	Description string            `yaml:"description,omitempty"`
	Components  map[string]string `yaml:"components"`
}

type Nodes struct {
	DesiredCount  int `yaml:"desiredCount"`
	UpToDateCount int `yaml:"upToDateCount"`
}

func (r *reconciler) getConfigMap(ctx context.Context) (*corev1.ConfigMap, error) {
	cm := &corev1.ConfigMap{}
	err := r.client.Get(ctx, client.ObjectKey{
		Name:      common.ConfigMapName,
		Namespace: common.KubeSystemNamespace,
	}, cm)

	if client.IgnoreNotFound(err) != nil {
		return nil, err
	}

	// NotFound is a real bootstrap path, not a failure: this controller authors every key of the
	// object, so it can synthesize the whole thing in memory and let touchConfigMap create it.
	// dhctl seeds the ConfigMap during bootstrap so that node-controller finds it from the moment
	// Deckhouse starts, but that seeding can be missed (dhctl deckhouse ... applies only the
	// Deployment) and the object can be deleted, and neither must leave the cluster without it.
	//
	// The empty ResourceVersion is what touchConfigMap keys off to Create rather than Update.
	if err != nil {
		cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:        common.ConfigMapName,
				Namespace:   common.KubeSystemNamespace,
				Annotations: map[string]string{},
				Labels:      identifyingLabels(),
			},
		}
	}

	return cm, nil
}

// identifyingLabels returns the labels that make the object recognizable to things that select it
// rather than name it. The name label is not decoration: the ValidatingWebhookConfiguration that
// forbids deleting this ConfigMap picks it out by objectSelector, and heritage: deckhouse sits on
// hundreds of objects so it cannot serve as that selector. Without the name label the delete
// protection either misses this object or, if widened, intercepts every ConfigMap in kube-system.
func identifyingLabels() map[string]string {
	return map[string]string{
		common.HeritageLabelKey: common.DeckhouseLabel,
		common.NameLabelKey:     common.ConfigMapName,
	}
}

func fillConfigMap(configMap *corev1.ConfigMap, clusterState *cluster.State, reconcileTrigger ReconcileTrigger) (*corev1.ConfigMap, error) {
	configMapData := renderConfigMapData(clusterState)
	configMap.Data = map[string]string{}

	// data.spec is rendered, not preserved. Copying the previous bytes through was what kept this
	// controller from colliding with a second writer; now that it is the only writer, the block is
	// simply authored from the container environment on every pass — which is also what lets a
	// hand edit of data.spec be corrected instead of pinned.
	if configMapData.Spec != nil {
		specBytes, err := yaml.Marshal(configMapData.Spec)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal Spec: %w", err)
		}
		configMap.Data["spec"] = string(specBytes)
	}

	if configMapData.Status != nil {
		statusBytes, err := yaml.Marshal(configMapData.Status)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal Status: %w", err)
		}
		configMap.Data["status"] = string(statusBytes)
	}

	// GetAnnotations/GetLabels can return nil for a ConfigMap this controller didn't create itself
	// (e.g. one seeded by dhctl during bootstrap, or a ConfigMap that came back from the API
	// server with an empty map omitted via `omitempty`) — guard against assigning into a nil map
	// below.
	annotations := configMap.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	labels := configMap.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	// Re-asserted on every pass, not only at creation: an object that lost the name label — an
	// older ConfigMap from before this release, or one edited by hand — would otherwise stay
	// outside the delete-protection webhook's objectSelector forever.
	for key, value := range identifyingLabels() {
		labels[key] = value
	}
	now := time.Now().UTC().Format(time.RFC3339)

	annotations[common.LastReconciliationTime] = now
	annotations[common.CauseLabelKey] = string(reconcileTrigger)

	switch reconcileTrigger {
	case ReconcileTriggerInit:
		labels[common.K8sVersionLabelKey] = clusterState.CurrentVersion
	case ReconcileTriggerUpgradeK8s:
		fallthrough
	case ReconcileTriggerDowngradeK8s:
		if clusterState.Phase == cluster.ClusterUpToDate {
			annotations[common.LastUpToDateTime] = now
			labels[common.K8sVersionLabelKey] = clusterState.CurrentVersion
		}
	case ReconcileTriggerIdle:
	}

	labels[common.MaxK8sVersionLabelKey] = version.GetMax(labels[common.MaxK8sVersionLabelKey], labels[common.K8sVersionLabelKey])

	configMap.SetAnnotations(annotations)
	configMap.SetLabels(labels)

	return configMap, nil
}

func renderConfigMapData(clusterState *cluster.State) ConfigMapData {
	renderControlPlanes := func(m map[string]*cluster.MasterNode) []ControlPlaneNode {
		controlPlanes := make([]ControlPlaneNode, 0, len(m))
		for name, nodeState := range m {
			controlPlaneNode := ControlPlaneNode{
				Name:        name,
				Phase:       string(nodeState.Phase),
				Description: nodeState.Description,
				Components:  make(map[string]string, len(nodeState.Components)),
			}

			for component, componentState := range nodeState.Components {
				controlPlaneNode.Components[component] = componentState.Version
			}

			controlPlanes = append(controlPlanes, controlPlaneNode)
		}

		slices.SortFunc(controlPlanes, func(a, b ControlPlaneNode) int {
			return strings.Compare(a.Name, b.Name)
		})

		return controlPlanes
	}

	renderProgress := func(progress string) string {
		if progress == "100%" {
			return ""
		}
		return progress
	}

	return ConfigMapData{
		Spec: &Spec{
			DesiredVersion: clusterState.Spec.DesiredVersion,
			UpdateMode:     string(clusterState.Spec.UpdateMode),
			MaxUsedVersion: clusterState.Spec.MaxUsedVersion,
		},
		Status: &Status{
			CurrentVersion:    clusterState.CurrentVersion,
			SupportedVersions: clusterState.SupportedVersions,
			AvailableVersions: clusterState.AvailableVersions,
			AutomaticVersion:  clusterState.AutomaticVersion,
			Phase:             string(clusterState.Status.Phase),
			Progress:          renderProgress(clusterState.Progress),
			ControlPlane:      renderControlPlanes(clusterState.ControlPlaneState.MasterNodes),
			Nodes: Nodes{
				DesiredCount:  clusterState.NodesState.DesiredCount,
				UpToDateCount: clusterState.NodesState.UpToDateCount,
			},
		},
	}
}

// touchConfigMap writes the reconciled object back. An empty ResourceVersion means getConfigMap
// synthesized the object because it did not exist, so this is a create; anything else came from a
// successful Get and its ResourceVersion also makes the write optimistically concurrent.
func (r *reconciler) touchConfigMap(ctx context.Context, configMap *corev1.ConfigMap) error {
	if configMap.ResourceVersion == "" {
		if err := r.client.Create(ctx, configMap); err != nil {
			return fmt.Errorf("failed to create configMap: %w", err)
		}

		return nil
	}

	if err := r.client.Update(ctx, configMap); err != nil {
		return fmt.Errorf("failed to update configMap: %w", err)
	}

	return nil
}
