/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package controller

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/deckhouse/pkg/log"

	networkv1alpha1 "service-with-healthchecks/api/v1alpha1"
)

const (
	testName      = "test-service"
	testNamespace = "test-namespace"
	lbIPsKey      = "network.deckhouse.io/load-balancer-ips"
	sharedIPKey   = "network.deckhouse.io/load-balancer-shared-ip-key"
)

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add corev1 to scheme: %v", err)
	}
	if err := discoveryv1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add discoveryv1 to scheme: %v", err)
	}
	if err := networkv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add networkv1alpha1 to scheme: %v", err)
	}
	return scheme
}

func newTestSWH(annotations, labels map[string]string) *networkv1alpha1.ServiceWithHealthchecks {
	return &networkv1alpha1.ServiceWithHealthchecks{
		ObjectMeta: metav1.ObjectMeta{
			Name:        testName,
			Namespace:   testNamespace,
			Annotations: annotations,
			Labels:      labels,
		},
		Spec: networkv1alpha1.ServiceWithHealthchecksSpec{
			ServiceSpec: corev1.ServiceSpec{
				Type:  corev1.ServiceTypeLoadBalancer,
				Ports: []corev1.ServicePort{{Name: "http", Port: 80}},
			},
		},
	}
}

func reconcileWith(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()
	scheme := newTestScheme(t)
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		WithStatusSubresource(&networkv1alpha1.ServiceWithHealthchecks{}).
		Build()

	reconciler := &ServiceWithHealthchecksReconciler{
		Client: fakeClient,
		Scheme: scheme,
		Logger: log.NewNop(),
	}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: testName, Namespace: testNamespace},
	}); err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
	return fakeClient
}

func getChildService(t *testing.T, c client.Client) *corev1.Service {
	t.Helper()
	service := &corev1.Service{}
	if err := c.Get(context.Background(), types.NamespacedName{Name: testName, Namespace: testNamespace}, service); err != nil {
		t.Fatalf("failed to get child Service: %v", err)
	}
	return service
}

func TestReconcilePropagatesAnnotationsAndLabelsOnCreate(t *testing.T) {
	swh := newTestSWH(
		map[string]string{
			lbIPsKey:                           "185.11.73.234",
			sharedIPKey:                        "code-e2e-stand",
			corev1.LastAppliedConfigAnnotation: "{}",
		},
		map[string]string{"heritage": "deckhouse"},
	)

	service := getChildService(t, reconcileWith(t, swh))

	if got := service.Annotations[lbIPsKey]; got != "185.11.73.234" {
		t.Errorf("annotation %s = %q, want %q", lbIPsKey, got, "185.11.73.234")
	}
	if got := service.Annotations[sharedIPKey]; got != "code-e2e-stand" {
		t.Errorf("annotation %s = %q, want %q", sharedIPKey, got, "code-e2e-stand")
	}
	if _, found := service.Annotations[corev1.LastAppliedConfigAnnotation]; found {
		t.Errorf("annotation %s must not be propagated", corev1.LastAppliedConfigAnnotation)
	}
	if got := service.Labels["heritage"]; got != "deckhouse" {
		t.Errorf("label heritage = %q, want %q", got, "deckhouse")
	}

	wantTracked := lbIPsKey + "," + sharedIPKey
	if got := service.Annotations[propagatedAnnotationsKey]; got != wantTracked {
		t.Errorf("tracking annotation = %q, want %q", got, wantTracked)
	}
	if got := service.Annotations[propagatedLabelsKey]; got != "heritage" {
		t.Errorf("tracking label annotation = %q, want %q", got, "heritage")
	}
}

func TestReconcileUpdatesAnnotationsOfExistingService(t *testing.T) {
	swh := newTestSWH(map[string]string{lbIPsKey: "185.11.73.234"}, nil)
	existingService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
			Annotations: map[string]string{
				"metallb.universe.tf/ip-allocated-from-pool": "bgp-default",
			},
		},
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{Name: "http", Port: 80}},
		},
	}

	service := getChildService(t, reconcileWith(t, swh, existingService))

	if got := service.Annotations[lbIPsKey]; got != "185.11.73.234" {
		t.Errorf("annotation %s = %q, want %q", lbIPsKey, got, "185.11.73.234")
	}
	if got := service.Annotations["metallb.universe.tf/ip-allocated-from-pool"]; got != "bgp-default" {
		t.Errorf("foreign annotation was lost, got %q", got)
	}
}

func TestReconcileRemovesStalePropagatedKeys(t *testing.T) {
	swh := newTestSWH(map[string]string{lbIPsKey: "185.11.73.234"}, nil)
	existingService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
			Annotations: map[string]string{
				lbIPsKey:                 "185.11.73.234",
				sharedIPKey:              "code-e2e-stand",
				propagatedAnnotationsKey: lbIPsKey + "," + sharedIPKey,
				"metallb.universe.tf/ip-allocated-from-pool": "bgp-default",
			},
		},
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{Name: "http", Port: 80}},
		},
	}

	service := getChildService(t, reconcileWith(t, swh, existingService))

	if _, found := service.Annotations[sharedIPKey]; found {
		t.Errorf("annotation %s must be removed after it disappeared from the parent", sharedIPKey)
	}
	if got := service.Annotations[lbIPsKey]; got != "185.11.73.234" {
		t.Errorf("annotation %s = %q, want %q", lbIPsKey, got, "185.11.73.234")
	}
	if got := service.Annotations["metallb.universe.tf/ip-allocated-from-pool"]; got != "bgp-default" {
		t.Errorf("foreign annotation was lost, got %q", got)
	}
	if got := service.Annotations[propagatedAnnotationsKey]; got != lbIPsKey {
		t.Errorf("tracking annotation = %q, want %q", got, lbIPsKey)
	}
}

func TestIsMetadataForServiceEqual(t *testing.T) {
	tests := []struct {
		name    string
		service corev1.Service
		swh     *networkv1alpha1.ServiceWithHealthchecks
		want    bool
	}{
		{
			name:    "both empty",
			service: corev1.Service{},
			swh:     newTestSWH(nil, nil),
			want:    true,
		},
		{
			name: "in sync",
			service: corev1.Service{ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					lbIPsKey:                 "1.2.3.4",
					propagatedAnnotationsKey: lbIPsKey,
					"metallb.universe.tf/ip-allocated-from-pool": "bgp-default",
				},
			}},
			swh:  newTestSWH(map[string]string{lbIPsKey: "1.2.3.4"}, nil),
			want: true,
		},
		{
			name:    "annotation is missing on the child",
			service: corev1.Service{},
			swh:     newTestSWH(map[string]string{lbIPsKey: "1.2.3.4"}, nil),
			want:    false,
		},
		{
			name: "annotation value differs",
			service: corev1.Service{ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{lbIPsKey: "1.2.3.4", propagatedAnnotationsKey: lbIPsKey},
			}},
			swh:  newTestSWH(map[string]string{lbIPsKey: "4.3.2.1"}, nil),
			want: false,
		},
		{
			name: "annotation removed from the parent",
			service: corev1.Service{ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{lbIPsKey: "1.2.3.4", propagatedAnnotationsKey: lbIPsKey},
			}},
			swh:  newTestSWH(nil, nil),
			want: false,
		},
		{
			name: "label is missing on the child",
			service: corev1.Service{ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{propagatedLabelsKey: "heritage"},
			}},
			swh:  newTestSWH(nil, map[string]string{"heritage": "deckhouse"}),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsMetadataForServiceEqual(tt.service, tt.swh); got != tt.want {
				t.Errorf("IsMetadataForServiceEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}
