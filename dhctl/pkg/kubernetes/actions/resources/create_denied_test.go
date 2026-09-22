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

package resources

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stesting "k8s.io/client-go/testing"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

// A namespace stub is queued for every namespace the resource document references, and a
// re-run queues it again on a cluster where Deckhouse already created it. Deckhouse's own
// system-ns.deckhouse.io policy then denies the CREATE - admission answers before the API
// server can report the conflict, so the refusal is a Forbidden and not an AlreadyExists.
// Reading the create's refusal as "the object is missing" leaves the second bootstrap
// retrying a call that can never succeed until the ten-minute loop gives up.
func TestCreateForbiddenOnExistingObjectIsTreatedAsUpdate(t *testing.T) {
	const namespaceName = "d8-cloud-provider-dvp"

	nsGVR := schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}
	kubeCl := client.NewFakeKubernetesClientWithListGVR(map[schema.GroupVersionResource]string{
		nsGVR: "NamespaceList",
	})

	existing := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Namespace",
		"metadata":   map[string]any{"name": namespaceName},
	}}
	_, err := kubeCl.Dynamic().Resource(nsGVR).Create(context.Background(), existing, metav1.CreateOptions{})
	require.NoError(t, err)

	dynamicFake, ok := kubeCl.Dynamic().(interface {
		PrependReactor(verb, resource string, reaction k8stesting.ReactionFunc)
	})
	require.True(t, ok, "the fake dynamic client has to accept reactors")

	dynamicFake.PrependReactor("create", "namespaces", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(nsGVR.GroupResource(), namespaceName,
			apierrors.NewBadRequest("ValidatingAdmissionPolicy 'system-ns.deckhouse.io' with binding "+
				"'system-ns.deckhouse.io' denied request: Creation of system namespaces is forbidden"))
	})

	stub := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Namespace",
		"metadata":   map[string]any{"name": namespaceName},
	}}
	resource := &template.Resource{
		GVK:    schema.FromAPIVersionAndKind("v1", "Namespace"),
		Object: *stub,
	}

	creator := NewCreator(kubeCl, template.Resources{resource}, nil)

	// The retry loop waits ten minutes; bound the test so the old behaviour fails fast
	// instead of hanging the suite.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = creator.createSingleResource(ctx, resource, metav1.APIResource{
		Name:       "namespaces",
		Namespaced: false,
		Kind:       "Namespace",
	})
	require.NoError(t, err, "a create the admission policy refuses on an object that already exists must fall through to the update")
}
