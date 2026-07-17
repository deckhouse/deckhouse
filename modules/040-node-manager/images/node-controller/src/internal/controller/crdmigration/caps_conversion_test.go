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

package crdmigration

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/node-controller/internal/register"
)

func newCAPSReconciler(t *testing.T, objs ...client.Object) *Reconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 scheme: %v", err)
	}
	if err := apiextensionsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add apiextensionsv1 scheme: %v", err)
	}
	cl := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return &Reconciler{Base: register.Base{Client: cl, Recorder: record.NewFakeRecorder(10)}, apiReader: cl}
}

func capsService() *corev1.Service {
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: capiNamespace, Name: capsWebhookServiceName}}
}

func capsSecret(ca string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: capiNamespace, Name: capsWebhookSecretName},
		Data:       map[string][]byte{"ca.crt": []byte(ca)},
	}
}

func sshCRD(conv *apiextensionsv1.CustomResourceConversion) *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: capsConversionCRDName},
		Spec:       apiextensionsv1.CustomResourceDefinitionSpec{Conversion: conv},
	}
}

func currentCAPSConversion(ca string) *apiextensionsv1.CustomResourceConversion {
	return &apiextensionsv1.CustomResourceConversion{
		Strategy: apiextensionsv1.WebhookConverter,
		Webhook: &apiextensionsv1.WebhookConversion{
			ClientConfig: &apiextensionsv1.WebhookClientConfig{
				Service: &apiextensionsv1.ServiceReference{
					Name:      capsWebhookServiceName,
					Namespace: capiNamespace,
					Path:      ptrString("/convert"),
					Port:      ptrInt32(443),
				},
				CABundle: []byte(ca),
			},
			ConversionReviewVersions: []string{"v1"},
		},
	}
}

func reconcileCAPS(t *testing.T, r *Reconciler) ctrl.Result {
	t.Helper()
	res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: capsConversionCRDName}})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	return res
}

func getSSHConversion(t *testing.T, r *Reconciler) *apiextensionsv1.CustomResourceConversion {
	t.Helper()
	crd := &apiextensionsv1.CustomResourceDefinition{}
	if err := r.Client.Get(context.Background(), types.NamespacedName{Name: capsConversionCRDName}, crd); err != nil {
		t.Fatalf("get crd: %v", err)
	}
	return crd.Spec.Conversion
}

// With Service + Secret + CRD present, the conversion webhook is written with the CAPS
// service, CA bundle, /convert path, port 443 and conversionReviewVersions [v1].
func TestReconcileCAPS_InjectsCA(t *testing.T) {
	r := newCAPSReconciler(t, capsService(), capsSecret("CA-DATA"), sshCRD(nil))
	reconcileCAPS(t, r)

	conv := getSSHConversion(t, r)
	if conv == nil || conv.Strategy != apiextensionsv1.WebhookConverter {
		t.Fatalf("expected webhook conversion, got %+v", conv)
	}
	cc := conv.Webhook.ClientConfig
	if cc.Service.Name != capsWebhookServiceName || cc.Service.Namespace != capiNamespace {
		t.Fatalf("wrong service ref: %+v", cc.Service)
	}
	if *cc.Service.Path != "/convert" || *cc.Service.Port != 443 {
		t.Fatalf("wrong path/port: %v %v", *cc.Service.Path, *cc.Service.Port)
	}
	if string(cc.CABundle) != "CA-DATA" {
		t.Fatalf("wrong CA bundle: %q", string(cc.CABundle))
	}
	if len(conv.Webhook.ConversionReviewVersions) != 1 || conv.Webhook.ConversionReviewVersions[0] != "v1" {
		t.Fatalf("wrong review versions: %v", conv.Webhook.ConversionReviewVersions)
	}
}

// A stale CA bundle is corrected to the current secret CA.
func TestReconcileCAPS_CorrectsStaleCA(t *testing.T) {
	r := newCAPSReconciler(t, capsService(), capsSecret("NEW-CA"), sshCRD(currentCAPSConversion("OLD-CA")))
	reconcileCAPS(t, r)

	if got := string(getSSHConversion(t, r).Webhook.ClientConfig.CABundle); got != "NEW-CA" {
		t.Fatalf("expected NEW-CA, got %q", got)
	}
}

// An already-correct CRD stays unchanged (idempotent).
func TestReconcileCAPS_Idempotent(t *testing.T) {
	r := newCAPSReconciler(t, capsService(), capsSecret("CA-DATA"), sshCRD(currentCAPSConversion("CA-DATA")))
	reconcileCAPS(t, r)

	if got := string(getSSHConversion(t, r).Webhook.ClientConfig.CABundle); got != "CA-DATA" {
		t.Fatalf("expected CA-DATA, got %q", got)
	}
}

// The webhook Service gates the patch: absent Service → requeue, CRD untouched.
func TestReconcileCAPS_MissingService_Requeue(t *testing.T) {
	r := newCAPSReconciler(t, capsSecret("CA-DATA"), sshCRD(currentCAPSConversion("OLD-CA")))
	res := reconcileCAPS(t, r)

	if res.RequeueAfter != requeuePrecondition {
		t.Fatalf("expected requeue %v, got %v", requeuePrecondition, res.RequeueAfter)
	}
	if got := string(getSSHConversion(t, r).Webhook.ClientConfig.CABundle); got != "OLD-CA" {
		t.Fatalf("expected untouched OLD-CA, got %q", got)
	}
}

// Absent Secret → requeue, CRD untouched.
func TestReconcileCAPS_MissingSecret_Requeue(t *testing.T) {
	r := newCAPSReconciler(t, capsService(), sshCRD(currentCAPSConversion("OLD-CA")))
	res := reconcileCAPS(t, r)

	if res.RequeueAfter != requeuePrecondition {
		t.Fatalf("expected requeue %v, got %v", requeuePrecondition, res.RequeueAfter)
	}
	if got := string(getSSHConversion(t, r).Webhook.ClientConfig.CABundle); got != "OLD-CA" {
		t.Fatalf("expected untouched OLD-CA, got %q", got)
	}
}

// Empty ca.crt → requeue, CRD untouched.
func TestReconcileCAPS_EmptyCA_Requeue(t *testing.T) {
	r := newCAPSReconciler(t, capsService(), capsSecret(""), sshCRD(currentCAPSConversion("OLD-CA")))
	res := reconcileCAPS(t, r)

	if res.RequeueAfter != requeuePrecondition {
		t.Fatalf("expected requeue %v, got %v", requeuePrecondition, res.RequeueAfter)
	}
}

// Preconditions met but the CRD does not exist yet → skip without error (patch is a no-op).
func TestReconcileCAPS_MissingCRD_NoError(t *testing.T) {
	r := newCAPSReconciler(t, capsService(), capsSecret("CA-DATA"))
	res := reconcileCAPS(t, r)

	if res.RequeueAfter != 0 {
		t.Fatalf("expected no requeue, got %v", res.RequeueAfter)
	}
}

func TestIsConversionWebhookCurrent_CAPS(t *testing.T) {
	ca := []byte("CA-DATA")
	if isConversionWebhookCurrent(sshCRD(nil), ca, capsWebhookServiceName) {
		t.Fatal("nil conversion must be not-current")
	}
	if !isConversionWebhookCurrent(sshCRD(currentCAPSConversion("CA-DATA")), ca, capsWebhookServiceName) {
		t.Fatal("matching conversion must be current")
	}
	if isConversionWebhookCurrent(sshCRD(currentCAPSConversion("OTHER")), ca, capsWebhookServiceName) {
		t.Fatal("different CA must be not-current")
	}
	if isConversionWebhookCurrent(sshCRD(currentCAPSConversion("CA-DATA")), ca, nodeControllerWebhookServiceName) {
		t.Fatal("different service must be not-current")
	}
}
