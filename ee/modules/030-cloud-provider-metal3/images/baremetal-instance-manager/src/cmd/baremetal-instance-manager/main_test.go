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

package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const testSystemUUID = "00583f1d-2903-e203-0010-d21de0c7571b"

type staticResolver struct {
	resolved ResolvedBMC
	err      error
	calls    int
}

func (r *staticResolver) Resolve(_ context.Context, _ BMCConfig, _, _ string) (ResolvedBMC, error) {
	r.calls++
	return r.resolved, r.err
}

func TestReconcileCreatesResolvedBareMetalHost(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	scheme.AddKnownTypeWithName(bareMetalInstanceGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(bareMetalInstanceGVK.GroupVersion().WithKind("BareMetalInstanceList"), &unstructured.UnstructuredList{})
	scheme.AddKnownTypeWithName(bareMetalHostGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(bareMetalHostGVK.GroupVersion().WithKind("BareMetalHostList"), &unstructured.UnstructuredList{})

	instance := testInstance()
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "server-bmc", Namespace: "d8-cloud-instance-manager"},
		Type:       corev1.SecretType(credentialsSecretType),
		Data: map[string][]byte{
			"authScheme": []byte(authSchemeUserPassword),
			"identity":   []byte("admin"),
			"secret":     []byte("password"),
		},
	}
	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(instance, secret).
		WithStatusSubresource(instance).
		Build()
	resolver := &staticResolver{resolved: ResolvedBMC{
		Protocol:   "Redfish",
		Address:    "redfish+https://192.0.2.10/redfish/v1/Systems/1",
		SystemUUID: testSystemUUID,
	}}
	r := &reconciler{Client: kubeClient, targetNamespace: "d8-cloud-instance-manager", resolver: resolver}
	request := ctrl.Request{NamespacedName: types.NamespacedName{Name: "server", Namespace: "d8-cloud-instance-manager"}}

	if _, err := r.Reconcile(context.Background(), request); err != nil {
		t.Fatalf("reconcile resources: %v", err)
	}
	updatedAfterFirstReconcile := &unstructured.Unstructured{}
	updatedAfterFirstReconcile.SetGroupVersionKind(bareMetalInstanceGVK)
	if err := kubeClient.Get(context.Background(), request.NamespacedName, updatedAfterFirstReconcile); err != nil {
		t.Fatalf("get instance after first reconcile: %v", err)
	}
	conditions, found, err := unstructured.NestedSlice(updatedAfterFirstReconcile.Object, "status", "conditions")
	if err != nil || !found || len(conditions) != 1 {
		t.Fatalf("expected BMCResolved condition after first reconcile, found=%v conditions=%#v err=%v", found, conditions, err)
	}
	firstCondition := conditions[0].(map[string]interface{})
	if firstCondition["type"] != "BMCResolved" || firstCondition["status"] != "True" || firstCondition["reason"] != "BMCResolved" {
		t.Fatalf("unexpected first condition: %#v", firstCondition)
	}
	statusResourceVersion := updatedAfterFirstReconcile.GetResourceVersion()
	if _, err := r.Reconcile(context.Background(), request); err != nil {
		t.Fatalf("reconcile cached BMC: %v", err)
	}

	bmh := &unstructured.Unstructured{}
	bmh.SetGroupVersionKind(bareMetalHostGVK)
	if err := kubeClient.Get(context.Background(), request.NamespacedName, bmh); err != nil {
		t.Fatalf("get generated BMH: %v", err)
	}
	assertNestedString(t, bmh, "redfish+https://192.0.2.10/redfish/v1/Systems/1", "spec", "bmc", "address")
	assertNestedString(t, bmh, "server-bmh-credentials", "spec", "bmc", "credentialsName")
	assertNestedString(t, bmh, "f2:4e:c6:e6:af:ac", "spec", "bootMACAddress")
	assertNestedMap(t, bmh, map[string]interface{}{}, "spec", "rootDeviceHints")
	if got := bmh.GetLabels()["pool"]; got != "workers" {
		t.Fatalf("expected pool label workers, got %q", got)
	}
	if got := bmh.GetAnnotations()[annotationInstance]; got != "server" {
		t.Fatalf("expected ownership annotation server, got %q", got)
	}

	generated := &corev1.Secret{}
	if err := kubeClient.Get(context.Background(), types.NamespacedName{Name: "server-bmh-credentials", Namespace: "d8-cloud-instance-manager"}, generated); err != nil {
		t.Fatalf("get translated credentials: %v", err)
	}
	if string(generated.Data["username"]) != "admin" || string(generated.Data["password"]) != "password" {
		t.Fatalf("unexpected translated credentials keys: %#v", generated.Data)
	}
	if resolver.calls != 1 {
		t.Fatalf("expected one resolver call, got %d", resolver.calls)
	}

	if _, err := r.Reconcile(context.Background(), request); err != nil {
		t.Fatalf("reconcile cached BMC: %v", err)
	}
	if resolver.calls != 1 {
		t.Fatalf("expected resolved BMC to be cached, got %d calls", resolver.calls)
	}
	updatedInstance := &unstructured.Unstructured{}
	updatedInstance.SetGroupVersionKind(bareMetalInstanceGVK)
	if err := kubeClient.Get(context.Background(), request.NamespacedName, updatedInstance); err != nil {
		t.Fatalf("get updated instance: %v", err)
	}
	if updatedInstance.GetResourceVersion() != statusResourceVersion {
		t.Fatalf("unchanged reconcile rewrote instance: resourceVersion %q -> %q", statusResourceVersion, updatedInstance.GetResourceVersion())
	}
	resolvedAt, found, err := unstructured.NestedString(updatedInstance.Object, "status", "bmc", "lastResolvedTime")
	if err != nil || !found {
		t.Fatalf("expected BMC resolution timestamp, found=%v err=%v", found, err)
	}
	if _, err := time.Parse(time.RFC3339, resolvedAt); err != nil {
		t.Fatalf("parse BMC resolution timestamp %q: %v", resolvedAt, err)
	}
	if hostName, found, err := unstructured.NestedString(updatedInstance.Object, "status", "host", "name"); err != nil || !found || hostName != "server" {
		t.Fatalf("expected status.host.name=server, got %q found=%v err=%v", hostName, found, err)
	}
	if _, found, err := unstructured.NestedMap(updatedInstance.Object, "status", "bareMetalHost"); err != nil || found {
		t.Fatalf("obsolete status.bareMetalHost is present: found=%v err=%v", found, err)
	}

	updatedInstance.Object["status"] = map[string]interface{}{}
	if err := kubeClient.Status().Update(context.Background(), updatedInstance); err != nil {
		t.Fatalf("clear instance status: %v", err)
	}
	if _, err := r.Reconcile(context.Background(), request); err != nil {
		t.Fatalf("restore cleared status: %v", err)
	}
	if resolver.calls != 2 {
		t.Fatalf("expected BMC resolution after status loss, got %d calls", resolver.calls)
	}
	restoredInstance := &unstructured.Unstructured{}
	restoredInstance.SetGroupVersionKind(bareMetalInstanceGVK)
	if err := kubeClient.Get(context.Background(), request.NamespacedName, restoredInstance); err != nil {
		t.Fatalf("get restored instance: %v", err)
	}
	if hostName, found, err := unstructured.NestedString(restoredInstance.Object, "status", "host", "name"); err != nil || !found || hostName != "server" {
		t.Fatalf("expected restored status.host.name=server, got %q found=%v err=%v", hostName, found, err)
	}
	if _, found, err := unstructured.NestedString(restoredInstance.Object, "status", "bmc", "address"); err != nil || !found {
		t.Fatalf("expected restored BMC status, found=%v err=%v", found, err)
	}
}

func TestEnsureCredentialSecretRejectsForeignSecret(t *testing.T) {
	instance := testInstance()
	foreign := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "server-bmh-credentials", Namespace: "d8-cloud-instance-manager"},
		Type:       corev1.SecretTypeOpaque,
		Data:       map[string][]byte{"username": []byte("foreign"), "password": []byte("foreign")},
	}
	kubeClient := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(foreign).Build()
	r := &reconciler{Client: kubeClient, targetNamespace: "d8-cloud-instance-manager"}

	err := r.ensureCredentialSecret(context.Background(), instance, credentials{Username: "admin", Password: "password"}, foreign.Name)
	if !errors.Is(err, errManagedResourceConflict) {
		t.Fatalf("expected ownership conflict, got %v", err)
	}
	actual := &corev1.Secret{}
	if err := kubeClient.Get(context.Background(), types.NamespacedName{Namespace: foreign.Namespace, Name: foreign.Name}, actual); err != nil {
		t.Fatal(err)
	}
	if string(actual.Data["username"]) != "foreign" || len(actual.GetAnnotations()) != 0 {
		t.Fatalf("foreign Secret was modified: %#v", actual)
	}
}

func TestEnsureBareMetalHostRejectsForeignHost(t *testing.T) {
	instance := testInstance()
	foreign := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "metal3.io/v1alpha1",
		"kind":       "BareMetalHost",
		"metadata": map[string]interface{}{
			"name": "server", "namespace": "d8-cloud-instance-manager",
		},
		"spec": map[string]interface{}{"online": false},
	}}
	foreign.SetGroupVersionKind(bareMetalHostGVK)
	kubeClient := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(foreign).Build()
	r := &reconciler{Client: kubeClient, targetNamespace: "d8-cloud-instance-manager"}

	_, err := r.ensureBareMetalHost(context.Background(), instance, instanceSpec{Online: true}, ResolvedBMC{Address: "ipmi://192.0.2.10:623"}, "server-bmh-credentials")
	if !errors.Is(err, errManagedResourceConflict) {
		t.Fatalf("expected ownership conflict, got %v", err)
	}
	actual := &unstructured.Unstructured{}
	actual.SetGroupVersionKind(bareMetalHostGVK)
	if err := kubeClient.Get(context.Background(), types.NamespacedName{Namespace: "d8-cloud-instance-manager", Name: "server"}, actual); err != nil {
		t.Fatal(err)
	}
	assertNestedBool(t, actual, false, "spec", "online")
	if len(actual.GetAnnotations()) != 0 {
		t.Fatalf("foreign BareMetalHost was modified: %#v", actual.GetAnnotations())
	}
}

func TestReconcileReportsManagedResourceConflict(t *testing.T) {
	instance := testInstance()
	controllerutil.AddFinalizer(instance, finalizerName)
	credentialsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "server-bmc", Namespace: "d8-cloud-instance-manager"},
		Type:       corev1.SecretType(credentialsSecretType),
		Data: map[string][]byte{
			"authScheme": []byte(authSchemeUserPassword),
			"identity":   []byte("admin"),
			"secret":     []byte("password"),
		},
	}
	foreignGeneratedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "server-bmh-credentials", Namespace: "d8-cloud-instance-manager"},
		Type:       corev1.SecretTypeOpaque,
	}
	kubeClient := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(instance, credentialsSecret, foreignGeneratedSecret).WithStatusSubresource(instance).Build()
	r := &reconciler{
		Client:          kubeClient,
		targetNamespace: "d8-cloud-instance-manager",
		resolver:        &staticResolver{resolved: ResolvedBMC{Protocol: "Redfish", Address: "redfish+https://192.0.2.10/redfish/v1/Systems/1", SystemUUID: testSystemUUID}},
	}
	request := ctrl.Request{NamespacedName: types.NamespacedName{Name: "server", Namespace: "d8-cloud-instance-manager"}}

	if _, err := r.Reconcile(context.Background(), request); !errors.Is(err, errManagedResourceConflict) {
		t.Fatalf("expected ownership conflict, got %v", err)
	}
	updated := &unstructured.Unstructured{}
	updated.SetGroupVersionKind(bareMetalInstanceGVK)
	if err := kubeClient.Get(context.Background(), request.NamespacedName, updated); err != nil {
		t.Fatal(err)
	}
	conditions, found, err := unstructured.NestedSlice(updated.Object, "status", "conditions")
	if err != nil || !found || len(conditions) != 1 {
		t.Fatalf("expected one condition, found=%v err=%v conditions=%#v", found, err, conditions)
	}
	condition, ok := conditions[0].(map[string]interface{})
	if !ok || condition["status"] != "False" || condition["reason"] != "ManagedResourceConflict" {
		t.Fatalf("unexpected condition: %#v", conditions[0])
	}
	assertNestedString(t, updated, "A generated resource with the required name is owned by another object.", "status", "message")
}

func TestReconcileDeleteRejectsForeignBareMetalHost(t *testing.T) {
	instance := testInstance()
	now := metav1.NewTime(time.Now())
	instance.SetDeletionTimestamp(&now)
	controllerutil.AddFinalizer(instance, finalizerName)
	foreign := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "metal3.io/v1alpha1",
		"kind":       "BareMetalHost",
		"metadata": map[string]interface{}{
			"name": "server", "namespace": "d8-cloud-instance-manager",
		},
	}}
	foreign.SetGroupVersionKind(bareMetalHostGVK)
	kubeClient := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(instance, foreign).Build()
	r := &reconciler{Client: kubeClient, targetNamespace: "d8-cloud-instance-manager"}

	_, err := r.reconcileDelete(context.Background(), instance)
	if !errors.Is(err, errManagedResourceConflict) {
		t.Fatalf("expected ownership conflict, got %v", err)
	}
	actual := &unstructured.Unstructured{}
	actual.SetGroupVersionKind(bareMetalHostGVK)
	if err := kubeClient.Get(context.Background(), types.NamespacedName{Namespace: "d8-cloud-instance-manager", Name: "server"}, actual); err != nil {
		t.Fatalf("foreign BareMetalHost was deleted: %v", err)
	}
}

func TestReconcileDeleteRejectsForeignCredentialSecret(t *testing.T) {
	instance := testInstance()
	now := metav1.NewTime(time.Now())
	instance.SetDeletionTimestamp(&now)
	controllerutil.AddFinalizer(instance, finalizerName)
	foreign := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "server-bmh-credentials", Namespace: "d8-cloud-instance-manager"}}
	kubeClient := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(instance, foreign).Build()
	r := &reconciler{Client: kubeClient, targetNamespace: "d8-cloud-instance-manager"}

	_, err := r.reconcileDelete(context.Background(), instance)
	if !errors.Is(err, errManagedResourceConflict) {
		t.Fatalf("expected ownership conflict, got %v", err)
	}
	actual := &corev1.Secret{}
	if err := kubeClient.Get(context.Background(), types.NamespacedName{Namespace: foreign.Namespace, Name: foreign.Name}, actual); err != nil {
		t.Fatalf("foreign Secret was deleted: %v", err)
	}
}

func TestBareMetalHostStatusSeparatesDesiredAndObservedPowerState(t *testing.T) {
	bmh := &unstructured.Unstructured{Object: map[string]interface{}{
		"metadata": map[string]interface{}{"name": "server", "namespace": "d8-cloud-instance-manager"},
		"spec":     map[string]interface{}{"online": true},
		"status": map[string]interface{}{
			"poweredOn":    false,
			"errorMessage": "x509: certificate signed by unknown authority",
		},
	}}
	status := (&reconciler{}).bareMetalHostStatus(bmh)
	if status["desiredOnline"] != true || status["poweredOn"] != false {
		t.Fatalf("unexpected power status: %#v", status)
	}
	if status["error"] != "Physical host reported an error." {
		t.Fatalf("unexpected BMH error status: %#v", status["error"])
	}
}

func TestGeneratedSecretNameFitsKubernetesLimit(t *testing.T) {
	instance := &unstructured.Unstructured{}
	instance.SetName(strings.Repeat("a", 253))
	r := &reconciler{}
	first := r.generatedSecretName(instance)
	second := r.generatedSecretName(instance)
	if len(first) > 253 || first != second || !strings.HasSuffix(first, "-bmh-credentials") {
		t.Fatalf("unexpected generated name %q (length %d)", first, len(first))
	}
}

func TestReconcilePassesRootDeviceHintsToBareMetalHost(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	scheme.AddKnownTypeWithName(bareMetalInstanceGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(bareMetalInstanceGVK.GroupVersion().WithKind("BareMetalInstanceList"), &unstructured.UnstructuredList{})
	scheme.AddKnownTypeWithName(bareMetalHostGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(bareMetalHostGVK.GroupVersion().WithKind("BareMetalHostList"), &unstructured.UnstructuredList{})

	instance := testInstance()
	if err := unstructured.SetNestedMap(instance.Object, map[string]interface{}{
		"wwn":          "eui.36363730546091630025384700000001",
		"serialNumber": "S667NG0T609163",
	}, "spec", "rootDeviceHints"); err != nil {
		t.Fatal(err)
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "server-bmc", Namespace: "d8-cloud-instance-manager"},
		Type:       corev1.SecretType(credentialsSecretType),
		Data: map[string][]byte{
			"authScheme": []byte(authSchemeUserPassword),
			"identity":   []byte("admin"),
			"secret":     []byte("password"),
		},
	}
	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(instance, secret).
		WithStatusSubresource(instance).
		Build()
	r := &reconciler{
		Client:          kubeClient,
		targetNamespace: "d8-cloud-instance-manager",
		resolver: &staticResolver{resolved: ResolvedBMC{
			Protocol:   "IPMI",
			Address:    "ipmi://192.0.2.10:623",
			SystemUUID: testSystemUUID,
		}},
	}
	request := ctrl.Request{NamespacedName: types.NamespacedName{Name: "server", Namespace: "d8-cloud-instance-manager"}}

	if _, err := r.Reconcile(context.Background(), request); err != nil {
		t.Fatalf("add finalizer: %v", err)
	}
	if _, err := r.Reconcile(context.Background(), request); err != nil {
		t.Fatalf("reconcile resources: %v", err)
	}

	bmh := &unstructured.Unstructured{}
	bmh.SetGroupVersionKind(bareMetalHostGVK)
	if err := kubeClient.Get(context.Background(), request.NamespacedName, bmh); err != nil {
		t.Fatalf("get generated BMH: %v", err)
	}
	assertNestedMap(t, bmh, map[string]interface{}{
		"wwn":          "eui.36363730546091630025384700000001",
		"serialNumber": "S667NG0T609163",
	}, "spec", "rootDeviceHints")
}

func TestUpdateBareMetalHostPreservesUnmanagedSpec(t *testing.T) {
	bmh := &unstructured.Unstructured{Object: map[string]interface{}{
		"spec": map[string]interface{}{
			"online":                false,
			"rootDeviceHints":       map[string]interface{}{"deviceName": "/dev/sda"},
			"automatedCleaningMode": "metadata",
			"bmc": map[string]interface{}{
				"address":         "redfish+https://old/redfish/v1/Systems/1",
				"credentialsName": "old",
			},
		},
	}}
	changed, err := updateBareMetalHostSpec(bmh, instanceSpec{
		Online:          true,
		BootMACAddress:  "f2:4e:c6:e6:af:ac",
		BMC:             BMCConfig{Insecure: true},
		RootDeviceHints: map[string]interface{}{},
	}, ResolvedBMC{Address: "ipmi://192.0.2.10:623"}, "new")
	if err != nil || !changed {
		t.Fatalf("update BMH: changed=%v err=%v", changed, err)
	}
	assertNestedMap(t, bmh, map[string]interface{}{}, "spec", "rootDeviceHints")
	assertNestedBool(t, bmh, true, "spec", "online")
	assertNestedString(t, bmh, "metadata", "spec", "automatedCleaningMode")
	assertNestedString(t, bmh, "ipmi://192.0.2.10:623", "spec", "bmc", "address")
}

func TestUpdateBareMetalHostPreservesClaimedHostOnlineState(t *testing.T) {
	bmh := &unstructured.Unstructured{Object: map[string]interface{}{
		"spec": map[string]interface{}{
			"online": false,
			"consumerRef": map[string]interface{}{
				"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta1",
				"kind":       "Metal3Machine",
				"name":       "worker-0",
				"namespace":  "d8-cloud-instance-manager",
			},
			"bmc": map[string]interface{}{
				"address":         "redfish+https://old/redfish/v1/Systems/1",
				"credentialsName": "old",
			},
		},
	}}
	changed, err := updateBareMetalHostSpec(bmh, instanceSpec{
		Online:          true,
		BootMACAddress:  "f2:4e:c6:e6:af:ac",
		BMC:             BMCConfig{Insecure: true},
		RootDeviceHints: map[string]interface{}{},
	}, ResolvedBMC{Address: "ipmi://192.0.2.10:623"}, "new")
	if err != nil || !changed {
		t.Fatalf("update BMH: changed=%v err=%v", changed, err)
	}
	assertNestedBool(t, bmh, false, "spec", "online")
	assertNestedString(t, bmh, "ipmi://192.0.2.10:623", "spec", "bmc", "address")
	assertNestedString(t, bmh, "new", "spec", "bmc", "credentialsName")
}

func TestResolveRedfishBySystemUUID(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/redfish/v1/", writeJSON(map[string]interface{}{
		"Systems": map[string]string{"@odata.id": "/redfish/v1/Systems"},
	}))
	mux.HandleFunc("/redfish/v1/Systems", writeJSON(map[string]interface{}{
		"Members": []map[string]string{{"@odata.id": "/redfish/v1/Systems/other"}, {"@odata.id": "/redfish/v1/Systems/target"}},
	}))
	mux.HandleFunc("/redfish/v1/Systems/other", writeJSON(map[string]string{
		"@odata.id": "/redfish/v1/Systems/other", "UUID": "11111111-1111-1111-1111-111111111111",
	}))
	mux.HandleFunc("/redfish/v1/Systems/target", writeJSON(map[string]string{
		"@odata.id": "/redfish/v1/Systems/target", "UUID": strings.ToUpper(testSystemUUID),
	}))
	server := httptest.NewServer(mux)
	defer server.Close()

	host, portString, err := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	port, _ := strconv.Atoi(portString)
	resolved, err := resolveRedfishEndpoint(context.Background(), server.URL, BMCConfig{
		IPAddress: host, Port: port, SystemUUID: testSystemUUID,
	}, "admin", "password")
	if err != nil {
		t.Fatalf("resolve Redfish: %v", err)
	}
	if resolved.Protocol != "Redfish" || resolved.SystemUUID != strings.ToUpper(testSystemUUID) {
		t.Fatalf("unexpected resolved result: %#v", resolved)
	}
	if want := "redfish+" + server.URL + "/redfish/v1/Systems/target"; resolved.Address != want {
		t.Fatalf("expected address %q, got %q", want, resolved.Address)
	}
}

func testInstance() *unstructured.Unstructured {
	instance := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "deckhouse.io/v1",
		"kind":       "BareMetalInstance",
		"metadata": map[string]interface{}{
			"name": "server", "namespace": "d8-cloud-instance-manager", "labels": map[string]interface{}{"pool": "workers"},
		},
		"spec": map[string]interface{}{
			"online": true, "bootMACAddress": "F2:4E:C6:E6:AF:AC",
			"bmc": map[string]interface{}{
				"ipAddress": "192.0.2.10", "systemUUID": testSystemUUID,
				"credentialsRef": map[string]interface{}{"kind": "Secret", "name": "server-bmc"},
			},
		},
	}}
	instance.SetGroupVersionKind(bareMetalInstanceGVK)
	return instance
}

func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	scheme.AddKnownTypeWithName(bareMetalInstanceGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(bareMetalInstanceGVK.GroupVersion().WithKind("BareMetalInstanceList"), &unstructured.UnstructuredList{})
	scheme.AddKnownTypeWithName(bareMetalHostGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(bareMetalHostGVK.GroupVersion().WithKind("BareMetalHostList"), &unstructured.UnstructuredList{})
	return scheme
}

func assertNestedString(t *testing.T, obj *unstructured.Unstructured, want string, fields ...string) {
	t.Helper()
	got, found, err := unstructured.NestedString(obj.Object, fields...)
	if err != nil || !found || got != want {
		t.Fatalf("field %s: want %q, got %q, found=%v, err=%v", strings.Join(fields, "."), want, got, found, err)
	}
}

func assertNestedBool(t *testing.T, obj *unstructured.Unstructured, want bool, fields ...string) {
	t.Helper()
	got, found, err := unstructured.NestedBool(obj.Object, fields...)
	if err != nil || !found || got != want {
		t.Fatalf("field %s: want %v, got %v, found=%v, err=%v", strings.Join(fields, "."), want, got, found, err)
	}
}

func assertNestedMap(t *testing.T, obj *unstructured.Unstructured, want map[string]interface{}, fields ...string) {
	t.Helper()
	got, found, err := unstructured.NestedMap(obj.Object, fields...)
	if err != nil || !found || !reflect.DeepEqual(got, want) {
		t.Fatalf("field %s: want %#v, got %#v, found=%v, err=%v", strings.Join(fields, "."), want, got, found, err)
	}
}

func writeJSON(value interface{}) http.HandlerFunc {
	return func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(value)
	}
}
