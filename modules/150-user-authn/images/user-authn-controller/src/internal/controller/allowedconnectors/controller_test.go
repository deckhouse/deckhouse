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

package allowedconnectors

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
	"user-authn-controller/internal/revocationfeed"
)

type call struct {
	clientID string
	allowed  []string
}

type fakeRevoker struct {
	calls []call
	err   error
}

func (f *fakeRevoker) DeleteClientTokensOutside(_ context.Context, clientID string, allowed []string) (int, error) {
	f.calls = append(f.calls, call{clientID: clientID, allowed: allowed})
	return 0, f.err
}

func oauth2Client(name, id string, allowed ...string) *unstructured.Unstructured {
	obj := controller.Object(controller.OAuth2ClientGVK)
	obj.SetNamespace(naming.DexNamespace)
	obj.SetName(name)
	obj.Object["id"] = id
	if allowed != nil {
		items := make([]any, 0, len(allowed))
		for _, a := range allowed {
			items = append(items, a)
		}
		obj.Object["allowedConnectors"] = items
	}
	return obj
}

func reconcileOf(t *testing.T, revoker *fakeRevoker, name string, objs ...client.Object) error {
	t.Helper()
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{controller.OAuth2ClientGVK.GroupVersion()})
	mapper.Add(controller.OAuth2ClientGVK, meta.RESTScopeNamespace)
	c := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).WithRESTMapper(mapper).WithObjects(objs...).Build()
	_, err := New(c, revoker).Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: naming.DexNamespace, Name: name}})
	return err
}

func TestReconcile_DeletesTheTokensOutsideTheList(t *testing.T) {
	t.Parallel()

	revoker := &fakeRevoker{}
	if err := reconcileOf(t, revoker, "console", oauth2Client("console", "console", "local", "ldap")); err != nil {
		t.Fatal(err)
	}
	if len(revoker.calls) != 1 || revoker.calls[0].clientID != "console" || !slices.Equal(revoker.calls[0].allowed, []string{"local", "ldap"}) {
		t.Fatalf("calls %+v", revoker.calls)
	}
}

// The name of the object is a hash of the id: the controller goes by the id.
func TestReconcile_GoesByTheIDOfTheClient(t *testing.T) {
	t.Parallel()

	revoker := &fakeRevoker{}
	if err := reconcileOf(t, revoker, "mfrggzdf", oauth2Client("mfrggzdf", "dex-client-app@ns", "ldap")); err != nil {
		t.Fatal(err)
	}
	if len(revoker.calls) != 1 || revoker.calls[0].clientID != "dex-client-app@ns" {
		t.Fatalf("calls %+v", revoker.calls)
	}
}

func TestReconcile_NothingToDoWithoutAList(t *testing.T) {
	t.Parallel()

	revoker := &fakeRevoker{}
	if err := reconcileOf(t, revoker, "console", oauth2Client("console", "console")); err != nil {
		t.Fatal(err)
	}
	if len(revoker.calls) != 0 {
		t.Fatalf("calls %+v", revoker.calls)
	}
}

func TestReconcile_ClientGone(t *testing.T) {
	t.Parallel()

	revoker := &fakeRevoker{}
	if err := reconcileOf(t, revoker, "console"); err != nil {
		t.Fatal(err)
	}
	if len(revoker.calls) != 0 {
		t.Fatalf("calls %+v", revoker.calls)
	}
}

func TestReconcile_ReportsAFailure(t *testing.T) {
	t.Parallel()

	revoker := &fakeRevoker{err: errors.New("the API server is away")}
	if err := reconcileOf(t, revoker, "console", oauth2Client("console", "console", "local")); err == nil {
		t.Fatal("the failure was not reported")
	}
}

// dexClientJSON is an OAuth2Client as the API server serves it for a DexClient: Dex writes the Client of its kubernetes
// storage with these field names.
const dexClientJSON = `{
  "apiVersion": "dex.coreos.com/v1",
  "kind": "OAuth2Client",
  "metadata": {"name": "mfrggzdf", "namespace": "d8-user-authn", "resourceVersion": "7", "uid": "3f1c"},
  "id": "dex-client-app@ns",
  "secret": "dh-test-secret-0123456789abcdef",
  "name": "app",
  "redirectURIs": ["https://app.example.com/callback"],
  "public": false,
  "allowedConnectors": ["local", "ldap"],
  "ssoSharedWith": []
}`

func clientFromJSON(t *testing.T, raw string) *unstructured.Unstructured {
	t.Helper()
	obj := &unstructured.Unstructured{}
	if err := json.Unmarshal([]byte(raw), &obj.Object); err != nil {
		t.Fatal(err)
	}
	return obj
}

func newFakeClient(objs ...client.Object) client.Client {
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{controller.OAuth2ClientGVK.GroupVersion()})
	mapper.Add(controller.OAuth2ClientGVK, meta.RESTScopeNamespace)
	return fake.NewClientBuilder().WithScheme(runtime.NewScheme()).WithRESTMapper(mapper).WithObjects(objs...).Build()
}

// fakeManager serves what the manager does: a client whose cache holds the OAuth2Clients as the controller caches them
// (TransformClient), and an API reader, which here refuses every read of an OAuth2Client: the controller has no right
// to get them.
type fakeManager struct {
	cached client.Client
	api    client.Reader
}

func (f fakeManager) GetClient() client.Client    { return f.cached }
func (f fakeManager) GetAPIReader() client.Reader { return f.api }

// forbiddenReader fails the test on a read.
type forbiddenReader struct{ t *testing.T }

func (f forbiddenReader) Get(_ context.Context, key client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
	f.t.Errorf("the controller got %s from the API server", key)
	return errors.New("forbidden")
}

func (f forbiddenReader) List(_ context.Context, _ client.ObjectList, _ ...client.ListOption) error {
	f.t.Error("the controller listed objects from the API server")
	return errors.New("forbidden")
}

// The controller works on the OAuth2Client the cache holds after TransformClient, the id and allowedConnectors of it,
// and never goes to the API server for it.
func TestReconcile_ReadsTheClientFromTheCache(t *testing.T) {
	t.Parallel()

	full := clientFromJSON(t, dexClientJSON)
	reduced, err := revocationfeed.TransformClient(full)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reduced.(*unstructured.Unstructured).Object["secret"]; ok {
		t.Fatal("the cache holds the secret")
	}

	revoker := &fakeRevoker{}
	manager := fakeManager{cached: newFakeClient(reduced.(*unstructured.Unstructured)), api: forbiddenReader{t}}
	_, err = newReconciler(manager, revoker).Reconcile(t.Context(),
		reconcile.Request{NamespacedName: types.NamespacedName{Namespace: naming.DexNamespace, Name: "mfrggzdf"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(revoker.calls) != 1 || revoker.calls[0].clientID != "dex-client-app@ns" || !slices.Equal(revoker.calls[0].allowed, []string{"local", "ldap"}) {
		t.Fatalf("calls %+v", revoker.calls)
	}
}
