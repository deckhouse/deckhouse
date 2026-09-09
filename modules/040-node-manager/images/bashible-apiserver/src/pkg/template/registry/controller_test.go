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

package registry

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	ctrl "sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// reconcileWith runs one reconciliation against a cluster holding exactly the given objects, and
// returns what the controller published — or the error it gave up with.
//
// The channel is read in the background because Reconcile publishes into an unbuffered one: a test that
// only called Reconcile would deadlock on success and pass on failure, which is the wrong way round.
func reconcileWith(t *testing.T, objects ...runtime.Object) (HashedRegistryData, error) {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	sc := &StateController{
		controllerName: "registry-state-test",
		namespace:      "d8-system",
		client:         fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objects...).Build(),
		dataCh:         make(chan HashedRegistryData),
	}

	published := make(chan HashedRegistryData, 1)
	go func() {
		select {
		case data := <-sc.dataCh:
			published <- data
		case <-time.After(5 * time.Second):
		}
	}()

	_, err := sc.Reconcile(context.Background(), ctrl.Request{})
	if err != nil {
		return HashedRegistryData{}, err
	}

	select {
	case data := <-published:
		return data, nil
	case <-time.After(5 * time.Second):
		t.Fatal("the controller published nothing and did not fail either")
		return HashedRegistryData{}, nil
	}
}

func bashibleLayoutSecret() *corev1.Secret {
	// One `config` key holding the whole document, which is how the module writes it and how
	// `bashibleConfigSecret.decode` reads it. Built as a literal rather than from the model so that a
	// change to either side shows up here as a decode failure instead of passing silently.
	const layout = `
mode: managed
imagesBase: registry.d8-system.svc:5001/system/deckhouse
version: "1.0"
hosts:
  registry.d8-system.svc:5001:
    mirrors:
      - host: registry.d8-system.svc:5001
        scheme: https
`

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: bashibleConfigSecretName, Namespace: "d8-system"},
		Data:       map[string][]byte{"config": []byte(layout)},
	}
}

// TestTheLayoutAloneIsEnough is the fix this file exists for.
//
// `loadFromInput` uses the module's layout when it exists and ignores the `deckhouse-registry` secret
// entirely — but this controller used to LOAD that secret first and fail the whole reconciliation when
// it was absent. This controller is what produces a node's bootstrap data, so that failure was not
// cosmetic. Measured on a cluster where the secret had been removed on purpose: no bootstrap data
// secret was produced, CAPI reported `Secret "worker-6a06716b" not found`, and no worker VM appeared at
// all — the installation ended on "waiting for a Ready worker node", with the layout sitting there
// unread.
func TestTheLayoutAloneIsEnough(t *testing.T) {
	data, err := reconcileWith(t, bashibleLayoutSecret())

	require.NoError(t, err, "the layout describes the registry, so the secret is not needed")
	require.NotEmpty(t, data.HashSum)
	require.NotEmpty(t, data.Data, "the node has to be given something to configure itself from")
}

// TestWithoutEitherItStillFails keeps the other half: on a cluster where nothing describes the
// registry, this has to be an error rather than empty node configuration.
func TestWithoutEitherItStillFails(t *testing.T) {
	_, err := reconcileWith(t)

	require.Error(t, err)
	require.Contains(t, err.Error(), "deckhouse registry secret")
}
