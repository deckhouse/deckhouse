/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package controller

import (
	"context"
	"slices"
	"testing"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
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
	testSWHUID    = types.UID("8f2b1c9e-7d3a-4f60-9a11-0c5e6d7b8a90")
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
			UID:         testSWHUID,
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
	c, _ := reconcileReturning(t, objects...)
	return c
}

func reconcileReturning(t *testing.T, objects ...client.Object) (client.Client, ctrl.Result) {
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

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: testName, Namespace: testNamespace},
	})
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
	return fakeClient, result
}

func getSWH(t *testing.T, c client.Client) *networkv1alpha1.ServiceWithHealthchecks {
	t.Helper()
	swh := &networkv1alpha1.ServiceWithHealthchecks{}
	if err := c.Get(context.Background(), types.NamespacedName{Name: testName, Namespace: testNamespace}, swh); err != nil {
		t.Fatalf("failed to get ServiceWithHealthchecks: %v", err)
	}
	return swh
}

// apiServerDefaults fills in what the API server writes into a Service on create. The fake client
// applies no defaulting, so a fixture standing for an existing Service has to spell it out, or the
// controller would rightfully see a spec that differs from the one it maintains.
func apiServerDefaults(service *corev1.Service) *corev1.Service {
	if service.Spec.Type == "" {
		service.Spec.Type = corev1.ServiceTypeClusterIP
	}
	if service.Spec.InternalTrafficPolicy == nil {
		policy := corev1.ServiceInternalTrafficPolicyCluster
		service.Spec.InternalTrafficPolicy = &policy
	}
	if service.Spec.ExternalTrafficPolicy == "" &&
		(service.Spec.Type == corev1.ServiceTypeLoadBalancer || service.Spec.Type == corev1.ServiceTypeNodePort) {
		service.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyCluster
	}
	for i := range service.Spec.Ports {
		if service.Spec.Ports[i].TargetPort == (intstr.IntOrString{}) {
			service.Spec.Ports[i].TargetPort = intstr.FromInt32(service.Spec.Ports[i].Port)
		}
	}
	return service
}

func ownedChildService(clusterIP string) *corev1.Service {
	isController := true
	return apiServerDefaults(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: networkv1alpha1.GroupVersion.String(),
				Kind:       "ServiceWithHealthchecks",
				Name:       testName,
				UID:        testSWHUID,
				Controller: &isController,
			}},
		},
		Spec: corev1.ServiceSpec{
			Type:      corev1.ServiceTypeLoadBalancer,
			Ports:     []corev1.ServicePort{{Name: "http", Port: 80}},
			ClusterIP: clusterIP,
		},
	})
}

func childServiceCondition(t *testing.T, c client.Client) metav1.Condition {
	t.Helper()
	swh := getSWH(t, c)
	for _, condition := range swh.Status.Conditions {
		if condition.Type == childServiceConditionType {
			return condition
		}
	}
	t.Fatalf("no ChildService condition found, got %+v", swh.Status.Conditions)
	return metav1.Condition{}
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
	existingService := apiServerDefaults(&corev1.Service{
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
	})

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
	existingService := apiServerDefaults(&corev1.Service{
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
	})

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

func TestReconcileKeepsRequeueingItself(t *testing.T) {
	scheme := newTestScheme(t)
	swh := newTestSWH(nil, nil)
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(swh).
		WithStatusSubresource(&networkv1alpha1.ServiceWithHealthchecks{}).
		Build()

	reconciler := &ServiceWithHealthchecksReconciler{
		Client: fakeClient,
		Scheme: scheme,
		Logger: log.NewNop(),
	}
	request := ctrl.Request{NamespacedName: types.NamespacedName{Name: testName, Namespace: testNamespace}}

	// The second pass changes nothing and must still requeue: clearNotUsedEPS only runs on
	// reconciliation, and nothing watches Nodes, so a controller that stops requeueing keeps
	// the EndpointSlices of removed nodes published.
	for _, pass := range []string{"first", "second"} {
		result, err := reconciler.Reconcile(context.Background(), request)
		if err != nil {
			t.Fatalf("%s reconcile failed: %v", pass, err)
		}
		if result.RequeueAfter != resyncPeriod {
			t.Errorf("%s reconcile: RequeueAfter = %v, want %v", pass, result.RequeueAfter, resyncPeriod)
		}
	}
}

// The child Service is the only thing tying the module's objects to the parent for the garbage
// collector, so the reference has to survive whatever state the Service is found in.
func TestReconcileSetsOwnerReferenceOnCreate(t *testing.T) {
	service := getChildService(t, reconcileWith(t, newTestSWH(nil, nil)))

	ref := metav1.GetControllerOf(service)
	if ref == nil {
		t.Fatalf("expected a controller reference, got %+v", service.OwnerReferences)
	}
	if ref.Kind != "ServiceWithHealthchecks" || ref.UID != testSWHUID {
		t.Errorf("expected the owner to be the parent ServiceWithHealthchecks, got %s %s", ref.Kind, ref.UID)
	}
	// OwnerReferencesPermissionEnforcement would demand access to the parent's finalizers
	// subresource otherwise, and nothing here relies on a foreground deletion.
	if ref.BlockOwnerDeletion != nil && *ref.BlockOwnerDeletion {
		t.Error("expected blockOwnerDeletion not to be set")
	}
}

// A Service created before the owner reference was introduced matches the desired spec, so it
// used to take the shortcut in Reconcile and stayed without an owner forever.
func TestReconcileAdoptsServiceWithoutOwnerReference(t *testing.T) {
	swh := newTestSWH(nil, nil)
	orphan := apiServerDefaults(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
		},
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{Name: "http", Port: 80}},
		},
	})

	service := getChildService(t, reconcileWith(t, swh, orphan))

	if !metav1.IsControlledBy(service, swh) {
		t.Errorf("expected the existing Service to be adopted, got %+v", service.OwnerReferences)
	}
}

// A parent recreated under the same name gets a new UID, and a reference to the old one would
// make the garbage collector drop the Service right after it was written.
func TestReconcileReplacesStaleOwnerReference(t *testing.T) {
	swh := newTestSWH(nil, nil)
	isController := true
	stale := apiServerDefaults(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: networkv1alpha1.GroupVersion.String(),
				Kind:       "ServiceWithHealthchecks",
				Name:       testName,
				UID:        types.UID("00000000-0000-0000-0000-000000000000"),
				Controller: &isController,
			}},
		},
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{Name: "http", Port: 80}},
		},
	})

	service := getChildService(t, reconcileWith(t, swh, stale))

	if !metav1.IsControlledBy(service, swh) {
		t.Errorf("expected the stale owner reference to be replaced, got %+v", service.OwnerReferences)
	}
}

// A selector on the child Service makes the EndpointSlice controller of kube-controller-manager
// publish a second set of slices for it, so a selector added by hand — or left on a Service the
// module adopted — has to be cleared even when the rest of the spec already matches.
func TestReconcileClearsSelectorOfAdoptedService(t *testing.T) {
	swh := newTestSWH(nil, nil)
	isController := true
	adopted := apiServerDefaults(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: networkv1alpha1.GroupVersion.String(),
				Kind:       "ServiceWithHealthchecks",
				Name:       testName,
				UID:        testSWHUID,
				Controller: &isController,
			}},
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeLoadBalancer,
			Ports:    []corev1.ServicePort{{Name: "http", Port: 80}},
			Selector: map[string]string{"app": "backend"},
		},
	})

	service := getChildService(t, reconcileWith(t, swh, adopted))

	if len(service.Spec.Selector) != 0 {
		t.Errorf("expected the selector to be cleared, got %v", service.Spec.Selector)
	}
}

// A Service owned by another controller is never touched and never claimed: rewriting the spec of
// somebody else's object would do more damage than the name clash itself.
func TestReconcileReportsConflictForForeignOwnedService(t *testing.T) {
	isController := true
	foreign := apiServerDefaults(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "backend",
				UID:        types.UID("2c4e5f60-1a2b-4c3d-8e9f-0a1b2c3d4e5f"),
				Controller: &isController,
			}},
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeLoadBalancer,
			Ports:    []corev1.ServicePort{{Name: "http", Port: 80}},
			Selector: map[string]string{"app": "backend"},
		},
	})

	fakeClient, result := reconcileReturning(t, newTestSWH(nil, nil), foreign)

	service := getChildService(t, fakeClient)
	if len(service.Spec.Selector) == 0 {
		t.Error("expected the Service of another controller to be left untouched")
	}
	if len(service.OwnerReferences) != 1 || service.OwnerReferences[0].Kind != "Deployment" {
		t.Errorf("expected no owner reference to be added, got %+v", service.OwnerReferences)
	}

	condition := childServiceCondition(t, fakeClient)
	if condition.Status != metav1.ConditionFalse || condition.Reason != "ChildServiceConflict" {
		t.Errorf("expected a ChildServiceConflict condition, got %s/%s", condition.Status, condition.Reason)
	}
	// Shorter than the plain resync: a Service the module does not own raises no event, so the
	// clash being resolved is only ever noticed on a timer.
	if result.RequeueAfter != childServiceConflictRetry {
		t.Errorf("expected the conflict to be re-checked in %v, got %v", childServiceConflictRetry, result.RequeueAfter)
	}
}

// A Service nobody owns is adopted only when it already matches the desired spec; one that
// differs belongs to whoever created it.
func TestReconcileReportsConflictForDivergingService(t *testing.T) {
	diverging := apiServerDefaults(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Ports:    []corev1.ServicePort{{Name: "grpc", Port: 9000}},
			Selector: map[string]string{"app": "backend"},
		},
	})

	fakeClient, result := reconcileReturning(t, newTestSWH(nil, nil), diverging)

	service := getChildService(t, fakeClient)
	if service.Spec.Type != corev1.ServiceTypeClusterIP || len(service.Spec.Selector) == 0 ||
		!slices.Equal(service.Spec.Ports, diverging.Spec.Ports) {
		t.Errorf("expected the diverging Service to be left untouched, got %+v", service.Spec)
	}
	if len(service.OwnerReferences) != 0 {
		t.Errorf("expected no owner reference to be added, got %+v", service.OwnerReferences)
	}

	condition := childServiceCondition(t, fakeClient)
	if condition.Status != metav1.ConditionFalse || condition.Reason != "ChildServiceConflict" {
		t.Errorf("expected a ChildServiceConflict condition, got %s/%s", condition.Status, condition.Reason)
	}
	// Shorter than the plain resync: a Service the module does not own raises no event, so the
	// clash being resolved is only ever noticed on a timer.
	if result.RequeueAfter != childServiceConflictRetry {
		t.Errorf("expected the conflict to be re-checked in %v, got %v", childServiceConflictRetry, result.RequeueAfter)
	}
}

// The clash is resolvable: once the Service matches again, the module takes it over and the
// condition recovers.
func TestReconcileRecoversAfterConflictIsResolved(t *testing.T) {
	fakeClient := reconcileWith(t, newTestSWH(nil, nil), apiServerDefaults(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeLoadBalancer,
			Ports:    []corev1.ServicePort{{Name: "http", Port: 80}},
			Selector: map[string]string{"app": "backend"},
		},
	}))
	if condition := childServiceCondition(t, fakeClient); condition.Reason != "ChildServiceConflict" {
		t.Fatalf("expected the conflict to be reported first, got %s", condition.Reason)
	}

	service := getChildService(t, fakeClient)
	service.Spec.Selector = nil
	if err := fakeClient.Update(context.Background(), service); err != nil {
		t.Fatalf("failed to drop the selector: %v", err)
	}

	reconciler := &ServiceWithHealthchecksReconciler{Client: fakeClient, Scheme: newTestScheme(t), Logger: log.NewNop()}
	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: testName, Namespace: testNamespace},
	}); err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	if condition := childServiceCondition(t, fakeClient); condition.Status != metav1.ConditionTrue {
		t.Errorf("expected the condition to recover, got %s/%s", condition.Status, condition.Reason)
	}
	if !metav1.IsControlledBy(getChildService(t, fakeClient), newTestSWH(nil, nil)) {
		t.Error("expected the Service to be adopted once it matched again")
	}
}

// The condition was renamed, so a copy under the old name is dropped instead of staying in the
// status of a resource reconciled by an earlier version.
func TestReconcileDropsLegacyChildServiceCondition(t *testing.T) {
	swh := newTestSWH(nil, nil)
	swh.Status.Conditions = []metav1.Condition{{
		Type:               legacyChildServiceConditionType,
		Status:             metav1.ConditionTrue,
		Reason:             "ChildServiceWasCreated",
		Message:            "Service was created successfully",
		LastTransitionTime: metav1.Now(),
	}}

	fakeClient := reconcileWith(t, swh)

	updated := &networkv1alpha1.ServiceWithHealthchecks{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: testName, Namespace: testNamespace}, updated); err != nil {
		t.Fatalf("failed to get ServiceWithHealthchecks: %v", err)
	}
	for _, condition := range updated.Status.Conditions {
		if condition.Type == legacyChildServiceConditionType {
			t.Errorf("expected the legacy condition to be dropped, got %+v", updated.Status.Conditions)
		}
	}
	if condition := childServiceCondition(t, fakeClient); condition.Status != metav1.ConditionTrue {
		t.Errorf("expected the renamed condition to be reported, got %s/%s", condition.Status, condition.Reason)
	}
}

// An address asked for in the spec is only applicable while the Service is being created.
func TestReconcileRequestsClusterIPFromSpecOnCreate(t *testing.T) {
	swh := newTestSWH(nil, nil)
	swh.Spec.ClusterIP = "10.96.0.42"

	fakeClient := reconcileWith(t, swh)

	service := getChildService(t, fakeClient)
	if service.Spec.ClusterIP != "10.96.0.42" {
		t.Errorf("expected the requested address to reach the child Service, got %q", service.Spec.ClusterIP)
	}
	if got := getSWH(t, fakeClient).Status.ClusterIP; got != "10.96.0.42" {
		t.Errorf("status.clusterIP = %q, want %q", got, "10.96.0.42")
	}
	if condition := childServiceCondition(t, fakeClient); condition.Status != metav1.ConditionTrue {
		t.Errorf("expected a healthy condition, got %s/%s", condition.Status, condition.Reason)
	}
}

// An address allocated by the cluster is reported in the status, and never written into the spec
// of the ServiceWithHealthchecks.
func TestReconcileReportsAllocatedClusterIP(t *testing.T) {
	swh := newTestSWH(nil, nil)

	fakeClient := reconcileWith(t, swh, ownedChildService("10.96.0.7"))

	if got := getChildService(t, fakeClient).Spec.ClusterIP; got != "10.96.0.7" {
		t.Errorf("expected the allocated address to be kept, got %q", got)
	}
	updated := getSWH(t, fakeClient)
	if updated.Status.ClusterIP != "10.96.0.7" {
		t.Errorf("status.clusterIP = %q, want %q", updated.Status.ClusterIP, "10.96.0.7")
	}
	if updated.Spec.ClusterIP != "" {
		t.Errorf("expected the spec to be left alone, got %q", updated.Spec.ClusterIP)
	}
}

// The field is immutable, so a request that no longer matches the Service is reported instead of
// being retried forever.
func TestReconcileReportsClusterIPMismatch(t *testing.T) {
	swh := newTestSWH(nil, nil)
	swh.Spec.ClusterIP = "10.96.0.42"

	fakeClient := reconcileWith(t, swh, ownedChildService("10.96.0.7"))

	if got := getChildService(t, fakeClient).Spec.ClusterIP; got != "10.96.0.7" {
		t.Errorf("expected the address of the Service to be left alone, got %q", got)
	}
	condition := childServiceCondition(t, fakeClient)
	if condition.Status != metav1.ConditionFalse || condition.Reason != "ChildServiceClusterIPImmutable" {
		t.Errorf("expected a ChildServiceClusterIPImmutable condition, got %s/%s", condition.Status, condition.Reason)
	}
	if got := getSWH(t, fakeClient).Status.ClusterIP; got != "10.96.0.7" {
		t.Errorf("expected the status to report the actual address, got %q", got)
	}
}

// A Service that belongs to somebody else is the more pressing problem of the two.
func TestReconcileReportsConflictBeforeClusterIPMismatch(t *testing.T) {
	swh := newTestSWH(nil, nil)
	swh.Spec.ClusterIP = "10.96.0.42"
	foreign := ownedChildService("10.96.0.7")
	foreign.OwnerReferences[0].APIVersion = "apps/v1"
	foreign.OwnerReferences[0].Kind = "Deployment"
	foreign.OwnerReferences[0].Name = "backend"

	fakeClient := reconcileWith(t, swh, foreign)

	if condition := childServiceCondition(t, fakeClient); condition.Reason != "ChildServiceConflict" {
		t.Errorf("expected the conflict to be reported, got %s", condition.Reason)
	}
}

// The API server fills in the fields the parent leaves out. Comparing the literal values would
// report a difference that no update can ever settle.
func TestIsSpecForServiceEqualAcceptsAPIServerDefaults(t *testing.T) {
	swh := newTestSWH(nil, nil)
	swh.Spec.Type = ""
	swh.Spec.InternalTrafficPolicy = nil
	swh.Spec.ExternalTrafficPolicy = ""
	swh.Spec.Ports = []corev1.ServicePort{{Name: "http", Port: 80}}

	service := apiServerDefaults(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: testName, Namespace: testNamespace},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{Name: "http", Port: 80}},
		},
	})

	if !IsSpecForServiceEqual(*service, swh) {
		t.Errorf("expected the defaulted Service to match the spec of the parent, got %+v", service.Spec)
	}
}

// Every reconciliation used to rewrite the Service, and every write came back as an event that
// started the next one.
func TestReconcileLeavesSettledServiceUntouched(t *testing.T) {
	settled := ownedChildService("10.96.0.7")

	fakeClient := reconcileWith(t, newTestSWH(nil, nil), settled)

	service := getChildService(t, fakeClient)
	if service.ResourceVersion != settled.ResourceVersion {
		t.Errorf("the Service was rewritten although it already matched: %q -> %q, spec %+v",
			settled.ResourceVersion, service.ResourceVersion, service.Spec)
	}
}

// A nodePort is allocated by the API server. Sending a zero over it makes the API server hand out
// a new one, so the port of a load balancer would move on every reconciliation.
func TestReconcileKeepsAllocatedNodePort(t *testing.T) {
	swh := newTestSWH(nil, nil)
	swh.Spec.Type = corev1.ServiceTypeLoadBalancer

	allocated := ownedChildService("10.96.0.7")
	allocated.Spec.Ports[0].NodePort = 31234
	// something the controller has to fix, so that the update path is taken at all
	allocated.Spec.Selector = map[string]string{"app": "backend"}

	fakeClient := reconcileWith(t, swh, allocated)

	service := getChildService(t, fakeClient)
	if len(service.Spec.Selector) != 0 {
		t.Fatalf("expected the selector to be cleared, got %v", service.Spec.Selector)
	}
	if service.Spec.Ports[0].NodePort != 31234 {
		t.Errorf("expected the allocated nodePort to be kept, got %d", service.Spec.Ports[0].NodePort)
	}
}

// ServicePort.AppProtocol is a pointer, so a struct comparison looks at its address: two ports
// carrying the same appProtocol come from different decodes and would never compare equal.
func TestReconcileLeavesSettledServiceWithAppProtocolUntouched(t *testing.T) {
	fromParent, fromService := "http", "http"

	swh := newTestSWH(nil, nil)
	swh.Spec.Ports = []corev1.ServicePort{{Name: "http", Port: 80, AppProtocol: &fromParent}}

	settled := ownedChildService("10.96.0.7")
	settled.Spec.Ports[0].AppProtocol = &fromService

	fakeClient := reconcileWith(t, swh, settled)

	service := getChildService(t, fakeClient)
	if service.ResourceVersion != settled.ResourceVersion {
		t.Errorf("the Service was rewritten although only the appProtocol pointer differed: %q -> %q",
			settled.ResourceVersion, service.ResourceVersion)
	}
	if !IsSpecForServiceEqual(*settled, swh) {
		t.Error("expected the appProtocol to be compared by value")
	}
}

// Only a controller reference means the object belongs to somebody else. A plain owner reference
// is an extra garbage collection link that anything may add, and it must not cost the module
// control over its own Service.
func TestReconcileAdoptsServiceWithPlainOwnerReference(t *testing.T) {
	swh := newTestSWH(nil, nil)
	adopted := ownedChildService("10.96.0.7")
	adopted.OwnerReferences = []metav1.OwnerReference{{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		Name:       "some-bookkeeping",
		UID:        types.UID("4a5b6c7d-8e9f-4a0b-9c1d-2e3f4a5b6c7d"),
	}}

	fakeClient := reconcileWith(t, swh, adopted)

	service := getChildService(t, fakeClient)
	if !metav1.IsControlledBy(service, swh) {
		t.Errorf("expected the Service to be adopted, got %+v", service.OwnerReferences)
	}
	if condition := childServiceCondition(t, fakeClient); condition.Status != metav1.ConditionTrue {
		t.Errorf("expected no conflict, got %s/%s: %s", condition.Status, condition.Reason, condition.Message)
	}
}
