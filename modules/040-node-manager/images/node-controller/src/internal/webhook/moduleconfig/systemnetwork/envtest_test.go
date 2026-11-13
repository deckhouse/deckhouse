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

package systemnetwork

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/deckhouse/node-controller/internal/testenv"
)

// The expression of matchConditions in templates/node-controller/webhook.yaml.
const matchExpression = `object.metadata.name == 'node-manager' &&
object.?spec.?settings.?fencing.?systemNetworkFallback.?systemNetworkName.orValue('') != '' &&
object.?spec.?settings.?fencing.?systemNetworkFallback.?systemNetworkName.orValue('') != null &&
object.?spec.?settings.?fencing.?systemNetworkFallback.?systemNetworkName.orValue('') !=
  (oldObject == null ? '' : oldObject.?spec.?settings.?fencing.?systemNetworkFallback.?systemNetworkName.orValue(''))
`

const (
	notFoundMessage     = "SystemNetwork `sysnet-b` not found"
	notInstalledMessage = "SystemNetwork is not installed (is the sdn module disabled?)"
)

var (
	moduleConfigGVK = schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "ModuleConfig"}
	crdTimeout      = 30 * time.Second
)

func webhookConfiguration() *admissionregistrationv1.ValidatingWebhookConfiguration {
	return &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: "node-controller-systemnetwork-envtest"},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{{
			Name:                    "systemnetwork.validation.node-controller.deckhouse.io",
			AdmissionReviewVersions: []string{"v1"},
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				// envtest joins host:port and path with a slash, so no leading slash here.
				Service: &admissionregistrationv1.ServiceReference{
					Name:      "node-controller-webhook",
					Namespace: "d8-cloud-instance-manager",
					Path:      ptr.To(strings.TrimPrefix(Path, "/")),
				},
			},
			FailurePolicy:  ptr.To(admissionregistrationv1.Fail),
			MatchPolicy:    ptr.To(admissionregistrationv1.Equivalent),
			SideEffects:    ptr.To(admissionregistrationv1.SideEffectClassNone),
			TimeoutSeconds: ptr.To(int32(10)),
			MatchConditions: []admissionregistrationv1.MatchCondition{
				{Name: "system-network-name-changed", Expression: matchExpression},
			},
			Rules: []admissionregistrationv1.RuleWithOperations{{
				Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{"deckhouse.io"},
					APIVersions: []string{"v1alpha1"},
					Resources:   []string{"moduleconfigs"},
					Scope:       ptr.To(admissionregistrationv1.ClusterScope),
				},
			}},
		}},
	}
}

// crd is a cluster-scoped CRD of the group of SystemNetwork that holds any object.
func crd(kind string) *apiextensionsv1.CustomResourceDefinition {
	plural := strings.ToLower(kind) + "s"
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: plural + ".network.deckhouse.io"},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "network.deckhouse.io",
			Scope: apiextensionsv1.ClusterScoped,
			Names: apiextensionsv1.CustomResourceDefinitionNames{Kind: kind, ListKind: kind + "List", Plural: plural, Singular: strings.ToLower(kind)},
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{
				Name: "v1alpha1", Served: true, Storage: true,
				Schema: &apiextensionsv1.CustomResourceValidation{OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
					Type: "object", XPreserveUnknownFields: ptr.To(true),
				}},
			}},
		},
	}
}

func systemNetwork(name string) *unstructured.Unstructured {
	network := &unstructured.Unstructured{}
	network.SetGroupVersionKind(schema.GroupVersionKind{Group: "network.deckhouse.io", Version: "v1alpha1", Kind: "SystemNetwork"})
	network.SetName(name)
	return network
}

func moduleConfigWith(t *testing.T, name, spec string) *unstructured.Unstructured {
	t.Helper()
	moduleConfig := &unstructured.Unstructured{Object: map[string]any{}}
	if err := json.Unmarshal([]byte(spec), &moduleConfig.Object); err != nil {
		t.Fatal(err)
	}
	moduleConfig.SetGroupVersionKind(moduleConfigGVK)
	moduleConfig.SetName(name)
	return moduleConfig
}

func TestWebhookEnvtest(t *testing.T) {
	if !testenv.AssetsAvailable() {
		t.Skip("envtest assets not found; run `make envtest` or set KUBEBUILDER_ASSETS")
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := apiextensionsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	env, cfg, c, err := testenv.StartWithWebhooks(scheme, []*admissionregistrationv1.ValidatingWebhookConfiguration{webhookConfiguration()})
	if err != nil {
		t.Fatalf("start envtest: %v", err)
	}
	defer func() {
		if err := env.Stop(); err != nil {
			t.Errorf("stop envtest: %v", err)
		}
	}()

	mgr, err := testenv.NewManagerWithWebhooks(ctx, cfg, scheme, &env.WebhookInstallOptions)
	if err != nil {
		t.Fatal(err)
	}
	// The same handler as Register adds, with a counter of the calls.
	var calls atomic.Int64
	handler := &validator{reader: mgr.GetAPIReader()}
	mgr.GetWebhookServer().Register(Path, &webhook.Admission{
		Handler: admission.HandlerFunc(func(ctx context.Context, req admission.Request) admission.Response {
			calls.Add(1)
			return handler.Handle(ctx, req)
		}),
	})
	go func() { _ = mgr.Start(ctx) }()
	if err := testenv.WaitForWebhookServer(ctx, &env.WebhookInstallOptions); err != nil {
		t.Fatal(err)
	}

	createCRD := func(t *testing.T, kind string) *apiextensionsv1.CustomResourceDefinition {
		t.Helper()
		object := crd(kind)
		if err := c.Create(ctx, object); err != nil {
			t.Fatal(err)
		}
		// The API serves the new resource a moment after the CRD exists.
		waitFor(t, "CRD "+object.Name+" is served", func() bool {
			list := &unstructured.UnstructuredList{}
			list.SetGroupVersionKind(schema.GroupVersionKind{Group: "network.deckhouse.io", Version: "v1alpha1", Kind: kind + "List"})
			return c.List(ctx, list) == nil
		})
		return object
	}
	// create and update return the calls of the webhook they caused and the error of the API.
	// A ModuleConfig lives until the end of the test: the stages below build on each other.
	root := t
	create := func(t *testing.T, name, spec string) (int64, error) {
		t.Helper()
		before := calls.Load()
		moduleConfig := moduleConfigWith(t, name, spec)
		err := c.Create(ctx, moduleConfig)
		if err == nil {
			root.Cleanup(func() { _ = c.Delete(ctx, moduleConfig) })
		}
		return calls.Load() - before, err
	}
	update := func(t *testing.T, name, spec string) (int64, error) {
		t.Helper()
		before := calls.Load()
		current := moduleConfigWith(t, name, "{}")
		if err := c.Get(ctx, client.ObjectKey{Name: name}, current); err != nil {
			t.Fatal(err)
		}
		next := moduleConfigWith(t, name, spec)
		next.SetResourceVersion(current.GetResourceVersion())
		err := c.Update(ctx, next)
		return calls.Load() - before, err
	}
	withNetwork := func(name string) string {
		return `{"spec":{"settings":{"fencing":{"systemNetworkFallback":{"systemNetworkName":"` + name + `"}}}}}`
	}
	requireDenied := func(t *testing.T, err error, want string) {
		t.Helper()
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("error %v, want a denial with %q", err, want)
		}
	}
	requireAllowed := func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("denied: %v", err)
		}
	}

	t.Run("the group of SystemNetwork is not served", func(t *testing.T) {
		called, err := create(t, "node-manager", withNetwork("sysnet-a"))
		requireDenied(t, err, notInstalledMessage)
		if called != 1 {
			t.Fatalf("the webhook was called %d times", called)
		}
	})

	t.Run("the group is served, the kind is not", func(t *testing.T) {
		decoy := createCRD(t, "Decoy")
		defer func() { _ = c.Delete(ctx, decoy) }()

		_, err := create(t, "node-manager", withNetwork("sysnet-a"))
		requireDenied(t, err, notInstalledMessage)
	})

	// The CRD appears after node-controller started, and the next check sees it.
	var systemNetworkCRD *apiextensionsv1.CustomResourceDefinition
	t.Run("the CRD appears while node-controller works", func(t *testing.T) {
		systemNetworkCRD = createCRD(t, "SystemNetwork")
		if err := c.Create(ctx, systemNetwork("sysnet-a")); err != nil {
			t.Fatal(err)
		}

		called, err := create(t, "node-manager", withNetwork("sysnet-a"))
		requireAllowed(t, err)
		if called != 1 {
			t.Fatalf("the webhook was called %d times", called)
		}
	})

	t.Run("the network does not exist", func(t *testing.T) {
		_, err := update(t, "node-manager", withNetwork("sysnet-b"))
		requireDenied(t, err, notFoundMessage)
	})

	t.Run("the value is not a string", func(t *testing.T) {
		_, err := update(t, "node-manager", `{"spec":{"settings":{"fencing":{"systemNetworkFallback":{"systemNetworkName":5}}}}}`)
		requireDenied(t, err, "expected a string")
	})

	t.Run("the network is deleted, the name stays", func(t *testing.T) {
		if err := c.Delete(ctx, systemNetwork("sysnet-a")); err != nil {
			t.Fatal(err)
		}

		for _, step := range []struct{ name, spec string }{
			{"another setting", `{"spec":{"settings":{"mcmEmergencyBrake":true,"fencing":{"systemNetworkFallback":{"systemNetworkName":"sysnet-a"}}}}}`},
			{"the same value", withNetwork("sysnet-a")},
			{"the value is removed", withNetwork("")},
		} {
			called, err := update(t, "node-manager", step.spec)
			if err != nil || called != 0 {
				t.Fatalf("%s: error %v, calls %d, want none of both", step.name, err, called)
			}
		}
	})

	t.Run("the webhook is not called", func(t *testing.T) {
		for name, spec := range map[string]string{
			"no settings":                `{}`,
			"settings is null":           `{"spec":{"settings":null}}`,
			"settings is empty":          `{"spec":{"settings":{}}}`,
			"the name is empty":          withNetwork(""),
			"fencing is a string":        `{"spec":{"settings":{"fencing":"text"}}}`,
			"systemNetworkFallback null": `{"spec":{"settings":{"fencing":{"systemNetworkFallback":null}}}}`,
			"the name of another module": withNetwork("sysnet-b"),
		} {
			t.Run(name, func(t *testing.T) {
				moduleConfigName := "node-manager"
				if name == "the name of another module" {
					moduleConfigName = "other"
				}
				// A spec of an earlier case could keep the object, so each case starts from a new one.
				_ = c.Delete(ctx, moduleConfigWith(t, moduleConfigName, "{}"))
				called, err := create(t, moduleConfigName, spec)
				if err != nil || called != 0 {
					t.Fatalf("error %v, calls %d, want none of both", err, called)
				}
			})
		}
	})

	// A null name is not a new value, so the webhook is not called.
	t.Run("the name is null", func(t *testing.T) {
		_ = c.Delete(ctx, moduleConfigWith(t, "node-manager", "{}"))
		called, err := create(t, "node-manager", `{"spec":{"settings":{"fencing":{"systemNetworkFallback":{"systemNetworkName":null}}}}}`)
		if err != nil || called != 0 {
			t.Fatalf("error %v, calls %d, want none of both", err, called)
		}
	})

	t.Run("the name changes to null", func(t *testing.T) {
		_ = c.Delete(ctx, moduleConfigWith(t, "node-manager", "{}"))
		if err := c.Create(ctx, systemNetwork("sysnet-a")); err != nil {
			t.Fatal(err)
		}
		if _, err := create(t, "node-manager", withNetwork("sysnet-a")); err != nil {
			t.Fatal(err)
		}
		called, err := update(t, "node-manager", `{"spec":{"settings":{"fencing":{"systemNetworkFallback":{"systemNetworkName":null}}}}}`)
		if err != nil || called != 0 {
			t.Fatalf("error %v, calls %d, want none of both", err, called)
		}
	})

	t.Run("a list instead of an object", func(t *testing.T) {
		_ = c.Delete(ctx, moduleConfigWith(t, "node-manager", "{}"))
		_, err := create(t, "node-manager", `{"spec":{"settings":{"fencing":{"systemNetworkFallback":["sysnet-a"]}}}}`)
		if err == nil || apierrors.IsNotFound(err) {
			t.Fatalf("error %v, want a denial", err)
		}
	})

	t.Run("the CRD is deleted after the first read", func(t *testing.T) {
		if err := c.Delete(ctx, systemNetworkCRD); err != nil {
			t.Fatal(err)
		}
		_ = c.Delete(ctx, moduleConfigWith(t, "node-manager", "{}"))
		create(t, "node-manager", `{}`)

		waitFor(t, "the API stops to serve SystemNetwork", func() bool {
			list := &unstructured.UnstructuredList{}
			list.SetGroupVersionKind(schema.GroupVersionKind{Group: "network.deckhouse.io", Version: "v1alpha1", Kind: "SystemNetworkList"})
			return c.List(ctx, list) != nil
		})
		_, err := update(t, "node-manager", withNetwork("sysnet-b"))
		requireDenied(t, err, notInstalledMessage)
	})
}

func waitFor(t *testing.T, what string, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(crdTimeout)
	for !done() {
		if time.Now().After(deadline) {
			t.Fatalf("timeout: %s", what)
		}
		time.Sleep(200 * time.Millisecond)
	}
}
