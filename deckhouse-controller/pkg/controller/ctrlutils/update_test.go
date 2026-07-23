// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ctrlutils

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

// TestUpdateStatusWithRetry_ConcurrentDelete reproduces the startup crash where a
// Module CR is still resolvable on the initial Get, but is deleted by another actor
// (another controller, a background loop, cache/apiserver skew) before Status().Update
// runs, so the API server answers NotFound. Such a concurrent deletion means there is
// nothing left to update and must NOT be treated as a fatal error.
func TestUpdateStatusWithRetry_ConcurrentDelete(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.SchemeBuilder.AddToScheme(scheme))

	module := &v1alpha1.Module{
		ObjectMeta: metav1.ObjectMeta{Name: "prometheus-metrics-adapter"},
		Properties: v1alpha1.ModuleProperties{Source: v1alpha1.ModuleSourceEmbedded},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(module).
		WithStatusSubresource(&v1alpha1.Module{}).
		WithInterceptorFuncs(interceptor.Funcs{
			// Simulate the object being deleted concurrently between the Get inside
			// UpdateWithRetry and this status update.
			SubResourceUpdate: func(_ context.Context, _ client.Client, _ string, obj client.Object, _ ...client.SubResourceUpdateOption) error {
				return apierrors.NewNotFound(v1alpha1.ModuleGVR.GroupResource(), obj.GetName())
			},
		}).
		Build()

	err := UpdateStatusWithRetry(context.Background(), cl, module, func() error {
		module.SetConditionUnknown(v1alpha1.ModuleConditionEnabledByModuleConfig, "", "")
		return nil
	})

	require.NoError(t, err, "a concurrent delete during status update must not be fatal")
}

// TestUpdateWithRetry_ConcurrentDelete is the same guarantee for the non-status Update path.
func TestUpdateWithRetry_ConcurrentDelete(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.SchemeBuilder.AddToScheme(scheme))

	module := &v1alpha1.Module{
		ObjectMeta: metav1.ObjectMeta{Name: "prometheus-metrics-adapter"},
		Properties: v1alpha1.ModuleProperties{Source: v1alpha1.ModuleSourceEmbedded},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(module).
		WithInterceptorFuncs(interceptor.Funcs{
			Update: func(_ context.Context, _ client.WithWatch, obj client.Object, _ ...client.UpdateOption) error {
				return apierrors.NewNotFound(v1alpha1.ModuleGVR.GroupResource(), obj.GetName())
			},
		}).
		Build()

	err := UpdateWithRetry(context.Background(), cl, module, func() error {
		module.Properties.Version = "v1.2.3"
		return nil
	})

	require.NoError(t, err, "a concurrent delete during update must not be fatal")
}
