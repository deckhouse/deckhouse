// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resourcesize

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// renderWithFallbacks runs the patches of the requests over resources, and then the
// fallback patches of the ranges the autoscalers do not apply, as a render after the
// autoscalers are known does.
func renderWithFallbacks(t *testing.T, resources []*unstructured.Unstructured, requests []Request, autoscaled Autoscaled) ([]Target, error) {
	t.Helper()

	patches, err := Patches(requests)
	if err != nil {
		return nil, err
	}

	fallbacks, fallbackPatches, err := FallbackPatches(requests, autoscaled)
	if err != nil {
		return nil, err
	}

	return fallbacks, applyPatches(t, resources, append(patches, fallbackPatches...))
}

// rangedController asks for ranges of deploymentManifest's workload: its replicas, the
// cpu and memory of its controller container and the memory of its migrate init container.
var rangedController = []Request{{
	Kind:        "Deployment",
	Name:        "controller",
	MinReplicas: int32Ptr(3),
	MaxReplicas: int32Ptr(7),
	Containers: []Container{
		{
			Name: "controller",
			Min:  map[string]string{"cpu": "100m", "memory": "64Mi"},
			Max:  map[string]string{"cpu": "1", "memory": "1Gi"},
		},
		{
			Name: "migrate",
			Min:  map[string]string{"memory": "16Mi"},
			Max:  map[string]string{"memory": "128Mi"},
		},
	},
}}

// Without autoscalers the ranges apply to the workload as fixed values: the replicas as
// the minimum, a resource as a request of its min and a limit of its max.
func TestFallbackPatchesApplyRangesWithoutAutoscalers(t *testing.T) {
	deployment := parse(t, deploymentManifest)

	fallbacks, err := renderWithFallbacks(t, []*unstructured.Unstructured{deployment}, rangedController, Autoscaled{})
	require.NoError(t, err)

	require.Equal(t, []Target{
		{Kind: "Deployment", Name: "controller", Resource: "replicas"},
		{Kind: "Deployment", Name: "controller", Container: "controller", Resource: "cpu"},
		{Kind: "Deployment", Name: "controller", Container: "controller", Resource: "memory"},
		{Kind: "Deployment", Name: "controller", Container: "migrate", Resource: "memory"},
	}, fallbacks)
	require.Equal(t, "HorizontalPodAutoscaler", fallbacks[0].Autoscaler())
	require.Equal(t, "VerticalPodAutoscaler", fallbacks[1].Autoscaler())

	require.Equal(t, int64(3), renderedReplicas(t, deployment))

	resources := containerResources(t, deployment, "controller", "spec", "template", "spec")
	require.Equal(t, map[string]any{"cpu": "100m", "memory": "64Mi"}, resources["requests"])
	require.Equal(t, map[string]any{"cpu": "1", "memory": "1Gi"}, resources["limits"])

	initContainers, _, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "initContainers")
	require.NoError(t, err)
	require.Equal(t, map[string]any{
		"requests": map[string]any{"memory": "16Mi"},
		"limits":   map[string]any{"memory": "128Mi"},
	}, initContainers[0].(map[string]any)["resources"])
}

// The ranges an autoscaler applies are left to it: the workload keeps what the chart renders.
func TestFallbackPatchesLeaveAutoscaledRangesAlone(t *testing.T) {
	deployment := parse(t, deploymentManifest)
	before := deployment.DeepCopy()

	fallbacks, err := renderWithFallbacks(t, []*unstructured.Unstructured{deployment}, rangedController, Autoscaled{
		Replicas: []AutoscaledReplicas{{Kind: "Deployment", Name: "controller"}},
		Resources: []AutoscaledResources{
			{Kind: "Deployment", Name: "controller", Container: "controller", Resources: []string{"cpu", "memory"}},
			{Kind: "Deployment", Name: "controller", Container: "migrate", Resources: []string{"memory"}},
		},
	})
	require.NoError(t, err)
	require.Empty(t, fallbacks)
	require.Equal(t, before, deployment)
}

// A VerticalPodAutoscaler that sets one resource of a container leaves the range of the
// other to the fallback; an autoscaler of another workload applies nothing here.
func TestFallbackPatchesCoverWhatAutoscalersLeave(t *testing.T) {
	deployment := parse(t, deploymentManifest)

	fallbacks, err := renderWithFallbacks(t, []*unstructured.Unstructured{deployment}, rangedController[:1], Autoscaled{
		Replicas: []AutoscaledReplicas{{Kind: "StatefulSet", Name: "controller"}},
		Resources: []AutoscaledResources{
			{Kind: "Deployment", Name: "controller", Container: "controller", Resources: []string{"memory"}},
			{Kind: "Deployment", Name: "controller", Container: "migrate", Resources: []string{"memory"}},
		},
	})
	require.NoError(t, err)

	require.Equal(t, []Target{
		{Kind: "Deployment", Name: "controller", Resource: "replicas"},
		{Kind: "Deployment", Name: "controller", Container: "controller", Resource: "cpu"},
	}, fallbacks)

	resources := containerResources(t, deployment, "controller", "spec", "template", "spec")
	require.Equal(t, map[string]any{"cpu": "100m", "memory": "32Mi"}, resources["requests"])
	require.Equal(t, map[string]any{"cpu": "1"}, resources["limits"])
}

// The replicas a fallback sets are guarded by the workload's annotations, like replicas
// asked for directly.
func TestFallbackPatchesGuardReplicas(t *testing.T) {
	deployment := scalableDeployment(t, "replicas: 5", map[string]string{annotationScaleDownDisabled: "true"})

	_, err := renderWithFallbacks(t, []*unstructured.Unstructured{deployment}, []Request{{
		Kind:        "Deployment",
		Name:        "controller",
		MinReplicas: int32Ptr(3),
		MaxReplicas: int32Ptr(7),
	}}, Autoscaled{})
	requireScalingError(t, err, "Deployment 'controller' does not allow its replicas to go below 5")
}

// Fixed values are no ranges and have nothing to fall back to.
func TestFallbackPatchesIgnoreFixedValues(t *testing.T) {
	fallbacks, patches, err := FallbackPatches([]Request{{
		Kind:       "Deployment",
		Name:       "controller",
		Replicas:   int32Ptr(3),
		Containers: []Container{{Name: "controller", Requests: map[string]string{"cpu": "100m"}}},
	}}, Autoscaled{})
	require.NoError(t, err)
	require.Empty(t, fallbacks)
	require.Empty(t, patches)
}

// nelm reads a name wrapped in slashes as a regexp, so such a request falls back to nothing.
func TestFallbackPatchesSkipNamesThatReadAsRegexp(t *testing.T) {
	fallbacks, patches, err := FallbackPatches([]Request{{
		Kind:        "Deployment",
		Name:        "/.*/",
		MinReplicas: int32Ptr(3),
		MaxReplicas: int32Ptr(7),
	}}, Autoscaled{})
	require.NoError(t, err)
	require.Empty(t, fallbacks)
	require.Empty(t, patches)
}

// controllerAutoscaled has the replicas of deploymentManifest's workload scaled by a
// HorizontalPodAutoscaler, and the memory alone of its controller container sized by a
// VerticalPodAutoscaler.
var controllerAutoscaled = Autoscaled{
	Replicas:  []AutoscaledReplicas{{Kind: "Deployment", Name: "controller"}},
	Resources: []AutoscaledResources{{Kind: "Deployment", Name: "controller", Container: "controller", Resources: []string{"memory"}}},
}

// A fixed value pins its autoscaler; a value no autoscaler scales, cpu here, or
// ephemeral-storage, which VPA leaves alone, pins nothing.
func TestPinsFindFixedValuesOfAutoscaledTargets(t *testing.T) {
	pins := Pins([]Request{{
		Kind:     "Deployment",
		Name:     "controller",
		Replicas: int32Ptr(3),
		Containers: []Container{
			{
				Name:     "controller",
				Requests: map[string]string{"cpu": "100m", "ephemeral-storage": "1Gi"},
				Limits:   map[string]string{"memory": "1Gi"},
			},
			{Name: "kube-rbac-proxy", Requests: map[string]string{"memory": "64Mi"}},
		},
	}}, controllerAutoscaled)

	require.Equal(t, []Target{
		{Kind: "Deployment", Name: "controller", Resource: "replicas"},
		{Kind: "Deployment", Name: "controller", Container: "controller", Resource: "memory"},
	}, pins)
}

// Zero replicas go to the workload alone, and ranges bound the autoscalers rather than
// fix them: neither pins anything.
func TestPinsLeaveZeroReplicasAndRangesAlone(t *testing.T) {
	require.Empty(t, Pins([]Request{{
		Kind:     "Deployment",
		Name:     "controller",
		Replicas: int32Ptr(0),
		Containers: []Container{{
			Name: "controller",
			Min:  map[string]string{"memory": "64Mi"},
			Max:  map[string]string{"memory": "1Gi"},
		}},
	}}, controllerAutoscaled))
}
