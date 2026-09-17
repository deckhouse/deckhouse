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

package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The control plane's own manifests are written by the node agent. A static pod
// of one of those names would give one file two writers, and the loser is
// whichever of them ran last.
func TestIsReservedStaticPodName(t *testing.T) {
	for _, name := range []string{"etcd", "kube-apiserver", "kube-controller-manager", "kube-scheduler"} {
		require.True(t, IsReservedStaticPodName(name), "%s is the node agent's own manifest", name)
	}
	require.False(t, IsReservedStaticPodName("registry-agent"))
	require.False(t, IsReservedStaticPodName("etcd-backup"))
}

// The manifest is checked where it is written, not on every node that reads it:
// a node refusing a document it cannot decode has already taken the rollout slot,
// and it refuses the whole NodeConfig with it. One question — is this a valid Pod
// — answered by the decoder rather than by a second opinion about kinds.
func TestValidateStaticPodManifest(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
		wantPod  string
		wantErr  string
	}{
		{
			// The pod key comes back so the caller does not parse the document a
			// second time to find out which pod two objects are fighting over.
			name:     "a Pod with a name and a namespace",
			manifest: "apiVersion: v1\nkind: Pod\nmetadata:\n  name: registry-agent\n  namespace: d8-system\nspec:\n  hostNetwork: true\n  containers:\n  - name: agent\n    image: deckhouse.local/images:registry-agent\n",
			wantPod:  "d8-system/registry-agent",
		},
		{
			// The object's own name is the file name on the node and nothing more:
			// kubelet takes the mirror pod's name from the document, never from the
			// file, so the two have no reason to agree.
			name:     "the pod is named nothing like its object",
			manifest: "apiVersion: v1\nkind: Pod\nmetadata:\n  name: something-else\n  namespace: kube-system\n",
			wantPod:  "kube-system/something-else",
		},
		{
			// Loose parsing, so a field this binary's k8s.io/api does not know is
			// accepted: the document is executed by a kubelet of its own version,
			// and refusing it here would refuse a Pod field newer than the vendored
			// types on every node the document reaches.
			name:     "a field the vendored Pod type does not know",
			manifest: "apiVersion: v1\nkind: Pod\nmetadata:\n  name: registry-agent\n  namespace: d8-system\nspec:\n  hostNetwrk: true\n",
			wantPod:  "d8-system/registry-agent",
		},
		{
			name:     "not a document at all",
			manifest: "\tnot: yaml",
			wantErr:  "manifest is not a valid Pod",
		},
		{
			// Loose is not blind: a field of the right name and the wrong type is
			// still a manifest kubelet cannot run, and the decoder says so.
			name:     "a field of the right name and the wrong type",
			manifest: "apiVersion: v1\nkind: Pod\nmetadata:\n  name: registry-agent\n  namespace: d8-system\nspec:\n  hostNetwork: \"yes, please\"\n",
			wantErr:  "manifest is not a valid Pod",
		},
		{
			// The case the nil "into" exists for: handed a &corev1.Pod{} the decoder
			// would take the missing kind from the destination type and accept this
			// as a Pod. With nil it answers "Object 'Kind' is missing in ...".
			name:     "no kind at all",
			manifest: "apiVersion: v1\nmetadata:\n  name: registry-agent\n  namespace: d8-system\n",
			wantErr:  "manifest is not a valid Pod",
		},
		{
			name:     "no apiVersion and no kind",
			manifest: "metadata:\n  name: registry-agent\n  namespace: d8-system\n",
			wantErr:  "manifest is not a valid Pod",
		},
		{
			// A group this scheme never registered: the decoder refuses it before
			// anything here has to reason about kinds.
			name:     "a Deployment is not a static pod",
			manifest: "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: registry-agent\n  namespace: d8-system\n",
			wantErr:  "manifest is not a valid Pod",
		},
		{
			// A core/v1 kind that is not a Pod decodes cleanly, so the type
			// assertion is the one that has to catch it — and it names the kind the
			// decoder did find rather than the Go type it built.
			name:     "a ConfigMap is not a static pod either",
			manifest: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: registry-agent\n  namespace: d8-system\n",
			wantErr:  "unexpected kind /v1, Kind=ConfigMap",
		},
		{
			name:     "no name",
			manifest: "apiVersion: v1\nkind: Pod\nmetadata:\n  namespace: d8-system\n",
			wantErr:  "metadata.name is empty",
		},
		{
			name:     "no namespace",
			manifest: "apiVersion: v1\nkind: Pod\nmetadata:\n  name: registry-agent\n",
			wantErr:  "metadata.namespace is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod, err := ValidateStaticPodManifest(tt.manifest)
			if tt.wantErr == "" {
				require.NoError(t, err)
				require.Equal(t, tt.wantPod, pod)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}
