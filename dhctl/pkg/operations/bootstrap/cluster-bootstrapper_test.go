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

package bootstrap

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

func newResource(t *testing.T, apiVersion, kind, name, namespace string, fields map[string]any) *template.Resource {
	t.Helper()
	obj := unstructured.Unstructured{}
	obj.SetAPIVersion(apiVersion)
	obj.SetKind(kind)
	obj.SetName(name)
	if namespace != "" {
		obj.SetNamespace(namespace)
	}
	for k, v := range fields {
		obj.Object[k] = v
	}
	return &template.Resource{
		GVK:    schema.FromAPIVersionAndKind(apiVersion, kind),
		Object: obj,
	}
}

func TestSplitResources_CredentialSecretGoesToBefore(t *testing.T) {
	credSecret := newResource(t, "v1", "Secret", "d8-credentials", "d8-cloud-provider-dvp", map[string]any{
		"type": "cloud-provider.deckhouse.io/credentials",
	})
	regularResource := newResource(t, "deckhouse.io/v1alpha1", "ModuleConfig", "user-authn", "", nil)

	before, after := splitResourcesOnPreAndPostDeckhouseInstall(context.TODO(), template.Resources{credSecret, regularResource})

	// before queue must contain the credential Secret AND a namespace stub for d8-cloud-provider-dvp.
	require.Len(t, before, 2)
	require.Equal(t, "Namespace", before[0].Object.GetKind())
	require.Equal(t, "d8-cloud-provider-dvp", before[0].Object.GetName())
	require.Equal(t, "Secret", before[1].Object.GetKind())
	require.Equal(t, "d8-credentials", before[1].Object.GetName())

	require.Len(t, after, 1)
	require.Equal(t, "ModuleConfig", after[0].Object.GetKind())
}

func TestSplitResources_NonCredentialSecretGoesToAfter(t *testing.T) {
	plainSecret := newResource(t, "v1", "Secret", "my-secret", "default", map[string]any{
		"type": "Opaque",
	})

	before, after := splitResourcesOnPreAndPostDeckhouseInstall(context.TODO(), template.Resources{plainSecret})

	require.Empty(t, before)
	require.Len(t, after, 1)
	require.Equal(t, "my-secret", after[0].Object.GetName())
}

func TestSplitResources_BeforeAnnotationStillRespected(t *testing.T) {
	annotated := newResource(t, "v1", "ConfigMap", "annotated", "kube-system", nil)
	annotated.Object.SetAnnotations(map[string]string{
		"dhctl.deckhouse.io/bootstrap-resource-place": "before-deckhouse",
	})

	before, after := splitResourcesOnPreAndPostDeckhouseInstall(context.TODO(), template.Resources{annotated})

	require.Empty(t, after)
	// Namespace stub for kube-system is added even though kube-system always exists; harmless.
	require.Len(t, before, 2)
	require.Equal(t, "Namespace", before[0].Object.GetKind())
	require.Equal(t, "ConfigMap", before[1].Object.GetKind())
}

func TestSplitResources_ExplicitNamespaceNotDuplicated(t *testing.T) {
	credSecret := newResource(t, "v1", "Secret", "d8-credentials", "my-ns", map[string]any{
		"type": "cloud-provider.deckhouse.io/credentials",
	})
	explicitNS := newResource(t, "v1", "Namespace", "my-ns", "", nil)
	explicitNS.Object.SetAnnotations(map[string]string{
		"dhctl.deckhouse.io/bootstrap-resource-place": "before-deckhouse",
	})

	before, _ := splitResourcesOnPreAndPostDeckhouseInstall(context.TODO(), template.Resources{credSecret, explicitNS})

	// Only one Namespace entry — the user-provided one, no auto-stub.
	nsCount := 0
	for _, r := range before {
		if r.Object.GetKind() == "Namespace" {
			nsCount++
		}
	}
	require.Equal(t, 1, nsCount)
}
