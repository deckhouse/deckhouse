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

package nodeconfig

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

func TestPublishedSystemType(t *testing.T) {
	const shipped, older = "sha256:shipped", "sha256:older"
	status := func(name, digest, state string) []internalv1alpha1.ExtensionStatus {
		return []internalv1alpha1.ExtensionStatus{{Name: name, Digest: digest, State: state}}
	}
	object := func(systemType internalv1alpha1.SystemType, extensions []internalv1alpha1.ExtensionStatus) *internalv1alpha1.NodeConfig {
		nc := appliedConfig("n1", internalv1alpha1.NodeSpec{SystemType: systemType})
		nc.Status.Extensions = extensions
		return &nc
	}
	applying := func(systemType internalv1alpha1.SystemType, extensions []internalv1alpha1.ExtensionStatus) *internalv1alpha1.NodeConfig {
		nc := notApplied("n1", internalv1alpha1.NodeSpec{SystemType: systemType})
		nc.Status.Extensions = extensions
		return &nc
	}
	rejected := func(systemType internalv1alpha1.SystemType, extensions []internalv1alpha1.ExtensionStatus) *internalv1alpha1.NodeConfig {
		nc := applying(systemType, extensions)
		meta.SetStatusCondition(&nc.Status.Conditions, metav1.Condition{
			Type: internalv1alpha1.NodeConfigConditionTypeConfigurationApplied, Status: metav1.ConditionFalse,
			Reason: "Rejected", Message: "spec.systemType: unknown system type", ObservedGeneration: nc.Generation,
		})
		return nc
	}
	for _, class := range []struct {
		name             string
		rendered, legacy internalv1alpha1.SystemType
	}{
		{name: "DeckhouseEngine", rendered: internalv1alpha1.SystemTypeDeckhouseEngine, legacy: internalv1alpha1.SystemTypeLegacyImmutable},
		{name: "GenericOS", rendered: internalv1alpha1.SystemTypeGenericOS, legacy: internalv1alpha1.SystemTypeLegacyMutable},
	} {
		desired := internalv1alpha1.NodeSpec{
			SystemType: class.rendered,
			Extensions: []internalv1alpha1.Extension{
				{Name: containerdExtension, Digest: shipped},
				{Name: nodeletExtension, Digest: shipped},
			},
		}
		for _, tc := range []struct {
			name     string
			existing *internalv1alpha1.NodeConfig
			want     internalv1alpha1.SystemType
		}{
			{name: "a node with no object yet", want: class.legacy},
			{
				name:     "a node that reported no nodelet",
				existing: object(class.legacy, status(containerdExtension, shipped, extensionStateReady)),
				want:     class.legacy,
			},
			{
				name:     "a node on an older nodelet",
				existing: object(class.legacy, status(nodeletExtension, older, extensionStateReady)),
				want:     class.legacy,
			},
			{
				name:     "a node still installing the shipped nodelet",
				existing: object(class.legacy, status(nodeletExtension, shipped, extensionStatePending)),
				want:     class.legacy,
			},
			{
				name:     "a node with the shipped nodelet still applying the rest of its document",
				existing: applying(class.legacy, status(nodeletExtension, shipped, extensionStateReady)),
				want:     class.legacy,
			},
			{
				name:     "a node running the shipped nodelet",
				existing: object(class.legacy, status(nodeletExtension, shipped, extensionStateReady)),
				want:     class.rendered,
			},
			{
				name:     "a node already given the new spelling",
				existing: object(class.rendered, status(nodeletExtension, older, extensionStateReady)),
				want:     class.rendered,
			},
			{
				name:     "a node still applying the new spelling",
				existing: applying(class.rendered, status(nodeletExtension, older, extensionStateReady)),
				want:     class.rendered,
			},
			{
				name:     "a node whose nodelet refused the new spelling",
				existing: rejected(class.rendered, status(nodeletExtension, older, extensionStateReady)),
				want:     class.legacy,
			},
		} {
			t.Run(class.name+"/"+tc.name, func(t *testing.T) {
				require.Equal(t, tc.want, publishedSystemType(desired, tc.existing))
			})
		}
	}
}

func TestTheBudgetCountsANodeHandedTheLegacySpelling(t *testing.T) {
	var in clusterInputs
	in.Platform.Extensions = []internalv1alpha1.Extension{{Name: nodeletExtension, Digest: testNodeletDigest}}
	for name, extensions := range map[string][]internalv1alpha1.ExtensionStatus{
		"before it merges the shipped nodelet": nil,
		"with the shipped nodelet, the rest still applying": {
			{Name: nodeletExtension, Digest: testNodeletDigest, State: extensionStateReady},
		},
	} {
		t.Run(name, func(t *testing.T) {
			ng := mutableGroup("worker", v1.SystemTypeGenericOS)
			node := nodeInGroup("worker-0", "worker")
			published := renderMutableSpec(&node, "worker", in)
			published.SystemType = internalv1alpha1.SystemTypeLegacyMutable
			config := notApplied("worker-0", published)
			config.Labels = map[string]string{nodecommon.NodeGroupLabel: "worker"}
			config.Status.Extensions = extensions
			cl := fake.NewClientBuilder().WithScheme(nsprStatusScheme(t)).WithObjects(ng, &node, &config).Build()
			r, p := mutableReconciler(cl, in)

			budget, err := r.rolloutBudget(t.Context(), ng, p)
			require.NoError(t, err)
			require.Contains(t, budget.updating, "worker-0", "a node mid-update on the legacy spelling holds its slot")
		})
	}
}
