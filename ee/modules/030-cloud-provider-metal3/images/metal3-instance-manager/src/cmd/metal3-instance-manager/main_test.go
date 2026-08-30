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
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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
	scheme.AddKnownTypeWithName(metal3InstanceGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(metal3InstanceGVK.GroupVersion().WithKind("Metal3InstanceList"), &unstructured.UnstructuredList{})
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
	assertNestedString(t, bmh, "redfish+https://192.0.2.10/redfish/v1/Systems/1", "spec", "bmc", "address")
	assertNestedString(t, bmh, "server-bmh-credentials", "spec", "bmc", "credentialsName")
	assertNestedString(t, bmh, "f2:4e:c6:e6:af:ac", "spec", "bootMACAddress")
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
		Online:         true,
		BootMACAddress: "f2:4e:c6:e6:af:ac",
		BMC:            BMCConfig{Insecure: true},
	}, ResolvedBMC{Address: "ipmi://192.0.2.10:623"}, "new")
	if err != nil || !changed {
		t.Fatalf("update BMH: changed=%v err=%v", changed, err)
	}
	assertNestedString(t, bmh, "/dev/sda", "spec", "rootDeviceHints", "deviceName")
	assertNestedString(t, bmh, "metadata", "spec", "automatedCleaningMode")
	assertNestedString(t, bmh, "ipmi://192.0.2.10:623", "spec", "bmc", "address")
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
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "Metal3Instance",
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
	instance.SetGroupVersionKind(metal3InstanceGVK)
	return instance
}

func assertNestedString(t *testing.T, obj *unstructured.Unstructured, want string, fields ...string) {
	t.Helper()
	got, found, err := unstructured.NestedString(obj.Object, fields...)
	if err != nil || !found || got != want {
		t.Fatalf("field %s: want %q, got %q, found=%v, err=%v", strings.Join(fields, "."), want, got, found, err)
	}
}

func writeJSON(value interface{}) http.HandlerFunc {
	return func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(value)
	}
}
