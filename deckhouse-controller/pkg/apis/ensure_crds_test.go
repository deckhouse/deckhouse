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

package apis

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	coordv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
)

func moduleDocumentation(name string, finalizers ...string) *unstructured.Unstructured {
	md := &unstructured.Unstructured{}
	md.SetAPIVersion("deckhouse.io/v1alpha1")
	md.SetKind("ModuleDocumentation")
	md.SetName(name)
	md.SetFinalizers(finalizers)
	return md
}

func TestDeleteModuleDocumentations(t *testing.T) {
	ctx := context.Background()
	listKinds := map[schema.GroupVersionResource]string{moduleDocumentationGVR: "ModuleDocumentationList"}

	t.Run("strips the finalizer and deletes every object", func(t *testing.T) {
		cli := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), listKinds,
			moduleDocumentation("with-finalizer", moduleDocumentationFinalizer, "other/finalizer"),
			moduleDocumentation("without-finalizer"),
		)

		require.NoError(t, deleteModuleDocumentations(ctx, cli))

		var patched []string
		for _, action := range cli.Actions() {
			if patch, ok := action.(k8stesting.PatchAction); ok {
				patched = append(patched, patch.GetName())
				require.JSONEq(t, `{"metadata":{"finalizers":["other/finalizer"]}}`, string(patch.GetPatch()))
			}
		}
		require.Equal(t, []string{"with-finalizer"}, patched)

		list, err := cli.Resource(moduleDocumentationGVR).List(ctx, metav1.ListOptions{})
		require.NoError(t, err)
		require.Empty(t, list.Items)
	})

	t.Run("a cluster without the CRD has nothing to delete", func(t *testing.T) {
		cli := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), listKinds)
		cli.PrependReactor("list", "moduledocumentations", func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewNotFound(moduleDocumentationGVR.GroupResource(), "")
		})

		require.NoError(t, deleteModuleDocumentations(ctx, cli))
	})

	t.Run("a failed delete is returned and the next call deletes the rest", func(t *testing.T) {
		cli := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), listKinds,
			moduleDocumentation("first", moduleDocumentationFinalizer),
			moduleDocumentation("second", moduleDocumentationFinalizer),
		)
		failed := false
		cli.PrependReactor("delete", "moduledocumentations", func(k8stesting.Action) (bool, runtime.Object, error) {
			if failed {
				return false, nil, nil
			}
			failed = true
			return true, nil, errors.New("the API server is down")
		})

		require.ErrorContains(t, deleteModuleDocumentations(ctx, cli), "the API server is down")
		require.NoError(t, deleteModuleDocumentations(ctx, cli))

		list, err := cli.Resource(moduleDocumentationGVR).List(ctx, metav1.ListOptions{})
		require.NoError(t, err)
		require.Empty(t, list.Items)
	})

	t.Run("an object gone before the patch is no error", func(t *testing.T) {
		cli := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), listKinds,
			moduleDocumentation("gone", moduleDocumentationFinalizer),
		)
		cli.PrependReactor("patch", "moduledocumentations", func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewNotFound(moduleDocumentationGVR.GroupResource(), "gone")
		})

		require.NoError(t, deleteModuleDocumentations(ctx, cli))
	})
}

func TestDeleteDocumentationLeases(t *testing.T) {
	ctx := context.Background()
	lease := func(name, namespace string, labels map[string]string) *coordv1.Lease {
		return &coordv1.Lease{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: labels}}
	}
	cli := kubefake.NewSimpleClientset(
		lease("module-docs-builder-abc", "d8-system", map[string]string{documentationLeaseLabel: ""}),
		lease("docs-builder-ondemand-leader", "d8-system", nil),
		lease("module-docs-builder-elsewhere", "default", map[string]string{documentationLeaseLabel: ""}),
	)

	require.NoError(t, deleteDocumentationLeases(ctx, cli))
	// Deleting again finds nothing to delete.
	require.NoError(t, deleteDocumentationLeases(ctx, cli))

	left, err := cli.CoordinationV1().Leases("d8-system").List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, left.Items, 1)
	require.Equal(t, "docs-builder-ondemand-leader", left.Items[0].Name)
	_, err = cli.CoordinationV1().Leases("default").Get(ctx, "module-docs-builder-elsewhere", metav1.GetOptions{})
	require.NoError(t, err)
}

// Run under every build tag set of editions.yaml: the licensing CRDs follow the
// build, everything else is installed regardless.
func TestInstallCRDFile(t *testing.T) {
	if installCRDFile("/deckhouse/deckhouse-controller/crds/doc-ru-module.yaml") {
		t.Error("a translated doc file was installed")
	}
	if !installCRDFile("/deckhouse/deckhouse-controller/crds/module.yaml") {
		t.Error("a regular CRD was skipped")
	}
	for _, name := range licensingCRDs {
		if got := installCRDFile("/deckhouse/deckhouse-controller/crds/" + name); got != app.LicensingEnabled() {
			t.Errorf("%s installed=%v, licensing enabled=%v", name, got, app.LicensingEnabled())
		}
	}
}
