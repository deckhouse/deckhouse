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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/go_lib/encoding"
	"github.com/deckhouse/deckhouse/modules/150-user-authn/hooks/internal/handover"
)

// TestHandoverKeepsTheDexAuthenticatorOfEveryObject: the release that stops rendering the
// dex-authenticator of every DexAuthenticator must prune nothing of it, and the objects the
// controller takes over get its label, without which it may not change them.
func TestHandoverKeepsTheDexAuthenticatorOfEveryObject(t *testing.T) {
	const managedBy = "app.kubernetes.io/managed-by"
	chartLabels := func(extra map[string]string) map[string]string {
		out := map[string]string{"app": "dex-authenticator", "heritage": "deckhouse", "module": "user-authn", managedBy: "Helm"}
		for key, value := range extra {
			out[key] = value
		}
		return out
	}
	forApp := chartLabels(map[string]string{"deckhouse.io/dex-authenticator-for": "app"})
	clientName := encoding.ToFnvLikeDex("dex-authenticator-app@shop")

	var (
		services        = schema.GroupVersionResource{Version: "v1", Resource: "services"}
		ingresses       = schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"}
		httpRoutes      = schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "httproutes"}
		deployments     = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
		budgets         = schema.GroupVersionResource{Group: "policy", Version: "v1", Resource: "poddisruptionbudgets"}
		autoscalers     = schema.GroupVersionResource{Group: "autoscaling.k8s.io", Version: "v1", Resource: "verticalpodautoscalers"}
		networkPolicies = schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"}
	)
	handed := []struct {
		gvr             schema.GroupVersionResource
		namespace, name string
	}{
		{oauth2ClientGVR, dexNamespace, clientName},
		{services, "shop", "app-dex-authenticator"},
		{ingresses, "shop", "app-dex-authenticator"},
		{ingresses, "shop", "app-dex-authenticator-sign-out"},
		{httpRoutes, "shop", "app-dex-authenticator"},
	}
	kept := []struct {
		gvr             schema.GroupVersionResource
		namespace, name string
	}{
		{deployments, "shop", "app-dex-authenticator"},
		{budgets, "shop", "app-dex-authenticator"},
		{autoscalers, "shop", "app-dex-authenticator"},
		{networkPolicies, "shop", "app-dex-authenticator"},
		{secretGVR, "shop", "dex-authenticator-app"},
		{secretGVR, "shop", "registry-dex-authenticator"},
	}

	c := newHandoverFakeClient(
		handoverObject(oauth2ClientGVR, "OAuth2Client", dexNamespace, clientName, map[string]string{
			"app": "dex", "heritage": "deckhouse", "module": "user-authn", managedBy: "Helm",
			"deckhouse.io/dex-authenticator-for": "app", "deckhouse.io/dex-authenticator-namespace": "shop",
		}, chartAnnotations),
		handoverObject(services, "Service", "shop", "app-dex-authenticator", forApp, chartAnnotations),
		handoverObject(ingresses, "Ingress", "shop", "app-dex-authenticator", forApp, chartAnnotations),
		handoverObject(ingresses, "Ingress", "shop", "app-dex-authenticator-sign-out", forApp, chartAnnotations),
		handoverObject(httpRoutes, "HTTPRoute", "shop", "app-dex-authenticator", forApp, chartAnnotations),
		handoverObject(deployments, "Deployment", "shop", "app-dex-authenticator", forApp, chartAnnotations),
		handoverObject(budgets, "PodDisruptionBudget", "shop", "app-dex-authenticator", forApp, chartAnnotations),
		handoverObject(autoscalers, "VerticalPodAutoscaler", "shop", "app-dex-authenticator", forApp, chartAnnotations),
		handoverObject(networkPolicies, "NetworkPolicy", "shop", "app-dex-authenticator", forApp, chartAnnotations),
		handoverObject(secretGVR, "Secret", "shop", "dex-authenticator-app", chartLabels(map[string]string{"name": "credentials", "deckhouse.io/dex-authenticator-for": "app"}), chartAnnotations),
		handoverObject(secretGVR, "Secret", "shop", "registry-dex-authenticator", chartLabels(nil), chartAnnotations),

		// Not of the release: the same labels on an object somebody else wrote, and an application
		// of a user that happens to be called dex-authenticator.
		handoverObject(services, "Service", "other", "app-dex-authenticator", forApp, nil),
		handoverObject(deployments, "Deployment", "other", "dex-authenticator", map[string]string{"app": "dex-authenticator"}, chartAnnotations),
		// The dex client of a DexClient is not the client of a DexAuthenticator.
		handoverObject(oauth2ClientGVR, "OAuth2Client", dexNamespace, grafanaOAuth2Client, map[string]string{"app": "dex-client", "heritage": "deckhouse", "module": "user-authn"}, nil),
	)

	res, err := handover.Protect(context.Background(), c, dexAuthenticatorKinds)
	if err != nil {
		t.Fatalf("Protect: %v", err)
	}
	if want := len(handed) + len(kept); res.Owned != want || res.Stamped != want {
		t.Errorf("result = %+v, want %d owned and stamped", res, want)
	}

	// An object that is not of the release costs no request of its own: the Service and the
	// routes the controller makes for every DexAuthenticator carry the labels of the chart too.
	for _, action := range c.Actions() {
		if action.GetVerb() != "list" && action.GetNamespace() == "other" {
			t.Errorf("a request for an object that is not of the release: %s %s in %s", action.GetVerb(), action.GetResource().Resource, action.GetNamespace())
		}
	}
	c.ClearActions()

	// A second run finds everything protected and patches nothing.
	res, err = handover.Protect(context.Background(), c, dexAuthenticatorKinds)
	if err != nil {
		t.Fatalf("Protect again: %v", err)
	}
	if want := len(handed) + len(kept); res.Owned != want || res.Stamped != 0 {
		t.Errorf("the second run = %+v, want %d owned and none stamped", res, want)
	}
	for _, action := range c.Actions() {
		if action.GetVerb() == "patch" {
			t.Errorf("the second run patched %s in %s", action.GetResource().Resource, action.GetNamespace())
		}
	}

	labelOf := func(gvr schema.GroupVersionResource, namespace, name string) string {
		obj, err := c.Resource(gvr).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("get %s %s/%s: %v", gvr.Resource, namespace, name, err)
		}
		return obj.GetLabels()[managedBy]
	}
	for _, obj := range handed {
		if got := policyOf(t, c, obj.gvr, obj.namespace, obj.name); got != "keep" {
			t.Errorf("%s %s/%s policy = %q", obj.gvr.Resource, obj.namespace, obj.name, got)
		}
		if got := labelOf(obj.gvr, obj.namespace, obj.name); got != "user-authn-controller" {
			t.Errorf("%s %s/%s is not handed to the controller: %s = %q", obj.gvr.Resource, obj.namespace, obj.name, managedBy, got)
		}
	}
	for _, obj := range kept {
		if got := policyOf(t, c, obj.gvr, obj.namespace, obj.name); got != "keep" {
			t.Errorf("%s %s/%s policy = %q", obj.gvr.Resource, obj.namespace, obj.name, got)
		}
		// The controller only deletes these, which takes no label of its own.
		if got := labelOf(obj.gvr, obj.namespace, obj.name); got != "Helm" {
			t.Errorf("%s %s/%s: %s = %q, want it untouched", obj.gvr.Resource, obj.namespace, obj.name, managedBy, got)
		}
	}

	if got := policyOf(t, c, services, "other", "app-dex-authenticator"); got != "" {
		t.Errorf("the Service of somebody else: policy %q", got)
	}
	if got := labelOf(services, "other", "app-dex-authenticator"); got != "Helm" {
		t.Errorf("the Service of somebody else was handed to the controller: %q", got)
	}
	if got := policyOf(t, c, deployments, "other", "dex-authenticator"); got != "" {
		t.Errorf("a Deployment without the labels of the chart: policy %q", got)
	}

	// A second run finds everything protected and handed over.
	res, err = handover.Protect(context.Background(), c, dexAuthenticatorKinds)
	if err != nil || res.Stamped != 0 || res.Owned != len(handed)+len(kept) {
		t.Errorf("second Protect = %+v, %v", res, err)
	}

	// An object protected by an earlier version of the hook, without the label, gets it.
	withoutLabel := newHandoverFakeClient(handoverObject(services, "Service", "shop", "app-dex-authenticator", forApp, map[string]string{
		"meta.helm.sh/release-name": "user-authn", "helm.sh/resource-policy": "keep",
	}))
	if res, err := handover.Protect(context.Background(), withoutLabel, dexAuthenticatorKinds); err != nil || res.Stamped != 1 {
		t.Errorf("Protect of a kept object without the label = %+v, %v", res, err)
	}
}

// TestHandoverCostsNoRequestPerDexAuthenticatorObject: TestHandoverCostsNoRequestPerObject with the
// dex-authenticator of hundreds of DexAuthenticator objects on top of the DexClients. An object
// costs a request of its own only when it has to be stamped; the objects the controller has taken
// over already, and every later run, cost the lists alone.
func TestHandoverCostsNoRequestPerDexAuthenticatorObject(t *testing.T) {
	const (
		adoptedClients   = 300 // DexClients whose Secret and OAuth2Clients the controller owns already
		releaseClients   = 3   // DexClients whose objects the release still owns
		authenticators   = 100 // DexAuthenticators with the dex-authenticator of the old chart
		formerClients    = 40  // of them, with one more Dex client under the former ID
		perClient        = 3   // a Secret and two OAuth2Clients
		perAuthenticator = 9
	)
	var (
		services        = schema.GroupVersionResource{Version: "v1", Resource: "services"}
		ingresses       = schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"}
		deployments     = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
		budgets         = schema.GroupVersionResource{Group: "policy", Version: "v1", Resource: "poddisruptionbudgets"}
		autoscalers     = schema.GroupVersionResource{Group: "autoscaling.k8s.io", Version: "v1", Resource: "verticalpodautoscalers"}
		networkPolicies = schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"}
	)

	var objs []runtime.Object
	dexClient := func(i int, annotations map[string]string) {
		namespace, name := fmt.Sprintf("team-%d", i), fmt.Sprintf("app-%d", i)
		objs = append(objs,
			handoverObject(dexClientGVR, "DexClient", namespace, name, nil, nil),
			handoverObject(secretGVR, "Secret", namespace, dexClientSecretPrefix+name, nil, annotations),
			handoverObject(oauth2ClientGVR, "OAuth2Client", dexNamespace, encoding.ToFnvLikeDex(fmt.Sprintf("dex-client-%s@%s", name, namespace)), nil, annotations),
			handoverObject(oauth2ClientGVR, "OAuth2Client", dexNamespace, encoding.ToFnvLikeDex(fmt.Sprintf("dex-client-%s:%s", name, namespace)), nil, annotations),
			// Other Secrets of the namespace are in the same list and cost nothing either.
			handoverObject(secretGVR, "Secret", namespace, "something-else", nil, nil),
		)
	}
	for i := range adoptedClients {
		dexClient(i, nil)
	}
	for i := range releaseClients {
		dexClient(adoptedClients+i, chartAnnotations)
	}
	objs = append(objs, handoverObject(oauth2ClientGVR, "OAuth2Client", dexNamespace, kubernetesClientName, nil, nil))

	chart := map[string]string{"app": "dex-authenticator", "heritage": "deckhouse", "module": "user-authn"}
	for i := range authenticators {
		namespace, name := fmt.Sprintf("shop-%d", i), "app-dex-authenticator"
		if i < formerClients {
			objs = append(objs, handoverObject(oauth2ClientGVR, "OAuth2Client", dexNamespace, encoding.ToFnvLikeDex(fmt.Sprintf("app-%s-dex-authenticator", namespace)), map[string]string{
				"app": "dex", "heritage": "deckhouse", "module": "user-authn",
			}, chartAnnotations))
		}
		objs = append(objs,
			handoverObject(dexAuthenticatorGVR, "DexAuthenticator", namespace, "app", nil, nil),
			handoverObject(oauth2ClientGVR, "OAuth2Client", dexNamespace, encoding.ToFnvLikeDex(fmt.Sprintf("dex-authenticator-app@%s", namespace)), map[string]string{
				"app": "dex", "heritage": "deckhouse", "module": "user-authn", "deckhouse.io/dex-authenticator-for": "app",
			}, chartAnnotations),
			handoverObject(services, "Service", namespace, name, chart, chartAnnotations),
			handoverObject(ingresses, "Ingress", namespace, name, chart, chartAnnotations),
			handoverObject(deployments, "Deployment", namespace, name, chart, chartAnnotations),
			handoverObject(budgets, "PodDisruptionBudget", namespace, name, chart, chartAnnotations),
			handoverObject(autoscalers, "VerticalPodAutoscaler", namespace, name, chart, chartAnnotations),
			handoverObject(networkPolicies, "NetworkPolicy", namespace, name, chart, chartAnnotations),
			handoverObject(secretGVR, "Secret", namespace, "dex-authenticator-app", chart, chartAnnotations),
			handoverObject(secretGVR, "Secret", namespace, "registry-dex-authenticator", chart, chartAnnotations),
		)
	}
	c := newHandoverFakeClient(objs...)

	// One list per kind, the DexClients once more for each of the two kinds made from them and
	// the DexAuthenticators for the kind made from them: the number of objects does not change it.
	lists := len(handoverKinds) + 3
	stamps := releaseClients*perClient + authenticators*perAuthenticator + formerClients

	res, err := handover.Protect(context.Background(), c, handoverKinds)
	if err != nil {
		t.Fatalf("Protect: %v", err)
	}
	if res.Owned != stamps || res.Stamped != stamps {
		t.Errorf("result = %+v, want %d owned and stamped", res, stamps)
	}
	calls := apiCalls(c)
	if calls["get"] != 0 || calls["list"] != lists || calls["patch"] != stamps || len(calls) != 2 {
		t.Errorf("the first run made %v, want %d lists, %d patches and nothing else", calls, lists, stamps)
	}

	// Everything is stamped: a later run is the lists alone.
	c.ClearActions()
	res, err = handover.Protect(context.Background(), c, handoverKinds)
	if err != nil || res.Owned != stamps || res.Stamped != 0 {
		t.Errorf("the second run = %+v, %v", res, err)
	}
	if calls := apiCalls(c); calls["list"] != lists || len(calls) != 1 {
		t.Errorf("the second run made %v, want %d lists and nothing else", calls, lists)
	}
}

// TestHandoverKeepsTheFormerDexClientOfADexAuthenticator: the pods of the dex-authenticator of an
// object that was rendered while its Dex client had the ID <name>-<namespace>-dex-authenticator
// sign users in with that client until the Service of the object points at an instance. The
// release that no longer renders the client must not prune it, and the controller, which deletes
// it then, gets it handed over.
func TestHandoverKeepsTheFormerDexClientOfADexAuthenticator(t *testing.T) {
	const managedBy = "app.kubernetes.io/managed-by"
	formerName := func(namespace, name string) string {
		return encoding.ToFnvLikeDex(fmt.Sprintf("%s-%s-dex-authenticator", name, namespace))
	}
	chart := func(app string) map[string]string {
		return map[string]string{"app": app, "heritage": "deckhouse", "module": "user-authn", managedBy: "Helm"}
	}

	var (
		former = formerName("shop", "app")
		// The DexAuthenticator kubeconfig in foo and the kubeconfig of the cluster
		// foo-dex-authenticator give one ID, kubeconfig-foo-dex-authenticator: the client is the
		// one of the kubeconfig, which the chart still renders.
		kubeconfig = formerName("foo", "kubeconfig")
		// Its DexAuthenticator is gone: the release prunes it.
		orphan = formerName("gone", "app")
		// A client somebody made under the name: not of the release.
		foreign = formerName("shop", "manual")
		// The client under the ID of today carries the name of its DexAuthenticator.
		current = encoding.ToFnvLikeDex("dex-authenticator-next@shop")
	)
	c := newHandoverFakeClient(
		handoverObject(dexAuthenticatorGVR, "DexAuthenticator", "shop", "app", nil, nil),
		handoverObject(dexAuthenticatorGVR, "DexAuthenticator", "shop", "manual", nil, nil),
		handoverObject(dexAuthenticatorGVR, "DexAuthenticator", "shop", "next", nil, nil),
		handoverObject(dexAuthenticatorGVR, "DexAuthenticator", "foo", "kubeconfig", nil, nil),
		handoverObject(oauth2ClientGVR, "OAuth2Client", dexNamespace, former, chart("dex"), chartAnnotations),
		handoverObject(oauth2ClientGVR, "OAuth2Client", dexNamespace, kubeconfig, chart("kubernetes-dex-clients"), chartAnnotations),
		handoverObject(oauth2ClientGVR, "OAuth2Client", dexNamespace, orphan, chart("dex"), chartAnnotations),
		handoverObject(oauth2ClientGVR, "OAuth2Client", dexNamespace, foreign, map[string]string{"app": "dex", "heritage": "deckhouse", "module": "user-authn"}, nil),
		handoverObject(oauth2ClientGVR, "OAuth2Client", dexNamespace, kubernetesClientName, chart("dex"), chartAnnotations),
		handoverObject(oauth2ClientGVR, "OAuth2Client", dexNamespace, current, map[string]string{
			"app": "dex", "heritage": "deckhouse", "module": "user-authn", managedBy: "Helm", "deckhouse.io/dex-authenticator-for": "next",
		}, chartAnnotations),
	)

	res, err := handover.Protect(context.Background(), c, dexAuthenticatorKinds)
	if err != nil {
		t.Fatalf("Protect: %v", err)
	}
	// The former client and the one under the ID of today.
	if res.Owned != 2 || res.Stamped != 2 {
		t.Errorf("result = %+v, want 2 owned and stamped", res)
	}
	if calls := apiCalls(c); calls["get"] != 0 || calls["patch"] != 2 {
		t.Errorf("the run made %v, want 2 patches and no get", calls)
	}

	labelOf := func(name string) string {
		obj, err := c.Resource(oauth2ClientGVR).Namespace(dexNamespace).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("get OAuth2Client %s: %v", name, err)
		}
		return obj.GetLabels()[managedBy]
	}
	for name, client := range map[string]string{"the former client": former, "the client under the ID of today": current} {
		if got := policyOf(t, c, oauth2ClientGVR, dexNamespace, client); got != "keep" {
			t.Errorf("%s: policy = %q, want keep", name, got)
		}
		if got := labelOf(client); got != "user-authn-controller" {
			t.Errorf("%s is not handed to the controller: %s = %q", name, managedBy, got)
		}
	}
	for name, client := range map[string]string{
		"the client of a kubeconfig with the same ID":   kubeconfig,
		"the client of a DexAuthenticator that is gone": orphan,
		"the client that is not of the release":         foreign,
		"the kubernetes client":                         kubernetesClientName,
	} {
		if got := policyOf(t, c, oauth2ClientGVR, dexNamespace, client); got != "" {
			t.Errorf("%s: policy = %q, want it untouched", name, got)
		}
		if got := labelOf(client); got == "user-authn-controller" {
			t.Errorf("%s was handed to the controller", name)
		}
	}

	// A second run finds the clients protected and patches nothing.
	c.ClearActions()
	res, err = handover.Protect(context.Background(), c, dexAuthenticatorKinds)
	if err != nil || res.Owned != 2 || res.Stamped != 0 {
		t.Errorf("the second run = %+v, %v", res, err)
	}
	if calls := apiCalls(c); calls["patch"] != 0 || calls["get"] != 0 {
		t.Errorf("the second run made %v, want lists alone", calls)
	}
}
