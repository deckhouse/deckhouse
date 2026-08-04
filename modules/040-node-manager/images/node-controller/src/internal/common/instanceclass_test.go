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

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

func instanceClass(kind, name string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("deckhouse.io/v1alpha1")
	u.SetKind(kind)
	u.SetName(name)
	return u
}

func cloudNodeGroup(name, classKind, className string) *v1.NodeGroup {
	return &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1.NodeGroupSpec{
			NodeType: v1.NodeTypeCloudEphemeral,
			CloudInstances: &v1.CloudInstancesSpec{
				ClassReference: v1.ClassReference{Kind: classKind, Name: className},
			},
		},
	}
}

func TestInstanceClassToNodeGroups(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		cloudNodeGroup("worker", "AWSInstanceClass", "worker"),
		cloudNodeGroup("worker-copy", "AWSInstanceClass", "worker"),
		cloudNodeGroup("other-name", "AWSInstanceClass", "spot"),
		cloudNodeGroup("other-kind", "DVPInstanceClass", "worker"),
		&v1.NodeGroup{
			ObjectMeta: metav1.ObjectMeta{Name: "static"},
			Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
		},
	).Build()

	t.Run("enqueues every NodeGroup referencing the class", func(t *testing.T) {
		got := InstanceClassToNodeGroups(t.Context(), c, instanceClass("AWSInstanceClass", "worker"))

		names := make([]string, 0, len(got))
		for _, r := range got {
			names = append(names, r.Name)
		}
		assert.ElementsMatch(t, []string{"worker", "worker-copy"}, names,
			"a class edit must re-render every NodeGroup pointing at it, and nothing else")
	})

	t.Run("same name in another provider kind does not match", func(t *testing.T) {
		got := InstanceClassToNodeGroups(t.Context(), c, instanceClass("YandexInstanceClass", "worker"))
		assert.Empty(t, got)
	})

	t.Run("unknown class enqueues nothing", func(t *testing.T) {
		got := InstanceClassToNodeGroups(t.Context(), c, instanceClass("AWSInstanceClass", "absent"))
		assert.Empty(t, got)
	})

	t.Run("non-unstructured object is ignored", func(t *testing.T) {
		got := InstanceClassToNodeGroups(t.Context(), c, &v1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "worker"}})
		assert.Empty(t, got)
	})
}
