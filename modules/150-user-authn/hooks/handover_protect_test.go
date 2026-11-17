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
	"fmt"
	"testing"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/metadata"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/encoding"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// TestHandoverHookStopsAskingForTheKindsThatAreHandedOver: an object that stays on its own
// dex-authenticator keeps the objects of that dex-authenticator in the release, maybe for good.
// The hook must not go on listing, on every run of the module, the kinds whose objects the
// controller adopted long ago: the metadata of every Secret of the cluster for the DexClients
// among them.
func TestHandoverHookStopsAskingForTheKindsThatAreHandedOver(t *testing.T) {
	var (
		services    = schema.GroupVersionResource{Version: "v1", Resource: "services"}
		deployments = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	)
	chart := map[string]string{"app": "dex-authenticator", "heritage": "deckhouse", "module": "user-authn"}
	kept := map[string]string{"meta.helm.sh/release-name": "user-authn", "meta.helm.sh/release-namespace": "d8-system", "helm.sh/resource-policy": "keep"}

	var objs []runtime.Object
	// The objects of the DexClients are the controller's already.
	for i := range 20 {
		namespace, name := fmt.Sprintf("team-%d", i), "app"
		objs = append(objs,
			handoverObject(dexClientGVR, "DexClient", namespace, name, nil, nil),
			handoverObject(secretGVR, "Secret", namespace, dexClientSecretPrefix+name, nil, nil),
			handoverObject(oauth2ClientGVR, "OAuth2Client", dexNamespace, encoding.ToFnvLikeDex(fmt.Sprintf("dex-client-%s@%s", name, namespace)), nil, nil),
		)
	}
	// One DexAuthenticator stays on its own dex-authenticator: its Deployment and Service are of
	// the release, kept from it.
	objs = append(objs,
		handoverObject(dexAuthenticatorGVR, "DexAuthenticator", "shop", "app", nil, nil),
		handoverObject(deployments, "Deployment", "shop", "app-dex-authenticator", chart, kept),
		handoverObject(services, "Service", "shop", "app-dex-authenticator", map[string]string{
			"app": "dex-authenticator", "heritage": "deckhouse", "module": "user-authn", "app.kubernetes.io/managed-by": "user-authn-controller",
		}, kept),
	)
	c := newHandoverFakeClient(objs...)

	previousClient, previousKinds := newHandoverClient, handedOver.names
	newHandoverClient = func(dependency.Container) (metadata.Interface, error) { return c, nil }
	handedOver.names = nil
	t.Cleanup(func() { newHandoverClient, handedOver.names = previousClient, previousKinds })

	lists := func() map[string]int {
		out := map[string]int{}
		for _, action := range c.Actions() {
			if action.GetVerb() != "list" {
				t.Errorf("the hook made a %s of %s: everything is protected already", action.GetVerb(), action.GetResource().Resource)
			}
			out[action.GetResource().Resource]++
		}
		return out
	}
	run := func() {
		t.Helper()
		c.ClearActions()
		if err := protectHandoverObjects(context.Background(), &go_hook.HookInput{Logger: log.NewNop()}, nil); err != nil {
			t.Fatalf("the hook: %v", err)
		}
	}

	// The first run asks for every kind.
	run()
	first := lists()
	if first["dexclients"] == 0 || first["secrets"] < 2 || first["deployments"] != 1 {
		t.Fatalf("the first run listed %v, want every kind asked for", first)
	}

	// The later runs ask only for the kinds the release still has objects of.
	for range 2 {
		run()
		later := lists()
		if want := map[string]int{"deployments": 1, "services": 1}; fmt.Sprint(later) != fmt.Sprint(want) {
			t.Errorf("a later run listed %v, want %v: the kinds without an object of the release are not asked for", later, want)
		}
	}

	// The object moves to an instance, and the controller removes the Deployment and takes the
	// Service over: nothing is left, and the hook makes no request at all.
	if err := c.Resource(deployments).Namespace("shop").Delete(context.Background(), "app-dex-authenticator", metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	adopted := handoverObject(services, "Service", "shop", "app-dex-authenticator", map[string]string{"app.kubernetes.io/managed-by": "user-authn-controller"}, nil)
	if err := c.Tracker().Update(services, adopted, "shop"); err != nil {
		t.Fatal(err)
	}
	run()
	run()
	if left := lists(); len(left) != 0 {
		t.Errorf("a run after everything was handed over listed %v, want no request", left)
	}
}
