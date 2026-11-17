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

package dexauthenticator

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authn-controller/internal/adoption"
	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

// fakeInstances is the table the pods of the instances serve, as the test sets it.
type fakeInstances struct {
	generation int64
	failed     []string
	err        error
}

func (f *fakeInstances) Loaded(context.Context, string) (LoadStatus, error) {
	return LoadStatus{Generation: f.generation, Failed: f.failed}, f.err
}

// testEnv is the two reconcilers over one fake cluster.
type testEnv struct {
	t         *testing.T
	client    client.Client
	tables    *TableReconciler
	objects   *Reconciler
	instances *fakeInstances
	recorder  *events.FakeRecorder
	announced chan event.GenericEvent
	now       time.Time
}

func newFakeClient(t *testing.T, objs ...client.Object) client.WithWatch {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{
		{Group: "deckhouse.io", Version: "v1"}, {Group: "dex.coreos.com", Version: "v1"},
		{Group: gatewayGroup, Version: "v1"}, corev1.SchemeGroupVersion, networkingv1.SchemeGroupVersion,
	})
	mapper.Add(controller.DexAuthenticatorGVK, meta.RESTScopeNamespace)
	mapper.Add(controller.OAuth2ClientGVK, meta.RESTScopeNamespace)
	mapper.Add(HTTPRouteGVK, meta.RESTScopeNamespace)
	mapper.Add(ReferenceGrantGVK, meta.RESTScopeNamespace)
	for _, kind := range []string{"Secret", "Service", "ConfigMap", "Pod"} {
		mapper.Add(corev1.SchemeGroupVersion.WithKind(kind), meta.RESTScopeNamespace)
	}
	mapper.Add(networkingv1.SchemeGroupVersion.WithKind("Ingress"), meta.RESTScopeNamespace)

	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithRESTMapper(mapper).
		WithStatusSubresource(controller.Object(controller.DexAuthenticatorGVK)).
		WithObjects(objs...).
		Build()
}

func settingsConfigMap(t *testing.T, settings Settings) *corev1.ConfigMap {
	t.Helper()

	raw, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: SettingsConfigMapName, Namespace: naming.DexNamespace},
		Data:       map[string]string{SettingsKey: string(raw)},
	}
}

// newTestEnv returns the reconcilers over a cluster with the settings and objs. The instances
// have loaded nothing.
func newTestEnv(t *testing.T, settings Settings, gatewayAPI bool, objs ...client.Object) *testEnv {
	t.Helper()

	return newTestEnvOver(t, newFakeClient(t, append([]client.Object{settingsConfigMap(t, settings)}, objs...)...), gatewayAPI)
}

// newTestEnvOver returns the reconcilers over the cluster behind c.
func newTestEnvOver(t *testing.T, c client.Client, gatewayAPI bool) *testEnv {
	t.Helper()

	env := &testEnv{
		t:         t,
		client:    c,
		instances: &fakeInstances{},
		recorder:  events.NewFakeRecorder(64),
		announced: make(chan event.GenericEvent, announcements),
		now:       testNow,
	}
	state := NewState()
	env.tables = NewTableReconciler(c, c, logr.Discard(), state, env.announced, nil, gatewayAPI)
	env.tables.now = func() time.Time { return env.now }
	env.objects = New(c, c, env.recorder, logr.Discard(), state, env.instances, gatewayAPI)
	return env
}

// buildTables runs the table reconciler and returns the objects it announced.
func (e *testEnv) buildTables() []types.NamespacedName {
	e.t.Helper()

	if _, err := e.tables.Reconcile(e.t.Context(), tablesRequest); err != nil {
		e.t.Fatalf("table Reconcile: %v", err)
	}
	var keys []types.NamespacedName
	for {
		select {
		case announced := <-e.announced:
			keys = append(keys, client.ObjectKeyFromObject(announced.Object))
		default:
			return keys
		}
	}
}

// reconcile runs the reconciler of the objects of key.
func (e *testEnv) reconcile(key types.NamespacedName) (reconcile.Result, error) {
	e.t.Helper()
	return e.objects.Reconcile(e.t.Context(), reconcile.Request{NamespacedName: key})
}

// mustReconcile runs the reconciler of the objects of key and fails the test on an error.
func (e *testEnv) mustReconcile(key types.NamespacedName) reconcile.Result {
	e.t.Helper()

	result, err := e.reconcile(key)
	if err != nil {
		e.t.Fatalf("Reconcile %s: %v", key, err)
	}
	return result
}

// setSettings replaces the settings of the module.
func (e *testEnv) setSettings(settings Settings) {
	e.t.Helper()

	current := &corev1.ConfigMap{}
	e.exists(current, naming.DexNamespace, SettingsConfigMapName)
	current.Data = settingsConfigMap(e.t, settings).Data
	if err := e.client.Update(e.t.Context(), current); err != nil {
		e.t.Fatal(err)
	}
}

// load makes the pods of the instances serve the tables that are written now.
func (e *testEnv) load() {
	e.t.Helper()

	*e.instances = fakeInstances{}
	for _, instance := range naming.AuthenticatorInstances() {
		e.instances.generation = max(e.instances.generation, e.table(instance).file.Generation)
	}
}

// serve brings the object key to the served state: its record is written and loaded, its objects made.
func (e *testEnv) serve(key types.NamespacedName) {
	e.t.Helper()

	e.buildTables()
	e.load()
	e.mustReconcile(key)
	if got := e.condition(key); got.Status != metav1.ConditionTrue {
		e.t.Fatalf("%s is not served: %+v", key, got)
	}
}

func (e *testEnv) table(instance string) table {
	e.t.Helper()

	secret := &corev1.Secret{}
	if err := e.client.Get(e.t.Context(), types.NamespacedName{Namespace: naming.DexNamespace, Name: TableSecretName(instance)}, secret); err != nil {
		e.t.Fatalf("get the table of %s: %v", instance, err)
	}
	parsed, err := parseTable(secret.Data)
	if err != nil {
		e.t.Fatalf("parse the table of %s: %v", instance, err)
	}
	return parsed
}

func (e *testEnv) condition(key types.NamespacedName) metav1.Condition {
	e.t.Helper()

	obj := controller.Object(controller.DexAuthenticatorGVK)
	if err := e.client.Get(e.t.Context(), key, obj); err != nil {
		e.t.Fatalf("get DexAuthenticator %s: %v", key, err)
	}
	if found := meta.FindStatusCondition(controller.ReadConditions(obj), ConditionServed); found != nil {
		return *found
	}
	return metav1.Condition{}
}

// exists reports whether the object named by obj and key is in the cluster, and reads it into obj.
func (e *testEnv) exists(obj client.Object, namespace, name string) bool {
	e.t.Helper()

	err := e.client.Get(e.t.Context(), types.NamespacedName{Namespace: namespace, Name: name}, obj)
	if err != nil && !apierrors.IsNotFound(err) {
		e.t.Fatalf("get %s/%s: %v", namespace, name, err)
	}
	return err == nil
}

func (e *testEnv) dexClient(name, namespace string) (*unstructured.Unstructured, bool) {
	e.t.Helper()

	obj := controller.Object(controller.OAuth2ClientGVK)
	found := e.exists(obj, naming.DexNamespace, naming.ToFnvLikeDex(naming.DexAuthenticatorClientID(name, namespace)))
	return obj, found
}

func (e *testEnv) events() []string {
	var out []string
	for {
		select {
		case recorded := <-e.recorder.Events:
			out = append(out, recorded)
		default:
			return out
		}
	}
}

// authenticatorObject is a DexAuthenticator of the stored version with the given domains, created
// age minutes after the start of the test time; every application has an Ingress class.
func authenticatorObject(namespace, name string, age int, domains ...string) *unstructured.Unstructured {
	obj := controller.Object(controller.DexAuthenticatorGVK)
	obj.SetNamespace(namespace)
	obj.SetName(name)
	obj.SetUID(types.UID("uid-" + namespace + "-" + name))
	obj.SetGeneration(1)
	obj.SetCreationTimestamp(metav1.NewTime(testNow.Add(time.Duration(age) * time.Minute)))

	spec := map[string]any{"applicationDomain": domains[0], "applicationIngressClassName": "nginx"}
	var additional []any
	for _, domain := range domains[1:] {
		additional = append(additional, map[string]any{"domain": domain, "ingressClassName": "nginx"})
	}
	if len(additional) > 0 {
		spec["additionalApplications"] = additional
	}
	obj.Object["spec"] = spec
	return obj
}

var (
	shopApp     = keyOf("shop", "app")
	shopService = "app-dex-authenticator"
)

func TestTableReconciler_WritesTheTableOfEveryInstance(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t, testSettings, false,
		authenticatorObject("shop", "app", 0, "app.example.com"),
		authenticatorObject("d8-console", "console", 1, "console.example.com"))

	announced := env.buildTables()
	if !slices.Equal(announced, []types.NamespacedName{keyOf("d8-console", "console"), shopApp}) {
		t.Errorf("announced %v, want both objects", announced)
	}

	user := env.table(naming.UserAuthenticatorInstance)
	if len(user.file.Clients) != 1 || user.file.Clients[0].key() != shopApp || user.file.Generation != testNow.Unix() {
		t.Errorf("the table of the user instance: %+v", user.file)
	}
	system := env.table(naming.SystemAuthenticatorInstance)
	if len(system.file.Clients) != 1 || system.file.Clients[0].Name != "console" {
		t.Errorf("the table of the system instance: %+v", system.file)
	}

	secret := &corev1.Secret{}
	if !env.exists(secret, naming.DexNamespace, "dex-authenticator-clients") {
		t.Fatal("no table Secret")
	}
	for key, want := range map[string]string{
		adoption.ManagedByLabel: adoption.ManagedByValue, "heritage": "deckhouse", "module": "user-authn",
		"app": "shared-dex-authenticator", "instance": "dex-authenticator",
	} {
		if got := secret.Labels[key]; got != want {
			t.Errorf("label %s of the table Secret = %q, want %q", key, got, want)
		}
	}

	// Nothing changed: nothing is written and nothing is announced.
	env.now = testNow.Add(time.Hour)
	before := secret.ResourceVersion
	if announced := env.buildTables(); len(announced) != 0 {
		t.Errorf("a pass that changed nothing announced %v", announced)
	}
	env.exists(secret, naming.DexNamespace, "dex-authenticator-clients")
	if secret.ResourceVersion != before {
		t.Error("a pass that changed nothing wrote the table")
	}

	// A change of one object: its table gets a new generation, and it alone is announced.
	obj := controller.Object(controller.DexAuthenticatorGVK)
	env.exists(obj, "shop", "app")
	if err := unstructured.SetNestedField(obj.Object, true, "spec", "sendAuthorizationHeader"); err != nil {
		t.Fatal(err)
	}
	if err := env.client.Update(t.Context(), obj); err != nil {
		t.Fatal(err)
	}
	if announced := env.buildTables(); !slices.Equal(announced, []types.NamespacedName{shopApp}) {
		t.Errorf("announced %v, want the changed object", announced)
	}
	user = env.table(naming.UserAuthenticatorInstance)
	if user.file.Generation != env.now.Unix() || !user.file.Clients[0].SendAuthorizationHeader {
		t.Errorf("the table after the change: %+v", user.file)
	}
	if got := env.table(naming.SystemAuthenticatorInstance).file.Generation; got != testNow.Unix() {
		t.Errorf("the table of the other instance got generation %d", got)
	}
}

func TestTableReconciler_DoesNothingWithoutTheSettings(t *testing.T) {
	t.Parallel()

	c := newFakeClient(t, authenticatorObject("shop", "app", 0, "app.example.com"))
	state := NewState()
	tables := NewTableReconciler(c, c, logr.Discard(), state, make(chan event.GenericEvent, 1), nil, false)
	if _, err := tables.Reconcile(t.Context(), tablesRequest); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	secrets := &corev1.SecretList{}
	if err := c.List(t.Context(), secrets); err != nil || len(secrets.Items) != 0 {
		t.Errorf("%d Secrets written without the settings, error %v", len(secrets.Items), err)
	}

	objects := New(c, c, nil, logr.Discard(), state, &fakeInstances{}, false)
	if result, err := objects.Reconcile(t.Context(), reconcile.Request{NamespacedName: shopApp}); err != nil || result != (reconcile.Result{}) {
		t.Errorf("Reconcile of the object: %+v, %v", result, err)
	}
}

// TestTableReconciler_ObjectMissingFromTheCache: an object the cache lost is looked for in the
// API before its record goes, since the record would come back with other secrets.
func TestTableReconciler_ObjectMissingFromTheCache(t *testing.T) {
	t.Parallel()

	obj := authenticatorObject("shop", "app", 0, "app.example.com")
	env := newTestEnv(t, testSettings, false, obj)
	env.buildTables()
	record := env.table(naming.UserAuthenticatorInstance).file.Clients[0]

	// The cache no longer has the object; the API does.
	cache := interceptor.NewClient(env.client.(client.WithWatch), interceptor.Funcs{
		List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			if list.GetObjectKind().GroupVersionKind().Kind == controller.DexAuthenticatorGVK.Kind+"List" {
				return nil
			}
			return c.List(ctx, list, opts...)
		},
	})
	tables := NewTableReconciler(cache, env.client, logr.Discard(), NewState(), make(chan event.GenericEvent, 8), nil, false)
	tables.now = func() time.Time { return testNow.Add(time.Hour) }

	result, err := tables.Reconcile(t.Context(), tablesRequest)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if result.RequeueAfter != laggingCacheRecheck {
		t.Errorf("requeue after %v, want %v", result.RequeueAfter, laggingCacheRecheck)
	}
	kept := env.table(naming.UserAuthenticatorInstance)
	if len(kept.file.Clients) != 1 || !recordsEqual(kept.file.Clients[0], record) || kept.file.Generation != testNow.Unix() {
		t.Errorf("the table changed: %+v", kept.file)
	}

	// The API does not have it either: the record goes.
	if err := env.client.Delete(t.Context(), obj); err != nil {
		t.Fatal(err)
	}
	if result, err := tables.Reconcile(t.Context(), tablesRequest); err != nil || result.RequeueAfter != 0 {
		t.Fatalf("Reconcile: %+v, %v", result, err)
	}
	if gone := env.table(naming.UserAuthenticatorInstance); len(gone.file.Clients) != 0 {
		t.Errorf("the record of the deleted object is still in the table: %+v", gone.file)
	}
}

// TestOrder_RouteAfterTheRecordIsLoaded: the Dex client is made at once, the Service and the
// routes once every pod of the instance serves the record.
func TestOrder_RouteAfterTheRecordIsLoaded(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t, testSettings, false, authenticatorObject("shop", "app", 0, "app.example.com"))

	// The table reconciler has not decided yet.
	if result := env.mustReconcile(shopApp); result.RequeueAfter != notReadyRecheck {
		t.Errorf("before the tables: requeue after %v, want %v", result.RequeueAfter, notReadyRecheck)
	}
	if _, found := env.dexClient("app", "shop"); found {
		t.Fatal("a Dex client before the table")
	}

	env.buildTables()
	record := env.table(naming.UserAuthenticatorInstance).file.Clients[0]

	for _, instances := range []fakeInstances{
		{generation: testNow.Unix() - 1},
		{generation: testNow.Unix(), err: errors.New("a pod does not answer")},
	} {
		*env.instances = instances
		if result := env.mustReconcile(shopApp); result.RequeueAfter != loadRecheck {
			t.Errorf("not loaded: requeue after %v, want %v", result.RequeueAfter, loadRecheck)
		}
		dexClient, found := env.dexClient("app", "shop")
		if !found {
			t.Fatal("no Dex client while the instance loads the record")
		}
		if secret, _, _ := unstructured.NestedString(dexClient.Object, "secret"); secret != record.ClientSecret {
			t.Errorf("the secret of the Dex client is not the one of the record")
		}
		if env.exists(&corev1.Service{}, "shop", shopService) || env.exists(&networkingv1.Ingress{}, "shop", shopService) {
			t.Fatal("a Service or a route before the instance loaded the record")
		}
		if got := env.condition(shopApp); got.Status != metav1.ConditionFalse || got.Reason != ReasonWaitingForInstance || got.ObservedGeneration != 1 {
			t.Errorf("condition = %+v", got)
		}
	}
	if recorded := env.events(); len(recorded) != 0 {
		t.Errorf("events while waiting for the instance: %v", recorded)
	}

	env.load()
	if result := env.mustReconcile(shopApp); result != (reconcile.Result{}) {
		t.Errorf("loaded: %+v", result)
	}
	service := &corev1.Service{}
	if !env.exists(service, "shop", shopService) {
		t.Fatal("no Service")
	}
	if service.Spec.Type != corev1.ServiceTypeExternalName || service.Spec.ExternalName != "dex-authenticator.d8-user-authn.svc.cluster.local" {
		t.Errorf("Service: type %s, externalName %s", service.Spec.Type, service.Spec.ExternalName)
	}
	if service.Labels[adoption.ManagedByLabel] != adoption.ManagedByValue || service.Labels[LabelFor] != "app" {
		t.Errorf("Service labels = %v", service.Labels)
	}
	ingress := &networkingv1.Ingress{}
	if !env.exists(ingress, "shop", shopService) {
		t.Fatal("no Ingress")
	}
	if ingress.Labels[adoption.ManagedByLabel] != adoption.ManagedByValue {
		t.Errorf("Ingress labels = %v", ingress.Labels)
	}
	if got := env.condition(shopApp); got.Status != metav1.ConditionTrue || got.Reason != ReasonServed {
		t.Errorf("condition = %+v", got)
	}

	// A second pass writes nothing.
	versions := func() []string {
		dexClient, _ := env.dexClient("app", "shop")
		obj := controller.Object(controller.DexAuthenticatorGVK)
		env.exists(obj, "shop", "app")
		env.exists(service, "shop", shopService)
		env.exists(ingress, "shop", shopService)
		return []string{dexClient.GetResourceVersion(), obj.GetResourceVersion(), service.ResourceVersion, ingress.ResourceVersion}
	}
	before := versions()
	env.mustReconcile(shopApp)
	if after := versions(); !slices.Equal(before, after) {
		t.Errorf("a second pass wrote something: resourceVersions %v, then %v", before, after)
	}
}

// TestOrder_RecordAfterTheRoutes: when an object is deleted, its routes go first and its record
// after them.
func TestOrder_RecordAfterTheRoutes(t *testing.T) {
	t.Parallel()

	obj := authenticatorObject("shop", "app", 0, "app.example.com")
	env := newTestEnv(t, testSettings, false, obj, authenticatorObject("shop", "other", 1, "other.example.com"))
	env.serve(shopApp)
	env.serve(keyOf("shop", "other"))

	if err := env.client.Delete(t.Context(), obj); err != nil {
		t.Fatal(err)
	}
	env.now = testNow.Add(time.Hour)

	// The routes are still there: the record stays.
	env.buildTables()
	if _, found := env.table(naming.UserAuthenticatorInstance).record(shopApp); !found {
		t.Fatal("the record went before the routes")
	}

	env.mustReconcile(shopApp)
	if env.exists(&networkingv1.Ingress{}, "shop", shopService) || env.exists(&corev1.Service{}, "shop", shopService) {
		t.Error("the Service or the route of the deleted object is left")
	}
	if _, found := env.dexClient("app", "shop"); found {
		t.Error("the Dex client of the deleted object is left")
	}

	env.buildTables()
	user := env.table(naming.UserAuthenticatorInstance)
	if _, found := user.record(shopApp); found {
		t.Error("the record of the deleted object is left")
	}
	if user.file.Generation != env.now.Unix() {
		t.Errorf("generation %d, want %d", user.file.Generation, env.now.Unix())
	}

	// The other object is untouched.
	if !env.exists(&networkingv1.Ingress{}, "shop", "other-dex-authenticator") || !env.exists(&corev1.Service{}, "shop", "other-dex-authenticator") {
		t.Error("the objects of another DexAuthenticator are gone")
	}
	if _, found := env.dexClient("other", "shop"); !found {
		t.Error("the Dex client of another DexAuthenticator is gone")
	}
}

// TestReconcile_RecreatedObject: the objects of an object created again under the name of a
// deleted one are not written until the table reconciler has decided about the new object, and
// then carry its new secret.
func TestReconcile_RecreatedObject(t *testing.T) {
	t.Parallel()

	obj := authenticatorObject("shop", "app", 0, "app.example.com")
	env := newTestEnv(t, testSettings, false, obj)
	env.serve(shopApp)
	old := env.table(naming.UserAuthenticatorInstance).file.Clients[0]

	if err := env.client.Delete(t.Context(), obj); err != nil {
		t.Fatal(err)
	}
	recreated := authenticatorObject("shop", "app", 5, "app.example.com")
	recreated.SetUID("uid-of-the-new-object")
	if err := env.client.Create(t.Context(), recreated); err != nil {
		t.Fatal(err)
	}

	if result := env.mustReconcile(shopApp); result.RequeueAfter != notReadyRecheck {
		t.Errorf("before the tables: requeue after %v, want %v", result.RequeueAfter, notReadyRecheck)
	}

	env.now = testNow.Add(time.Hour)
	env.buildTables()
	record := env.table(naming.UserAuthenticatorInstance).file.Clients[0]
	if record.ClientSecret == old.ClientSecret || record.CookieSecret == old.CookieSecret {
		t.Fatal("the new object took the secrets of the deleted one")
	}

	env.mustReconcile(shopApp)
	dexClient, _ := env.dexClient("app", "shop")
	if secret, _, _ := unstructured.NestedString(dexClient.Object, "secret"); secret != record.ClientSecret {
		t.Error("the Dex client does not carry the new secret")
	}
	if got := env.condition(shopApp); got.Reason != ReasonWaitingForInstance {
		t.Errorf("condition = %+v, want the object to wait for the new record to be loaded", got)
	}
}

// gatewayObject is an object whose applications are published through the Gateway API.
func gatewayObject(namespace, name string, age int, domains ...string) *unstructured.Unstructured {
	obj := authenticatorObject(namespace, name, age, domains...)
	spec, _ := obj.Object["spec"].(map[string]any)
	delete(spec, "applicationIngressClassName")
	spec["gatewayAPI"] = map[string]any{"applicationHTTPRouteListenerSetName": "public"}
	additional, _ := spec["additionalApplications"].([]any)
	for _, item := range additional {
		app, _ := item.(map[string]any)
		delete(app, "ingressClassName")
		app["gatewayAPI"] = map[string]any{"httpRouteListenerSetName": "public"}
	}
	return obj
}

// TestReconcile_HostConflict: an application published through the Gateway API does not share a
// host with another object, so the object that names a taken host for it is not served on it.
func TestReconcile_HostConflict(t *testing.T) {
	t.Parallel()

	withGatewayAPI := testSettings
	withGatewayAPI.GatewayAPI = true
	env := newTestEnv(t, withGatewayAPI, true,
		authenticatorObject("shop", "app", 0, "app.example.com"),
		gatewayObject("another", "loser", 5, "app.example.com"),
		gatewayObject("another", "partial", 6, "app.example.com", "partial.example.com"))
	env.buildTables()
	env.load()

	loser := keyOf("another", "loser")
	env.mustReconcile(loser)
	if got := env.condition(loser); got.Status != metav1.ConditionFalse || got.Reason != ReasonHostConflict || !strings.Contains(got.Message, "app.example.com") {
		t.Errorf("condition of the object with no host = %+v", got)
	}
	if _, found := env.dexClient("loser", "another"); found || env.exists(&corev1.Service{}, "another", "loser-dex-authenticator") {
		t.Error("the object with no host has a Dex client or a Service")
	}
	recorded := env.events()
	if len(recorded) != 1 || !strings.Contains(recorded[0], "Warning HostConflict") {
		t.Errorf("events = %v, want one HostConflict warning", recorded)
	}
	// The same refusal is not reported again.
	env.mustReconcile(loser)
	if recorded := env.events(); len(recorded) != 0 {
		t.Errorf("a second pass recorded %v", recorded)
	}

	// An object that lost one host of two is served on the other, and says so.
	partial := keyOf("another", "partial")
	env.mustReconcile(partial)
	if got := env.condition(partial); got.Status != metav1.ConditionFalse || got.Reason != ReasonHostConflict || !strings.Contains(got.Message, "served on partial.example.com") {
		t.Errorf("condition of the object that lost a host = %+v", got)
	}
	routes := controller.List(HTTPRouteGVK)
	if err := env.client.List(t.Context(), routes, client.InNamespace("another")); err != nil {
		t.Fatal(err)
	}
	if len(routes.Items) != 1 {
		t.Fatalf("HTTPRoutes of the object that lost a host: %d, want the one of partial.example.com", len(routes.Items))
	}
	if hostnames, _, _ := unstructured.NestedStringSlice(routes.Items[0].Object, "spec", "hostnames"); !slices.Equal(hostnames, []string{"partial.example.com"}) {
		t.Errorf("hostnames of the HTTPRoute of the object that lost a host = %v", hostnames)
	}
	dexClient, _ := env.dexClient("partial", "another")
	if redirects, _, _ := unstructured.NestedStringSlice(dexClient.Object, "redirectURIs"); !slices.Equal(redirects, []string{"https://partial.example.com/dex-authenticator/callback"}) {
		t.Errorf("redirectURIs = %v, want the host the object holds only", redirects)
	}

	// The holder goes: the host passes to the oldest claimant.
	holder := controller.Object(controller.DexAuthenticatorGVK)
	env.exists(holder, "shop", "app")
	if err := env.client.Delete(t.Context(), holder); err != nil {
		t.Fatal(err)
	}
	env.now = testNow.Add(time.Hour)
	if announced := env.buildTables(); !slices.Contains(announced, loser) {
		t.Errorf("announced %v, want the object that gets the host", announced)
	}
	env.load()
	env.mustReconcile(loser)
	if got := env.condition(loser); got.Status != metav1.ConditionTrue {
		t.Errorf("condition of the object that got the host = %+v", got)
	}
}

// TestReconcile_SharedHost: two objects publish one domain through two Ingress classes. Both are
// served, and the Ingress objects of both tell ingress-nginx to send the name of the Service of
// their object as the Host header, which is how the instance tells their requests apart. The
// annotation is there only while the host has two objects.
func TestReconcile_SharedHost(t *testing.T) {
	t.Parallel()

	const (
		ownerVhost  = "app-dex-authenticator.shop.svc.cluster.local"
		sharerVhost = "app-dex-authenticator.another.svc.cluster.local"
	)
	owner, sharer := keyOf("shop", "app"), keyOf("another", "app")
	vhost := func(env *testEnv, key types.NamespacedName, name string) (string, bool) {
		ingress := &networkingv1.Ingress{}
		if !env.exists(ingress, key.Namespace, name) {
			t.Fatalf("no Ingress %s/%s", key.Namespace, name)
		}
		value, has := ingress.Annotations[upstreamVhostAnnotation]
		return value, has
	}

	ownerObject := authenticatorObject("shop", "app", 0, "app.example.com", "own.example.com")
	ownerSpec, _ := ownerObject.Object["spec"].(map[string]any)
	ownerSpec["signOutURL"] = "/logout"
	env := newTestEnv(t, testSettings, false, ownerObject)
	env.serve(owner)

	// One object on the host: its Ingress objects are the ones the chart made.
	for _, name := range []string{"app-dex-authenticator", "app-dex-authenticator-sign-out"} {
		if value, has := vhost(env, owner, name); has {
			t.Errorf("Ingress %s of the only object on the host sends the Host %q", name, value)
		}
	}

	ownerVersions := map[string]string{}
	owned := &networkingv1.IngressList{}
	if err := env.client.List(t.Context(), owned, client.InNamespace("shop")); err != nil {
		t.Fatal(err)
	}
	for _, item := range owned.Items {
		ownerVersions[item.Name] = item.ResourceVersion
	}

	// A second object names the host through another Ingress class.
	sharerObject := authenticatorObject("another", "app", 5, "app.example.com")
	sharerSpec, _ := sharerObject.Object["spec"].(map[string]any)
	sharerSpec["applicationIngressClassName"] = "nginx-internal"
	if err := env.client.Create(t.Context(), sharerObject); err != nil {
		t.Fatal(err)
	}
	env.now = testNow.Add(time.Hour)
	announced := env.buildTables()
	if !slices.Contains(announced, sharer) || slices.Contains(announced, owner) {
		t.Errorf("announced %v, want the object that shares the host and not the one the host belongs to", announced)
	}

	// The record of the second object is not loaded yet: it has no route, and nothing of the first
	// object waits for it.
	env.mustReconcile(sharer)
	if env.exists(&networkingv1.Ingress{}, "another", "app-dex-authenticator") {
		t.Error("the object that shares the host has a route before its record is loaded")
	}
	env.mustReconcile(owner)
	if got := env.condition(owner); got.Status != metav1.ConditionTrue {
		t.Errorf("condition of the first object = %+v", got)
	}

	env.load()
	env.mustReconcile(sharer)
	if got := env.condition(sharer); got.Status != metav1.ConditionTrue || got.Reason != ReasonServed {
		t.Fatalf("condition of the object that shares the host = %+v", got)
	}
	if value, _ := vhost(env, sharer, "app-dex-authenticator"); value != sharerVhost {
		t.Errorf("the Ingress of the object that shares the host sends the Host %q, want %q", value, sharerVhost)
	}
	ingress := &networkingv1.Ingress{}
	env.exists(ingress, "another", "app-dex-authenticator")
	if class := ingress.Spec.IngressClassName; class == nil || *class != "nginx-internal" || ingress.Spec.Rules[0].Host != "app.example.com" {
		t.Errorf("the Ingress of the object that shares the host: %+v", ingress.Spec)
	}
	dexClient, _ := env.dexClient("app", "another")
	if redirects, _, _ := unstructured.NestedStringSlice(dexClient.Object, "redirectURIs"); !slices.Equal(redirects, []string{"https://app.example.com/dex-authenticator/callback"}) {
		t.Errorf("redirectURIs of the object that shares the host = %v", redirects)
	}

	// The object the host belongs to is chosen by the host, so nothing of it changes: an object
	// of another namespace that begins to share the host writes nothing into its Ingress objects.
	ingresses := &networkingv1.IngressList{}
	if err := env.client.List(t.Context(), ingresses, client.InNamespace("shop")); err != nil {
		t.Fatal(err)
	}
	for _, item := range ingresses.Items {
		if value, has := item.Annotations[upstreamVhostAnnotation]; has {
			t.Errorf("Ingress %s of the object the host belongs to sends the Host %q", item.Name, value)
		}
		if item.ResourceVersion != ownerVersions[item.Name] {
			t.Errorf("Ingress %s of the object the host belongs to was written when another object began to share the host", item.Name)
		}
	}
	table := env.table(naming.UserAuthenticatorInstance)
	if record, _ := table.record(sharer); !slices.Equal(record.SharedHosts, []string{"app.example.com"}) || !slices.Equal(record.Services, []string{sharerVhost}) {
		t.Errorf("the record of the object that shares the host: %+v", record)
	}

	// A second pass writes nothing.
	env.exists(ingress, "shop", "app-dex-authenticator")
	before := ingress.ResourceVersion
	env.mustReconcile(owner)
	env.exists(ingress, "shop", "app-dex-authenticator")
	if ingress.ResourceVersion != before {
		t.Error("a second pass wrote the Ingress")
	}

	// The second object names another host: the pair is no more, and the annotation goes from
	// its Ingress.
	live := controller.Object(controller.DexAuthenticatorGVK)
	env.exists(live, "another", "app")
	liveSpec, _ := live.Object["spec"].(map[string]any)
	liveSpec["applicationDomain"] = "moved.example.com"
	if err := env.client.Update(t.Context(), live); err != nil {
		t.Fatal(err)
	}
	env.now = testNow.Add(2 * time.Hour)
	if announced := env.buildTables(); !slices.Contains(announced, sharer) {
		t.Errorf("announced %v, want the object that left the host", announced)
	}
	env.load()
	env.mustReconcile(owner)
	env.mustReconcile(sharer)
	for _, name := range []string{"app-dex-authenticator", "app-dex-authenticator-sign-out"} {
		if value, has := vhost(env, owner, name); has {
			t.Errorf("Ingress %s of the object that is left on the host sends the Host %q", name, value)
		}
	}
	if value, has := vhost(env, sharer, "app-dex-authenticator"); has {
		t.Errorf("the Ingress of the object that left the host sends the Host %q", value)
	}
	env.exists(ingress, "another", "app-dex-authenticator")
	if ingress.Spec.Rules[0].Host != "moved.example.com" {
		t.Errorf("the Ingress of the object that left the host is for %s", ingress.Spec.Rules[0].Host)
	}

	// The pair again, and then the first object is deleted: the annotation goes from the Ingress
	// of the object that is left, which is chosen by the host from now on.
	liveSpec["applicationDomain"] = "app.example.com"
	env.exists(live, "another", "app")
	live.Object["spec"] = liveSpec
	if err := env.client.Update(t.Context(), live); err != nil {
		t.Fatal(err)
	}
	env.now = testNow.Add(3 * time.Hour)
	env.buildTables()
	env.load()
	env.mustReconcile(owner)
	env.mustReconcile(sharer)
	if value, _ := vhost(env, sharer, "app-dex-authenticator"); value != sharerVhost {
		t.Fatalf("the pair again: the Ingress sends the Host %q", value)
	}

	env.exists(live, "shop", "app")
	if err := env.client.Delete(t.Context(), live); err != nil {
		t.Fatal(err)
	}
	env.buildTables()
	// The routes of the deleted object are still there, and so is its record: the pair stays.
	env.mustReconcile(sharer)
	if value, _ := vhost(env, sharer, "app-dex-authenticator"); value != sharerVhost {
		t.Errorf("the Ingress lost the annotation while the routes of the deleted object are there: %q", value)
	}
	env.mustReconcile(owner)
	env.now = testNow.Add(4 * time.Hour)
	env.buildTables()
	env.load()
	env.mustReconcile(sharer)
	if value, has := vhost(env, sharer, "app-dex-authenticator"); has {
		t.Errorf("the Ingress of the only object on the host sends the Host %q", value)
	}
	if record, _ := env.table(naming.UserAuthenticatorInstance).record(sharer); !slices.Equal(record.Hosts, []string{"app.example.com"}) || len(record.SharedHosts) != 0 {
		t.Errorf("the record of the only object on the host: %+v", record)
	}
	if got := env.condition(sharer); got.Status != metav1.ConditionTrue {
		t.Errorf("condition of the only object on the host = %+v", got)
	}
}

// TestReconcile_ForeignObjects: a Service, an Ingress or a Dex client that somebody else made
// under a name of the object is left alone.
func TestReconcile_ForeignObjects(t *testing.T) {
	t.Parallel()

	foreignService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: shopService, Namespace: "shop", Labels: map[string]string{LabelFor: "app"}},
		Spec:       corev1.ServiceSpec{Selector: map[string]string{"app": "theirs"}},
	}
	env := newTestEnv(t, testSettings, false, authenticatorObject("shop", "app", 0, "app.example.com"), foreignService)
	env.buildTables()
	env.load()

	result := env.mustReconcile(shopApp)
	if result.RequeueAfter != refusedRecheck {
		t.Errorf("requeue after %v, want %v", result.RequeueAfter, refusedRecheck)
	}
	service := &corev1.Service{}
	env.exists(service, "shop", shopService)
	if service.Spec.Selector["app"] != "theirs" || service.Spec.Type == corev1.ServiceTypeExternalName {
		t.Errorf("the Service of somebody else was changed: %+v", service.Spec)
	}
	if got := env.condition(shopApp); got.Status != metav1.ConditionFalse || got.Reason != ReasonForeignObject || !strings.Contains(got.Message, shopService) {
		t.Errorf("condition = %+v", got)
	}
	if env.exists(&networkingv1.Ingress{}, "shop", shopService) {
		t.Error("a route to the Service of somebody else")
	}

	// Its owner removes it: the object is served, and the deleted DexAuthenticator takes nothing of others.
	if err := env.client.Delete(t.Context(), service); err != nil {
		t.Fatal(err)
	}
	env.mustReconcile(shopApp)
	if got := env.condition(shopApp); got.Status != metav1.ConditionTrue {
		t.Errorf("condition = %+v", got)
	}
}

func TestReconcile_ForeignDexClient(t *testing.T) {
	t.Parallel()

	foreign := controller.Object(controller.OAuth2ClientGVK)
	foreign.SetNamespace(naming.DexNamespace)
	foreign.SetName(naming.ToFnvLikeDex("dex-authenticator-app@shop"))
	foreign.Object["id"] = "dex-authenticator-app@shop"
	foreign.Object["secret"] = "dh-test-secret-0123456789abcdef0"
	env := newTestEnv(t, testSettings, false, authenticatorObject("shop", "app", 0, "app.example.com"), foreign)
	env.buildTables()
	env.load()

	if result := env.mustReconcile(shopApp); result.RequeueAfter != refusedRecheck {
		t.Errorf("requeue after %v, want %v", result.RequeueAfter, refusedRecheck)
	}
	dexClient, _ := env.dexClient("app", "shop")
	if secret, _, _ := unstructured.NestedString(dexClient.Object, "secret"); secret != "dh-test-secret-0123456789abcdef0" {
		t.Error("the Dex client of somebody else was changed")
	}
	if got := env.condition(shopApp); got.Reason != ReasonClientNotCreated {
		t.Errorf("condition = %+v", got)
	}
	if env.exists(&corev1.Service{}, "shop", shopService) {
		t.Error("a Service without the Dex client")
	}

	// Deleting the DexAuthenticator leaves the client of somebody else.
	obj := controller.Object(controller.DexAuthenticatorGVK)
	env.exists(obj, "shop", "app")
	if err := env.client.Delete(t.Context(), obj); err != nil {
		t.Fatal(err)
	}
	env.mustReconcile(shopApp)
	if _, found := env.dexClient("app", "shop"); !found {
		t.Error("the Dex client of somebody else was deleted")
	}
}

func TestReconcile_InstanceRefusedTheRecord(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t, testSettings, false, authenticatorObject("shop", "app", 0, "app.example.com"))
	env.buildTables()
	env.load()
	record, _ := env.table(naming.UserAuthenticatorInstance).record(shopApp)
	env.instances.failed = []string{statusID(&record)}

	if result := env.mustReconcile(shopApp); result.RequeueAfter != refusedRecheck {
		t.Errorf("requeue after %v, want %v", result.RequeueAfter, refusedRecheck)
	}
	if got := env.condition(shopApp); got.Status != metav1.ConditionFalse || got.Reason != ReasonInvalidRecord {
		t.Errorf("condition = %+v", got)
	}
	if env.exists(&corev1.Service{}, "shop", shopService) {
		t.Error("a Service for a record the instance refused")
	}

	// The record is corrected, and a pod still serves the table with the record it refused: the
	// refusal is of the old record, so the object waits for the pods, it is not refused.
	live := controller.Object(controller.DexAuthenticatorGVK)
	env.exists(live, "shop", "app")
	spec, _ := live.Object["spec"].(map[string]any)
	spec["keepUsersLoggedInFor"] = "24h"
	if err := env.client.Update(t.Context(), live); err != nil {
		t.Fatal(err)
	}
	env.now = testNow.Add(time.Hour)
	env.buildTables()
	if result := env.mustReconcile(shopApp); result.RequeueAfter != loadRecheck {
		t.Errorf("requeue after %v, want %v", result.RequeueAfter, loadRecheck)
	}
	if got := env.condition(shopApp); got.Reason != ReasonWaitingForInstance {
		t.Errorf("condition while a pod has the old table = %+v", got)
	}

	// Every pod has the new table and none refuses the record.
	env.load()
	env.mustReconcile(shopApp)
	if got := env.condition(shopApp); got.Status != metav1.ConditionTrue {
		t.Errorf("condition after the pods loaded the corrected record = %+v", got)
	}
}

// TestReconcile_FullTableKeepsTheObjectServed: the record of a served object grows over the limit
// of the table. Its routes, its Service, its Dex client and its secrets stay as they are, and the
// condition tells why the change is not there.
func TestReconcile_FullTableKeepsTheObjectServed(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t, testSettings, false, authenticatorObject("shop", "app", 0, "app.example.com"))
	env.serve(shopApp)
	record := env.table(naming.UserAuthenticatorInstance).file.Clients[0]
	dexClient, _ := env.dexClient("app", "shop")
	clientVersion := dexClient.GetResourceVersion()
	service := &corev1.Service{}
	env.exists(service, "shop", shopService)
	ingress := &networkingv1.Ingress{}
	env.exists(ingress, "shop", shopService)

	// Room for the record the object has and for nothing more.
	env.tables.limit = emptyTableSize + recordSize(record, "uid-shop-app") + 10

	live := controller.Object(controller.DexAuthenticatorGVK)
	env.exists(live, "shop", "app")
	spec, _ := live.Object["spec"].(map[string]any)
	spec["additionalApplications"] = []any{map[string]any{"domain": "one-more.example.com", "ingressClassName": "nginx"}}
	if err := env.client.Update(t.Context(), live); err != nil {
		t.Fatal(err)
	}
	env.now = testNow.Add(time.Hour)
	env.buildTables()
	env.mustReconcile(shopApp)

	if got := env.condition(shopApp); got.Status != metav1.ConditionFalse || got.Reason != ReasonTableFull || !strings.Contains(got.Message, "served by the record it has") {
		t.Errorf("condition = %+v", got)
	}
	if got := env.table(naming.UserAuthenticatorInstance).file.Clients; len(got) != 1 || !recordsEqual(got[0], record) {
		t.Errorf("the table = %+v, want the record the object had", got)
	}
	after, found := env.dexClient("app", "shop")
	if !found || after.GetResourceVersion() != clientVersion {
		t.Error("the Dex client was written or removed")
	}
	if secret, _, _ := unstructured.NestedString(after.Object, "secret"); secret != record.ClientSecret {
		t.Error("the Dex client has another secret")
	}
	stillService, stillIngress := &corev1.Service{}, &networkingv1.Ingress{}
	if !env.exists(stillService, "shop", shopService) || stillService.ResourceVersion != service.ResourceVersion {
		t.Error("the Service was written or removed")
	}
	if !env.exists(stillIngress, "shop", shopService) || stillIngress.ResourceVersion != ingress.ResourceVersion {
		t.Error("the Ingress of the domain the object is served on was written or removed")
	}
	ingresses := &networkingv1.IngressList{}
	if err := env.client.List(t.Context(), ingresses, client.InNamespace("shop")); err != nil {
		t.Fatal(err)
	}
	if len(ingresses.Items) != 1 {
		t.Errorf("%d Ingress objects, want none for the domain the object is not served on", len(ingresses.Items))
	}

	// The object names another domain instead of the one it is served on and changes who may sign
	// in: nothing of what was made for the record it stays on is taken away or changed.
	env.exists(live, "shop", "app")
	spec, _ = live.Object["spec"].(map[string]any)
	spec["applicationDomain"] = "a-much-longer-domain-of-the-application.example.com"
	spec["allowedEmails"] = []any{"somebody@example.com"}
	if err := env.client.Update(t.Context(), live); err != nil {
		t.Fatal(err)
	}
	env.now = testNow.Add(90 * time.Minute)
	env.buildTables()
	env.mustReconcile(shopApp)
	if got := env.condition(shopApp); got.Reason != ReasonTableFull || !strings.Contains(got.Message, "served by the record it has") {
		t.Errorf("condition after the object changed its domain = %+v", got)
	}
	if !env.exists(stillIngress, "shop", shopService) || stillIngress.ResourceVersion != ingress.ResourceVersion || stillIngress.Spec.Rules[0].Host != "app.example.com" {
		t.Error("the Ingress of the domain of the record was changed or removed")
	}
	if after, found := env.dexClient("app", "shop"); !found || after.GetResourceVersion() != clientVersion {
		t.Error("the Dex client was changed or removed")
	}
	if got := env.table(naming.UserAuthenticatorInstance).file.Clients; len(got) != 1 || !recordsEqual(got[0], record) {
		t.Errorf("the table = %+v, want the record the object had", got)
	}
	// Back to the object that only adds a domain.
	env.exists(live, "shop", "app")
	spec, _ = live.Object["spec"].(map[string]any)
	spec["applicationDomain"] = "app.example.com"
	delete(spec, "allowedEmails")
	if err := env.client.Update(t.Context(), live); err != nil {
		t.Fatal(err)
	}

	// Room appears: the object is served on both domains, with the secrets it had.
	env.tables.limit = TableLimit
	env.now = testNow.Add(2 * time.Hour)
	env.buildTables()
	env.load()
	env.mustReconcile(shopApp)
	if got := env.condition(shopApp); got.Status != metav1.ConditionTrue {
		t.Errorf("condition once the table has room = %+v", got)
	}
	if got := env.table(naming.UserAuthenticatorInstance).file.Clients[0]; got.ClientSecret != record.ClientSecret || got.CookieSecret != record.CookieSecret || len(got.Hosts) != 2 {
		t.Errorf("the record once the table has room: hosts %v", got.Hosts)
	}
}

// TestReconcile_IngressKeepsOneHSTSAnnotation: an Ingress of an object carries HSTS in the
// annotation or in the snippet, never in both. The chart of an earlier release always wrote the
// snippet, and nothing records it as the controller's on an Ingress taken over from that chart.
func TestReconcile_IngressKeepsOneHSTSAnnotation(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t, testSettings, false, authenticatorObject("shop", "app", 0, "app.example.com"))
	env.serve(shopApp)

	const snippet = nginxAnnotation + "configuration-snippet"
	ingress := &networkingv1.Ingress{}
	env.exists(ingress, "shop", shopService)
	if ingress.Annotations[nginxAnnotation+"ingress-nginx-hsts"] != "true" {
		t.Fatalf("annotations = %v, want the HSTS annotation", ingress.Annotations)
	}
	ingress.Annotations[snippet] = hstsSnippet
	ingress.Annotations["example.com/of-somebody-else"] = "kept"
	if err := env.client.Update(t.Context(), ingress); err != nil {
		t.Fatal(err)
	}

	env.mustReconcile(shopApp)
	env.exists(ingress, "shop", shopService)
	if _, has := ingress.Annotations[snippet]; has {
		t.Error("the snippet is left next to the HSTS annotation")
	}
	if ingress.Annotations[nginxAnnotation+"ingress-nginx-hsts"] != "true" || ingress.Annotations["example.com/of-somebody-else"] != "kept" {
		t.Errorf("annotations = %v", ingress.Annotations)
	}

	// With controllers that do not know the annotation it is the other way round.
	old := testSettings
	old.IngressNginxHSTSAnnotation = false
	env.setSettings(old)
	env.mustReconcile(shopApp)
	env.exists(ingress, "shop", shopService)
	if _, has := ingress.Annotations[nginxAnnotation+"ingress-nginx-hsts"]; has || ingress.Annotations[snippet] != hstsSnippet {
		t.Errorf("annotations with the snippet = %v", ingress.Annotations)
	}
}

// TestReconcile_ChangedApplications: the routes follow the applications of the object, and the
// metadata of an object follows its settings.
func TestReconcile_ChangedApplications(t *testing.T) {
	t.Parallel()

	obj := authenticatorObject("shop", "app", 0, "app.example.com", "admin.example.com")
	spec, _ := obj.Object["spec"].(map[string]any)
	spec["signOutURL"] = "/logout"
	spec["whitelistSourceRanges"] = []any{"10.0.0.0/8"}
	spec["sendAuthorizationHeader"] = true
	env := newTestEnv(t, testSettings, false, obj)
	env.serve(shopApp)

	names := func() []string {
		list := &networkingv1.IngressList{}
		if err := env.client.List(t.Context(), list, client.InNamespace("shop")); err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, item := range list.Items {
			out = append(out, item.Name)
		}
		slices.Sort(out)
		return out
	}
	second := routeName("app", 1, "admin.example.com").name
	if got := names(); !slices.Equal(got, []string{second, "app-dex-authenticator", "app-dex-authenticator-sign-out"}) {
		t.Fatalf("Ingresses = %v", got)
	}
	ingress := &networkingv1.Ingress{}
	env.exists(ingress, "shop", shopService)
	if ingress.Annotations["nginx.ingress.kubernetes.io/whitelist-source-range"] != "10.0.0.0/8" || ingress.Annotations["nginx.ingress.kubernetes.io/proxy-buffer-size"] != "32k" {
		t.Fatalf("annotations = %v", ingress.Annotations)
	}

	// The second application, the sign-out address and the two settings go.
	current := controller.Object(controller.DexAuthenticatorGVK)
	env.exists(current, "shop", "app")
	current.Object["spec"] = map[string]any{"applicationDomain": "app.example.com", "applicationIngressClassName": "nginx"}
	current.SetGeneration(2)
	if err := env.client.Update(t.Context(), current); err != nil {
		t.Fatal(err)
	}
	env.now = testNow.Add(time.Hour)
	env.buildTables()
	env.load()
	env.mustReconcile(shopApp)

	if got := names(); !slices.Equal(got, []string{"app-dex-authenticator"}) {
		t.Errorf("Ingresses = %v, want the one of the application that is left", got)
	}
	env.exists(ingress, "shop", shopService)
	for _, annotation := range []string{"nginx.ingress.kubernetes.io/whitelist-source-range", "nginx.ingress.kubernetes.io/proxy-buffer-size"} {
		if _, has := ingress.Annotations[annotation]; has {
			t.Errorf("annotation %s is left", annotation)
		}
	}
	dexClient, _ := env.dexClient("app", "shop")
	if redirects, _, _ := unstructured.NestedStringSlice(dexClient.Object, "redirectURIs"); len(redirects) != 1 {
		t.Errorf("redirectURIs = %v", redirects)
	}
	if got := env.condition(shopApp); got.Status != metav1.ConditionTrue || got.ObservedGeneration != 2 {
		t.Errorf("condition = %+v", got)
	}
}

func TestReconcile_HTTPRoutes(t *testing.T) {
	t.Parallel()

	newObject := func() *unstructured.Unstructured {
		obj := authenticatorObject("shop", "app", 0, "app.example.com")
		spec, _ := obj.Object["spec"].(map[string]any)
		delete(spec, "applicationIngressClassName")
		spec["gatewayAPI"] = map[string]any{"applicationHTTPRouteListenerSetName": "public"}
		return obj
	}
	route := func(env *testEnv) (*unstructured.Unstructured, bool) {
		obj := controller.Object(HTTPRouteGVK)
		return obj, env.exists(obj, "shop", shopService)
	}

	withGatewayAPI := testSettings
	withGatewayAPI.GatewayAPI = true

	// Without HTTPRoute objects in the cluster, or with the Gateway API off in the settings, nothing
	// is made, and the object is served by its Service.
	for _, tt := range []struct {
		settings Settings
		kind     bool
	}{{withGatewayAPI, false}, {testSettings, true}} {
		env := newTestEnv(t, tt.settings, tt.kind, newObject())
		env.serve(shopApp)
		if _, found := route(env); found {
			t.Errorf("an HTTPRoute with the kind in the cluster: %v, the setting: %v", tt.kind, tt.settings.GatewayAPI)
		}
	}

	env := newTestEnv(t, withGatewayAPI, true, newObject())
	env.serve(shopApp)
	made, found := route(env)
	if !found {
		t.Fatal("no HTTPRoute")
	}
	if made.GetLabels()[adoption.ManagedByLabel] != adoption.ManagedByValue {
		t.Errorf("labels = %v", made.GetLabels())
	}
	if hostnames, _, _ := unstructured.NestedStringSlice(made.Object, "spec", "hostnames"); !slices.Equal(hostnames, []string{"app.example.com"}) {
		t.Errorf("hostnames = %v", hostnames)
	}
	before := made.GetResourceVersion()
	env.mustReconcile(shopApp)
	if again, _ := route(env); again.GetResourceVersion() != before {
		t.Error("a second pass wrote the HTTPRoute")
	}

	// The Gateway API is turned off in the settings: the route is removed, not left behind.
	off := newTestEnv(t, withGatewayAPI, true, newObject())
	off.serve(shopApp)
	off.setSettings(testSettings)
	off.mustReconcile(shopApp)
	if _, found := route(off); found {
		t.Error("the HTTPRoute is left after the Gateway API was turned off")
	}

	// The record waits for an HTTPRoute as it does for an Ingress.
	obj := controller.Object(controller.DexAuthenticatorGVK)
	env.exists(obj, "shop", "app")
	if err := env.client.Delete(t.Context(), obj); err != nil {
		t.Fatal(err)
	}
	env.buildTables()
	if _, found := env.table(naming.UserAuthenticatorInstance).record(shopApp); !found {
		t.Fatal("the record went before the HTTPRoute")
	}
	env.mustReconcile(shopApp)
	if _, found := route(env); found {
		t.Error("the HTTPRoute of the deleted object is left")
	}
	env.buildTables()
	if _, found := env.table(naming.UserAuthenticatorInstance).record(shopApp); found {
		t.Error("the record of the deleted object is left")
	}
}

// TestReconcile_PlatformObjectsShareTheSignIn: with the sessions of Dex, the client of every
// object of the d8-* namespaces names the clients of the others, and a new one changes them all.
func TestReconcile_PlatformObjectsShareTheSignIn(t *testing.T) {
	t.Parallel()

	settings := testSettings
	settings.DexSessions, settings.PlatformSSO = true, true
	console, grafana := keyOf("d8-console", "console"), keyOf("d8-monitoring", "grafana")
	env := newTestEnv(t, settings, false,
		authenticatorObject("d8-console", "console", 0, "console.example.com"),
		authenticatorObject("d8-monitoring", "grafana", 1, "grafana.example.com"),
		authenticatorObject("shop", "app", 2, "app.example.com"))
	env.buildTables()
	env.load()
	for _, key := range []types.NamespacedName{console, grafana, shopApp} {
		env.mustReconcile(key)
	}

	peers := func(name, namespace string) []string {
		dexClient, _ := env.dexClient(name, namespace)
		out, _, _ := unstructured.NestedStringSlice(dexClient.Object, "ssoSharedWith")
		return out
	}
	if got := peers("console", "d8-console"); !slices.Equal(got, []string{"dex-authenticator-grafana@d8-monitoring"}) {
		t.Errorf("peers of console = %v", got)
	}
	if got := peers("app", "shop"); len(got) != 0 {
		t.Errorf("peers of an object of a user = %v", got)
	}
	dexClient, _ := env.dexClient("app", "shop")
	if lifetime, _, _ := unstructured.NestedString(dexClient.Object, "refreshTokenLifetime"); lifetime != "session" {
		t.Errorf("refreshTokenLifetime = %q", lifetime)
	}

	if err := env.client.Create(t.Context(), authenticatorObject("d8-system", "documentation", 3, "documentation.example.com")); err != nil {
		t.Fatal(err)
	}
	env.now = testNow.Add(time.Hour)
	announced := env.buildTables()
	if !slices.Contains(announced, console) || !slices.Contains(announced, grafana) || slices.Contains(announced, shopApp) {
		t.Errorf("announced %v, want every object of the d8-* namespaces and no other", announced)
	}
	env.mustReconcile(console)
	if got := peers("console", "d8-console"); !slices.Equal(got, []string{"dex-authenticator-grafana@d8-monitoring", "dex-authenticator-documentation@d8-system"}) {
		t.Errorf("peers of console = %v", got)
	}
}

// TestReconcile_SettingsChange: the objects follow the settings of the module.
func TestReconcile_SettingsChange(t *testing.T) {
	t.Parallel()

	obj := authenticatorObject("shop", "app", 0, "app.example.com")
	spec, _ := obj.Object["spec"].(map[string]any)
	spec["applicationIngressCertificateSecretName"] = "app-tls"
	env := newTestEnv(t, testSettings, false, obj)
	env.serve(shopApp)

	ingress := &networkingv1.Ingress{}
	env.exists(ingress, "shop", shopService)
	if len(ingress.Spec.TLS) != 1 || ingress.Annotations["nginx.ingress.kubernetes.io/ingress-nginx-hsts"] != "true" {
		t.Fatalf("Ingress with a certificate: tls %v, annotations %v", ingress.Spec.TLS, ingress.Annotations)
	}

	settings := testSettings
	settings.HTTPSMode = "OnlyInURI"
	env.setSettings(settings)
	env.mustReconcile(shopApp)
	env.exists(ingress, "shop", shopService)
	if len(ingress.Spec.TLS) != 0 {
		t.Errorf("tls = %v, want none without a certificate", ingress.Spec.TLS)
	}
	if _, has := ingress.Annotations["nginx.ingress.kubernetes.io/ingress-nginx-hsts"]; has {
		t.Error("the HSTS annotation is left")
	}
}

func TestReconcile_InvalidObject(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t, testSettings, false, authenticatorObject("shop", "app", 0, "app.example.com"))
	env.serve(shopApp)
	record := env.table(naming.UserAuthenticatorInstance).file.Clients[0]

	obj := controller.Object(controller.DexAuthenticatorGVK)
	env.exists(obj, "shop", "app")
	if err := unstructured.SetNestedField(obj.Object, "a week", "spec", "keepUsersLoggedInFor"); err != nil {
		t.Fatal(err)
	}
	if err := env.client.Update(t.Context(), obj); err != nil {
		t.Fatal(err)
	}
	env.now = testNow.Add(time.Hour)
	env.buildTables()
	env.mustReconcile(shopApp)

	if got := env.table(naming.UserAuthenticatorInstance).file.Clients; len(got) != 1 || !recordsEqual(got[0], record) {
		t.Errorf("the record of the object that cannot be read changed: %+v", got)
	}
	if !env.exists(&corev1.Service{}, "shop", shopService) || !env.exists(&networkingv1.Ingress{}, "shop", shopService) {
		t.Error("the objects of the object that cannot be read are gone")
	}
	if got := env.condition(shopApp); got.Status != metav1.ConditionFalse || got.Reason != ReasonInvalidRecord || !strings.Contains(got.Message, "keepUsersLoggedInFor") {
		t.Errorf("condition = %+v", got)
	}
}

// TestTableReconciler_FailedWrite: a pass that wrote one table and failed on the other leaves no
// record with a generation older than the table it is in, or a route would be made for a record
// no pod has loaded.
func TestTableReconciler_FailedWrite(t *testing.T) {
	t.Parallel()

	console := keyOf("d8-console", "console")
	base := newFakeClient(t, settingsConfigMap(t, testSettings),
		authenticatorObject("shop", "app", 0, "app.example.com"),
		authenticatorObject("d8-console", "console", 1, "console.example.com"))
	failing := false
	c := interceptor.NewClient(base, interceptor.Funcs{
		Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			if failing && obj.GetName() == TableSecretName(naming.UserAuthenticatorInstance) {
				return errors.New("the API server is away")
			}
			return c.Update(ctx, obj, opts...)
		},
	})

	env := newTestEnvOver(t, c, false)
	state := env.objects.state
	env.serve(shopApp)
	env.serve(console)

	// Both objects get one more domain; the table of the system instance is written, the other is not.
	for _, key := range []types.NamespacedName{console, shopApp} {
		obj := controller.Object(controller.DexAuthenticatorGVK)
		env.exists(obj, key.Namespace, key.Name)
		additional := []any{map[string]any{"domain": "more." + key.Name + ".example.com", "ingressClassName": "nginx"}}
		if err := unstructured.SetNestedSlice(obj.Object, additional, "spec", "additionalApplications"); err != nil {
			t.Fatal(err)
		}
		if err := env.client.Update(t.Context(), obj); err != nil {
			t.Fatal(err)
		}
	}
	env.now = testNow.Add(time.Hour)
	failing = true
	if _, err := env.tables.Reconcile(t.Context(), tablesRequest); err == nil {
		t.Fatal("the failed write is not reported")
	}
	if got := env.table(naming.SystemAuthenticatorInstance).file.Generation; got != env.now.Unix() {
		t.Fatalf("the table of the system instance has generation %d, want the written one", got)
	}

	// The next pass succeeds. The pods still serve the tables of before.
	failing = false
	env.now = testNow.Add(2 * time.Hour)
	env.buildTables()
	for _, key := range []types.NamespacedName{console, shopApp} {
		entry, _, _ := state.Entry(key)
		written := env.table(entry.Instance).file.Generation
		if entry.Generation != written {
			t.Errorf("%s: the record has generation %d in a table of generation %d", key, entry.Generation, written)
		}
		if result := env.mustReconcile(key); result.RequeueAfter != loadRecheck {
			t.Errorf("%s: requeue after %v, want to wait for the instance", key, result.RequeueAfter)
		}
		if env.exists(&networkingv1.Ingress{}, key.Namespace, routeName(key.Name, 1, "more."+key.Name+".example.com").name) {
			t.Errorf("%s: a route for a domain no pod has loaded", key)
		}
	}
}

// TestOrder_RouteOfARemovedHostGoesAtOnce: the route of a domain the object no longer names is
// removed without waiting for the instance, and the routes that stay are not touched meanwhile.
func TestOrder_RouteOfARemovedHostGoesAtOnce(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t, testSettings, false, authenticatorObject("shop", "app", 0, "app.example.com", "admin.example.com"))
	env.serve(shopApp)
	second := routeName("app", 1, "admin.example.com").name

	obj := controller.Object(controller.DexAuthenticatorGVK)
	env.exists(obj, "shop", "app")
	obj.Object["spec"] = map[string]any{
		"applicationDomain": "app.example.com", "applicationIngressClassName": "nginx",
		"additionalApplications": []any{map[string]any{"domain": "new.example.com", "ingressClassName": "nginx"}},
	}
	if err := env.client.Update(t.Context(), obj); err != nil {
		t.Fatal(err)
	}
	env.now = testNow.Add(time.Hour)
	env.buildTables()

	// The pods have not loaded the new record.
	if result := env.mustReconcile(shopApp); result.RequeueAfter != loadRecheck {
		t.Errorf("requeue after %v, want to wait for the instance", result.RequeueAfter)
	}
	if env.exists(&networkingv1.Ingress{}, "shop", second) {
		t.Error("the route of the removed domain is left while the instance loads the record")
	}
	if env.exists(&networkingv1.Ingress{}, "shop", routeName("app", 1, "new.example.com").name) {
		t.Error("the route of the new domain was made before the instance loaded the record")
	}
	if !env.exists(&networkingv1.Ingress{}, "shop", shopService) || !env.exists(&corev1.Service{}, "shop", shopService) {
		t.Error("the route of the domain that stays, or the Service, is gone")
	}

	env.load()
	env.mustReconcile(shopApp)
	if !env.exists(&networkingv1.Ingress{}, "shop", routeName("app", 1, "new.example.com").name) {
		t.Error("no route of the new domain after the instance loaded the record")
	}
}

// TestReconcile_InstanceThatDoesNotAnswer: an object that is served stays served while its
// instance tells nothing, and a record the pods were seen to serve is not asked about again.
func TestReconcile_InstanceThatDoesNotAnswer(t *testing.T) {
	t.Parallel()

	obj := authenticatorObject("shop", "app", 0, "app.example.com")
	env := newTestEnv(t, testSettings, false, obj, authenticatorObject("shop", "fresh", 1, "fresh.example.com"))
	env.serve(shopApp)

	env.instances.err = errors.New("a pod does not answer")
	if result := env.mustReconcile(shopApp); result != (reconcile.Result{}) {
		t.Errorf("an object whose record was seen loaded: %+v", result)
	}
	if got := env.condition(shopApp); got.Status != metav1.ConditionTrue {
		t.Errorf("condition = %+v", got)
	}

	// The controller starts again and remembers nothing: the object is still not called unserved.
	restarted := New(env.client, env.client, env.recorder, logr.Discard(), env.objects.state, env.instances, false)
	result, err := restarted.Reconcile(t.Context(), reconcile.Request{NamespacedName: shopApp})
	if err != nil || result.RequeueAfter != loadRecheck {
		t.Errorf("after a restart: %+v, %v; want a later look", result, err)
	}
	if got := env.condition(shopApp); got.Status != metav1.ConditionTrue {
		t.Errorf("condition after a restart = %+v", got)
	}

	// An object that was never served waits, and says so.
	fresh := keyOf("shop", "fresh")
	if result := env.mustReconcile(fresh); result.RequeueAfter != loadRecheck {
		t.Errorf("a new object: %+v", result)
	}
	if got := env.condition(fresh); got.Status != metav1.ConditionFalse || got.Reason != ReasonWaitingForInstance {
		t.Errorf("condition of a new object = %+v", got)
	}
	if env.exists(&corev1.Service{}, "shop", "fresh-dex-authenticator") {
		t.Error("a Service for a record nobody was seen to serve")
	}

	// A deleted object is forgotten: another one of its name is asked about anew.
	if err := env.client.Delete(t.Context(), obj); err != nil {
		t.Fatal(err)
	}
	env.mustReconcile(shopApp)
	if env.objects.isLoaded(shopApp, Entry{UID: obj.GetUID()}) {
		t.Error("the record of a deleted object is remembered as loaded")
	}
}

// TestReconcile_RouteOfADomainThatMovesUp: the name of a route depends on the place of its
// application in the object, so a domain that becomes the first one gets a route under another
// name. The route it has stays until the new one is written, once the instance has loaded the
// record: until then the domain is served and must not lose its route.
func TestReconcile_RouteOfADomainThatMovesUp(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t, testSettings, false, authenticatorObject("shop", "app", 0, "d0.example.com", "d1.example.com"))
	env.serve(shopApp)

	routes := func() map[string]string {
		t.Helper()
		list := &networkingv1.IngressList{}
		if err := env.client.List(t.Context(), list, client.InNamespace("shop")); err != nil {
			t.Fatal(err)
		}
		out := map[string]string{}
		for _, item := range list.Items {
			out[item.Name] = item.Spec.Rules[0].Host
		}
		return out
	}
	second := routeName("app", 1, "d1.example.com").name
	if got, want := routes(), map[string]string{shopService: "d0.example.com", second: "d1.example.com"}; !maps.Equal(got, want) {
		t.Fatalf("routes = %v, want %v", got, want)
	}

	// The object drops its first domain.
	live := controller.Object(controller.DexAuthenticatorGVK)
	env.exists(live, "shop", "app")
	spec, _ := live.Object["spec"].(map[string]any)
	spec["applicationDomain"] = "d1.example.com"
	delete(spec, "additionalApplications")
	if err := env.client.Update(t.Context(), live); err != nil {
		t.Fatal(err)
	}
	env.now = testNow.Add(time.Hour)
	env.buildTables()

	// The instance has not loaded the new record: the route of the domain that stays is there.
	env.mustReconcile(shopApp)
	if got := env.condition(shopApp); got.Reason != ReasonWaitingForInstance {
		t.Fatalf("condition = %+v", got)
	}
	if got := routes(); got[second] != "d1.example.com" {
		t.Errorf("routes before the instance loaded the record = %v: the domain that stays lost its route", got)
	}

	// The instance has loaded it: the domain has the route of the first application, and only it.
	env.load()
	env.mustReconcile(shopApp)
	if got, want := routes(), map[string]string{shopService: "d1.example.com"}; !maps.Equal(got, want) {
		t.Errorf("routes = %v, want %v", got, want)
	}
	if got := env.condition(shopApp); got.Status != metav1.ConditionTrue {
		t.Errorf("condition = %+v", got)
	}
}

// TestReconcile_RouteOfADroppedDomainGoesAtOnce: the route of a domain the object no longer names
// does not wait for the instance.
func TestReconcile_RouteOfADroppedDomainGoesAtOnce(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t, testSettings, false, authenticatorObject("shop", "app", 0, "d0.example.com", "d1.example.com"))
	env.serve(shopApp)

	live := controller.Object(controller.DexAuthenticatorGVK)
	env.exists(live, "shop", "app")
	spec, _ := live.Object["spec"].(map[string]any)
	delete(spec, "additionalApplications")
	if err := env.client.Update(t.Context(), live); err != nil {
		t.Fatal(err)
	}
	env.now = testNow.Add(time.Hour)
	env.buildTables()
	env.mustReconcile(shopApp)

	if env.exists(&networkingv1.Ingress{}, "shop", routeName("app", 1, "d1.example.com").name) {
		t.Error("the route of the domain the object dropped is left until the instance loads the record")
	}
	if !env.exists(&networkingv1.Ingress{}, "shop", shopService) {
		t.Error("the route of the domain that stays is gone")
	}
}

// TestReconcile_RouteOfAChangedFirstDomainGoesAtOnce: the route of the first application has no
// hash of the domain in its name, so the route of a new first domain has the name of the route of
// the old one. The route of the old domain does not wait for the instance under that name: the
// record has lost the domain, and the route would keep it from another object.
func TestReconcile_RouteOfAChangedFirstDomainGoesAtOnce(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t, testSettings, false, authenticatorObject("shop", "app", 0, "d0.example.com"))
	env.serve(shopApp)

	live := controller.Object(controller.DexAuthenticatorGVK)
	env.exists(live, "shop", "app")
	spec, _ := live.Object["spec"].(map[string]any)
	spec["applicationDomain"] = "d1.example.com"
	if err := env.client.Update(t.Context(), live); err != nil {
		t.Fatal(err)
	}
	env.now = testNow.Add(time.Hour)
	env.buildTables()

	env.mustReconcile(shopApp)
	if got := env.condition(shopApp); got.Reason != ReasonWaitingForInstance {
		t.Fatalf("condition = %+v", got)
	}
	if env.exists(&networkingv1.Ingress{}, "shop", shopService) {
		t.Error("the route of the domain the object dropped is left until the instance loads the record")
	}

	env.load()
	env.mustReconcile(shopApp)
	ingress := &networkingv1.Ingress{}
	if !env.exists(ingress, "shop", shopService) {
		t.Fatal("no route of the new domain after the instance loaded the record")
	}
	if host := ingress.Spec.Rules[0].Host; host != "d1.example.com" {
		t.Errorf("host of the route = %q", host)
	}
}

// TestReconcile_RouteOfADroppedPathGoesAtOnce: a route of a domain the object still names stays
// until the instance loads the record only when it leads a path the object still has a route of.
// The route of a sign-out address the object no longer has goes at once.
func TestReconcile_RouteOfADroppedPathGoesAtOnce(t *testing.T) {
	t.Parallel()

	obj := authenticatorObject("shop", "app", 0, "app.example.com")
	spec, _ := obj.Object["spec"].(map[string]any)
	spec["signOutURL"] = "/logout"
	env := newTestEnv(t, testSettings, false, obj)
	env.serve(shopApp)
	signOut := signOutRouteName("app", 0, "app.example.com").name
	if !env.exists(&networkingv1.Ingress{}, "shop", signOut) {
		t.Fatal("no route of the sign-out address")
	}

	// The object drops the sign-out address and names one more domain: its record changes.
	live := controller.Object(controller.DexAuthenticatorGVK)
	env.exists(live, "shop", "app")
	spec, _ = live.Object["spec"].(map[string]any)
	delete(spec, "signOutURL")
	spec["additionalApplications"] = []any{map[string]any{"domain": "new.example.com", "ingressClassName": "nginx"}}
	if err := env.client.Update(t.Context(), live); err != nil {
		t.Fatal(err)
	}
	env.now = testNow.Add(time.Hour)
	env.buildTables()

	env.mustReconcile(shopApp)
	if got := env.condition(shopApp); got.Reason != ReasonWaitingForInstance {
		t.Fatalf("condition = %+v", got)
	}
	if env.exists(&networkingv1.Ingress{}, "shop", signOut) {
		t.Error("the route of the sign-out address the object dropped is left until the instance loads the record")
	}
	if !env.exists(&networkingv1.Ingress{}, "shop", shopService) {
		t.Error("the route of the domain that stays is gone")
	}
}

// TestReconcile_ChangedSpecWaitsForTheDecisionAboutIt: after the spec of a served object changes,
// the reconciler of its objects leaves the condition as it is until the table reconciler has
// decided about the new generation. Acting on the decision about the previous generation would
// write the new generation as observed with the reason Served for a spec no instance serves yet.
func TestReconcile_ChangedSpecWaitsForTheDecisionAboutIt(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t, testSettings, false, authenticatorObject("shop", "app", 0, "d0.example.com"))
	env.serve(shopApp)
	before := env.condition(shopApp)

	live := controller.Object(controller.DexAuthenticatorGVK)
	env.exists(live, "shop", "app")
	spec, _ := live.Object["spec"].(map[string]any)
	spec["applicationDomain"] = "d1.example.com"
	// The fake client does not count generations: the API server raises it on a change of spec.
	live.SetGeneration(live.GetGeneration() + 1)
	if err := env.client.Update(t.Context(), live); err != nil {
		t.Fatal(err)
	}
	env.exists(live, "shop", "app")
	generation := live.GetGeneration()
	if generation <= before.ObservedGeneration {
		t.Fatalf("generation = %d, observed before = %d", generation, before.ObservedGeneration)
	}

	// The table reconciler has not run yet: its decision is about the previous generation.
	if result := env.mustReconcile(shopApp); result.RequeueAfter == 0 {
		t.Errorf("result = %+v, want a recheck", result)
	}
	if got := env.condition(shopApp); got != before {
		t.Fatalf("condition before the decision about generation %d = %+v, want it untouched: %+v", generation, got, before)
	}

	env.now = testNow.Add(time.Hour)
	env.buildTables()
	env.mustReconcile(shopApp)
	if got := env.condition(shopApp); got.Reason != ReasonWaitingForInstance || got.ObservedGeneration != generation {
		t.Fatalf("condition after the decision = %+v", got)
	}

	env.load()
	env.mustReconcile(shopApp)
	if got := env.condition(shopApp); got.Status != metav1.ConditionTrue || got.ObservedGeneration != generation {
		t.Fatalf("condition after the instance loaded the record = %+v", got)
	}
}

// TestTableReconciler_LostTableKeepsTheRoutedHost: the table Secrets are lost while an older
// object of another namespace names the host of a served object. The table written anew gives the
// host to the object the routes of the controller lead to by it, not to the older one, and the
// served object keeps its routes. Only a route with the heritage label is evidence: one without
// it, as a user of the namespace may create, claims nothing.
func TestTableReconciler_LostTableKeepsTheRoutedHost(t *testing.T) {
	t.Parallel()

	evil := keyOf("evil", "old")
	env := newTestEnv(t, testSettings, false,
		authenticatorObject("evil", "old", 1, "e.example.com"),
		authenticatorObject("shop", "app", 100, "d.example.com"))
	env.serve(evil)
	env.mustReconcile(shopApp)
	if got := env.condition(shopApp); got.Status != metav1.ConditionTrue {
		t.Fatalf("the victim is not served: %+v", got)
	}

	// evil/old names the host of the served object ahead, with a route of its own for it.
	obj := controller.Object(controller.DexAuthenticatorGVK)
	env.exists(obj, "evil", "old")
	spec, _ := obj.Object["spec"].(map[string]any)
	spec["additionalApplications"] = []any{map[string]any{"domain": "d.example.com", "ingressClassName": "nginx"}}
	obj.SetGeneration(2)
	if err := env.client.Update(t.Context(), obj); err != nil {
		t.Fatal(err)
	}
	forged := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "forged", Namespace: "evil", Labels: map[string]string{
			LabelFor: "old", adoption.ManagedByLabel: adoption.ManagedByValue,
		}},
		Spec: networkingv1.IngressSpec{Rules: []networkingv1.IngressRule{{Host: "d.example.com"}}},
	}
	if err := env.client.Create(t.Context(), forged); err != nil {
		t.Fatal(err)
	}
	env.buildTables()
	if record, _ := env.table(naming.UserAuthenticatorInstance).record(evil); slices.Contains(record.Hosts, "d.example.com") {
		t.Fatalf("the older object got the host of the served one: %+v", record)
	}

	for _, instance := range naming.AuthenticatorInstances() {
		secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: naming.DexNamespace, Name: TableSecretName(instance)}}
		if err := env.client.Delete(t.Context(), secret); err != nil {
			t.Fatal(err)
		}
	}
	env.now = testNow.Add(time.Hour)
	env.buildTables()

	written := env.table(naming.UserAuthenticatorInstance)
	if record, found := written.record(shopApp); !found || !slices.Equal(record.Hosts, []string{"d.example.com"}) {
		t.Errorf("the served object after the table was lost: %+v, %v", record, found)
	}
	if record, _ := written.record(evil); !slices.Equal(record.Hosts, []string{"e.example.com"}) {
		t.Errorf("the older object after the table was lost: %+v", record)
	}
	env.load()
	env.mustReconcile(shopApp)
	if !env.exists(&networkingv1.Ingress{}, "shop", shopService) {
		t.Error("the routes of the served object are gone")
	}
}
