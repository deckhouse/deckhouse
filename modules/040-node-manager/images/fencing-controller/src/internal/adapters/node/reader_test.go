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

package node

import (
	"context"
	"errors"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

const (
	nodeName = "worker-3"
	nodeUID  = "1a2b3c4d-1111-2222-3333-444455556666"
)

func TestGetNodeReturnsTheNode(t *testing.T) {
	c := fake.NewClientBuilder().
		WithScheme(newScheme(t)).
		WithObjects(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: nodeName, UID: nodeUID}}).
		Build()

	got, err := NewReader(c).GetNode(context.Background(), nodeName)
	if err != nil {
		t.Fatalf("get node: %v", err)
	}

	// The UID is the whole point of the read: it is what tells a recreated Node
	// from the one an incident was created for.
	if got.UID != nodeUID {
		t.Errorf("read UID %q, want %q", got.UID, nodeUID)
	}
}

// TestGetNodeKeepsNotFoundRecognizable is the invariant the caller depends on: a
// deleted Node is a terminal answer, and the validator tells it apart from a
// transient failure with apierrors.IsNotFound. Wrapping the error with %v
// instead of %w would turn that terminal answer into an endless retry.
func TestGetNodeKeepsNotFoundRecognizable(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(newScheme(t)).Build()

	_, err := NewReader(c).GetNode(context.Background(), nodeName)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("get of a missing node returned %v, want an error IsNotFound recognizes", err)
	}

	if !strings.Contains(err.Error(), nodeName) {
		t.Errorf("error %q does not name the node that was read", err)
	}
}

func TestGetNodeWrapsOtherFailures(t *testing.T) {
	apiDown := apierrors.NewServiceUnavailable("etcd leader changed")

	c := fake.NewClientBuilder().
		WithScheme(newScheme(t)).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
				return apiDown
			},
		}).
		Build()

	_, err := NewReader(c).GetNode(context.Background(), nodeName)
	if !errors.Is(err, apiDown) {
		t.Fatalf("get returned %v, want the API error so the caller retries with backoff", err)
	}
}

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	s := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(s); err != nil {
		t.Fatalf("register client-go scheme: %v", err)
	}

	return s
}
