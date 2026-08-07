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
	"encoding/json"
	"testing"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	sdkpkg "github.com/deckhouse/module-sdk/pkg"
	sdkpatchablevalues "github.com/deckhouse/module-sdk/pkg/patchable-values"
)

// recordedMerge is a PatchWithMerge call, kept whole so a test can assert on the patch body and
// not just on the fact that something was patched.
type recordedMerge struct {
	patch      interface{}
	apiVersion string
	kind       string
	namespace  string
	name       string
}

// recordingPatchCollector implements go_hook.PatchCollector through an embedded nil interface:
// any operation other than PatchWithMerge panics, which is itself an assertion — this hook must
// never create, delete or replace the Secret.
type recordingPatchCollector struct {
	go_hook.PatchCollector

	merges []recordedMerge
}

func (c *recordingPatchCollector) PatchWithMerge(patch interface{}, apiVersion, kind, namespace, name string, _ ...sdkpkg.PatchCollectorOption) {
	c.merges = append(c.merges, recordedMerge{
		patch: patch, apiVersion: apiVersion, kind: kind, namespace: namespace, name: name,
	})
}

type stubSnapshot struct{ value interface{} }

func (s stubSnapshot) UnmarshalTo(out interface{}) error {
	raw, err := json.Marshal(s.value)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

func (s stubSnapshot) String() string {
	raw, _ := json.Marshal(s.value)
	return string(raw)
}

type stubSnapshots map[string][]sdkpkg.Snapshot

func (s stubSnapshots) Get(key string) []sdkpkg.Snapshot { return s[key] }

func adoptHookInput(t *testing.T, selfHosted bool, bashibleContext string, snapshots []sdkpkg.Snapshot) (*go_hook.HookInput, *recordingPatchCollector) {
	t.Helper()

	values, err := sdkpatchablevalues.NewPatchableValues(map[string]interface{}{
		"global":      map[string]interface{}{"deckhouseSelfHosted": selfHosted},
		"nodeManager": map[string]interface{}{"internal": map[string]interface{}{"bashibleContext": bashibleContext}},
	})
	require.NoError(t, err)

	collector := &recordingPatchCollector{}
	return &go_hook.HookInput{
		Values:         values,
		Snapshots:      stubSnapshots{"context": snapshots},
		PatchCollector: collector,
	}, collector
}

func contextSnapshot(adopted bool) []sdkpkg.Snapshot {
	return []sdkpkg.Snapshot{stubSnapshot{value: bashibleContextOwnership{Adopted: adopted}}}
}

const someBashibleContext = "clusterDomain: cluster.virtual\n"

func TestBashibleContextAdoptVCP(t *testing.T) {
	t.Run("secret absent", func(t *testing.T) {
		input, collector := adoptHookInput(t, false, someBashibleContext, nil)
		require.NoError(t, handleBashibleContextAdoptVCP(context.Background(), input))
		require.Empty(t, collector.merges)
	})

	t.Run("secret present without metadata", func(t *testing.T) {
		input, collector := adoptHookInput(t, false, someBashibleContext, contextSnapshot(false))
		require.NoError(t, handleBashibleContextAdoptVCP(context.Background(), input))

		require.Len(t, collector.merges, 1)
		got := collector.merges[0]
		require.Equal(t, "v1", got.apiVersion)
		require.Equal(t, "Secret", got.kind)
		require.Equal(t, "d8-cloud-instance-manager", got.namespace)
		require.Equal(t, "bashible-apiserver-context", got.name)

		// Metadata only, and only the three fields Helm's ownership check reads: no data, no
		// type, and no mention of the app label the Secret already carries.
		require.Equal(t, map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]string{
					"app.kubernetes.io/managed-by": "Helm",
				},
				"annotations": map[string]string{
					"meta.helm.sh/release-name":      "node-manager",
					"meta.helm.sh/release-namespace": "d8-system",
				},
			},
		}, got.patch)
	})

	t.Run("secret already stamped", func(t *testing.T) {
		input, collector := adoptHookInput(t, false, someBashibleContext, contextSnapshot(true))
		require.NoError(t, handleBashibleContextAdoptVCP(context.Background(), input))
		require.Empty(t, collector.merges)
	})

	t.Run("self-hosted", func(t *testing.T) {
		input, collector := adoptHookInput(t, true, someBashibleContext, contextSnapshot(false))
		require.NoError(t, handleBashibleContextAdoptVCP(context.Background(), input))
		require.Empty(t, collector.merges)
	})

	t.Run("nested but no context to render", func(t *testing.T) {
		input, collector := adoptHookInput(t, false, "", contextSnapshot(false))
		require.NoError(t, handleBashibleContextAdoptVCP(context.Background(), input))
		require.Empty(t, collector.merges)
	})
}

// TestFilterBashibleContextOwnership pins that the context never reaches the snapshot: the
// Secret's payload is the live bashible input.yaml and this hook has no business reading it.
func TestFilterBashibleContextOwnership(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bashible-apiserver-context",
			Namespace: "d8-cloud-instance-manager",
			Labels:    map[string]string{"app": "bashible-apiserver"},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{"input.yaml": []byte("kubernetesCA: secret\n")},
	}

	raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(secret)
	require.NoError(t, err)

	filtered, err := filterBashibleContextOwnership(&unstructured.Unstructured{Object: raw})
	require.NoError(t, err)
	require.Equal(t, bashibleContextOwnership{Adopted: false}, filtered)

	secret.Labels["app.kubernetes.io/managed-by"] = "Helm"
	secret.Annotations = map[string]string{
		"meta.helm.sh/release-name":      "node-manager",
		"meta.helm.sh/release-namespace": "d8-system",
	}
	raw, err = runtime.DefaultUnstructuredConverter.ToUnstructured(secret)
	require.NoError(t, err)

	filtered, err = filterBashibleContextOwnership(&unstructured.Unstructured{Object: raw})
	require.NoError(t, err)
	require.Equal(t, bashibleContextOwnership{Adopted: true}, filtered)
}

// TestBashibleContextAdoptedByWrongRelease guards the failure mode that no retry clears: a
// Secret stamped for another release must be corrected, not left alone.
func TestBashibleContextAdoptedByWrongRelease(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bashible-apiserver-context",
			Namespace: "d8-cloud-instance-manager",
			Labels:    map[string]string{"app.kubernetes.io/managed-by": "Helm"},
			Annotations: map[string]string{
				"meta.helm.sh/release-name":      "node-manager",
				"meta.helm.sh/release-namespace": "d8-cloud-instance-manager",
			},
		},
	}

	raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(secret)
	require.NoError(t, err)

	filtered, err := filterBashibleContextOwnership(&unstructured.Unstructured{Object: raw})
	require.NoError(t, err)
	require.Equal(t, bashibleContextOwnership{Adopted: false}, filtered)
}
