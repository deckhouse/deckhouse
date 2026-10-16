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

package layout

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

func registryService(selector map[string]string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "registry", Namespace: Namespace},
		Spec:       corev1.ServiceSpec{ClusterIP: corev1.ClusterIPNone, Selector: selector},
	}
}

func everyAgentServing() []registryv1alpha1.RegistryNode {
	return []registryv1alpha1.RegistryNode{{
		ObjectMeta: metav1.ObjectMeta{Name: "master-0"},
		Status:     registryv1alpha1.RegistryNodeStatus{Reconciled: true, ProxyListening: true},
	}}
}

// The store's own Service shares the name with the previous implementation's, and is the one the
// in-cluster name resolves through. The cleanup must take the old one and leave this one: deleted
// by name, it went on every reconciliation, and an air-gapped cluster's infrastructure converger
// could no longer resolve the registry at all.
func TestTheCleanupLeavesTheStoresOwnService(t *testing.T) {
	r, c := newReconciler(t, registryService(map[string]string{"app": "registry-storage"}))

	require.NoError(t, r.removeLegacyPullPath(context.Background(), everyAgentServing()))

	service := &corev1.Service{}
	assert.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "registry", Namespace: Namespace}, service),
		"the storage's Service is this implementation's, and stays")
}

func TestTheCleanupRemovesThePreviousImplementationsService(t *testing.T) {
	r, c := newReconciler(t, registryService(map[string]string{"component": "registry-service", "type": "proxy"}))

	require.NoError(t, r.removeLegacyPullPath(context.Background(), everyAgentServing()))

	err := c.Get(context.Background(), client.ObjectKey{Name: "registry", Namespace: Namespace}, &corev1.Service{})
	assert.True(t, apierrors.IsNotFound(err), "the in-cluster proxy's Service goes once every agent serves")
}
