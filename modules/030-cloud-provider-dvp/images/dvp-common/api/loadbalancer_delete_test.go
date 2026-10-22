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

package api

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/deckhouse/virtualization/api/core/v1alpha2"
)

const testLBName = "a0123456789"

type deleteRecorder struct {
	services []string
	swhcs    []string
}

func newLoadBalancerServiceForDelete(
	t *testing.T,
	rec *deleteRecorder,
	serviceDeleteErr error,
	objs ...ctrlclient.Object,
) *LoadBalancerService {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("corev1.AddToScheme: %v", err)
	}
	if err := v1alpha2.AddToScheme(scheme); err != nil {
		t.Fatalf("v1alpha2.AddToScheme: %v", err)
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithInterceptorFuncs(interceptor.Funcs{
			Delete: func(ctx context.Context, cl ctrlclient.WithWatch, obj ctrlclient.Object, opts ...ctrlclient.DeleteOption) error {
				switch obj.(type) {
				case *corev1.Service:
					rec.services = append(rec.services, obj.GetName())
					if serviceDeleteErr != nil {
						return serviceDeleteErr
					}
				default:
					rec.swhcs = append(rec.swhcs, obj.GetName())
				}
				return cl.Delete(ctx, obj, opts...)
			},
		}).
		Build()

	return NewLoadBalancerService(&Service{client: c, namespace: testNamespace})
}

func newTestLBService(name string, labels map[string]string, owners ...metav1.OwnerReference) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       testNamespace,
			Labels:          labels,
			OwnerReferences: owners,
		},
	}
}

func swhcOwnerReference(name string) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: "network.deckhouse.io/v1alpha1",
		Kind:       "ServiceWithHealthchecks",
		Name:       name,
		UID:        "swhc-uid",
	}
}

func TestDeleteLoadBalancerByNameDeletesOnlySWHCWhenServiceIsOwned(t *testing.T) {
	rec := &deleteRecorder{}
	svc := newTestLBService(testLBName, map[string]string{"heritage": "deckhouse"}, swhcOwnerReference(testLBName))
	// The Service delete is rejected the way the parent cluster rejects heritage=deckhouse objects:
	// the test fails if the code ever tries it.
	forbidden := k8serrors.NewForbidden(schema.GroupResource{Resource: "services"}, testLBName, nil)
	lb := newLoadBalancerServiceForDelete(t, rec, forbidden, svc)

	swhc := newServiceWithHealthchecksUnstructured(testNamespace, testLBName)
	swhc.SetUID("swhc-uid")
	if err := lb.client.Create(context.Background(), swhc); err != nil {
		t.Fatalf("create ServiceWithHealthchecks: %v", err)
	}

	if err := lb.DeleteLoadBalancerByName(context.Background(), testLBName); err != nil {
		t.Fatalf("DeleteLoadBalancerByName() error = %v", err)
	}

	if len(rec.services) != 0 {
		t.Fatalf("Service owned by ServiceWithHealthchecks must be left to the garbage collector, deleted: %v", rec.services)
	}
	if len(rec.swhcs) != 1 || rec.swhcs[0] != testLBName {
		t.Fatalf("ServiceWithHealthchecks must be deleted once, got %v", rec.swhcs)
	}
}

func TestDeleteLoadBalancerByNameIsIdempotentAfterSWHCIsGone(t *testing.T) {
	rec := &deleteRecorder{}
	// The garbage collector has not removed the child Service yet, and the owner is already deleted.
	svc := newTestLBService(testLBName, map[string]string{"heritage": "deckhouse"}, swhcOwnerReference(testLBName))
	lb := newLoadBalancerServiceForDelete(t, rec, nil, svc)

	for i := 0; i < 2; i++ {
		if err := lb.DeleteLoadBalancerByName(context.Background(), testLBName); err != nil {
			t.Fatalf("DeleteLoadBalancerByName() call %d error = %v", i+1, err)
		}
	}

	if len(rec.services) != 0 || len(rec.swhcs) != 0 {
		t.Fatalf("nothing must be deleted, got services=%v swhcs=%v", rec.services, rec.swhcs)
	}
}

func TestDeleteLoadBalancerByNameDeletesPlainService(t *testing.T) {
	rec := &deleteRecorder{}
	svc := newTestLBService(testLBName, map[string]string{"deckhouse.io/managed-by": "deckhouse"})
	lb := newLoadBalancerServiceForDelete(t, rec, nil, svc)

	if err := lb.DeleteLoadBalancerByName(context.Background(), testLBName); err != nil {
		t.Fatalf("DeleteLoadBalancerByName() error = %v", err)
	}

	if len(rec.services) != 1 || rec.services[0] != testLBName {
		t.Fatalf("plain Service must be deleted, got %v", rec.services)
	}
}

func TestDeleteLoadBalancerByNameToleratesForbiddenOnOwnerlessHeritageService(t *testing.T) {
	rec := &deleteRecorder{}
	svc := newTestLBService(testLBName, map[string]string{"heritage": "deckhouse"})
	forbidden := k8serrors.NewForbidden(schema.GroupResource{Resource: "services"}, testLBName, nil)
	lb := newLoadBalancerServiceForDelete(t, rec, forbidden, svc)

	if err := lb.DeleteLoadBalancerByName(context.Background(), testLBName); err != nil {
		t.Fatalf("DeleteLoadBalancerByName() must not fail on a protected ownerless Service, got %v", err)
	}
}

func TestDeleteLoadBalancerByNameReturnsForbiddenForUnprotectedService(t *testing.T) {
	rec := &deleteRecorder{}
	svc := newTestLBService(testLBName, map[string]string{"deckhouse.io/managed-by": "deckhouse"})
	forbidden := k8serrors.NewForbidden(schema.GroupResource{Resource: "services"}, testLBName, nil)
	lb := newLoadBalancerServiceForDelete(t, rec, forbidden, svc)

	if err := lb.DeleteLoadBalancerByName(context.Background(), testLBName); !k8serrors.IsForbidden(err) {
		t.Fatalf("DeleteLoadBalancerByName() error = %v, want Forbidden", err)
	}
}
