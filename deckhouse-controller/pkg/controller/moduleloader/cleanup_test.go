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

package moduleloader

import (
	"context"
	"testing"

	addonmodules "github.com/flant/addon-operator/pkg/module_manager/models/modules"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// TestCleanupDeletedModules_ConcurrentDelete reproduces the startup crash: a Module CR
// with no matching ModuleConfig is deleted by another actor while cleanupDeletedModules
// is updating its EnabledByModuleConfig condition. The status update then hits the API
// server with NotFound. This must not abort controller startup.
func TestCleanupDeletedModules_ConcurrentDelete(t *testing.T) {
	// source != Embedded so the module skips the delete branch and reaches the
	// status-update branch — the exact path that crashed on prometheus-metrics-adapter.
	module := testModule("prometheus-metrics-adapter", "deckhouse")

	cl := fake.NewClientBuilder().
		WithScheme(newTestScheme(t)).
		WithObjects(module).
		WithStatusSubresource(&v1alpha1.Module{}).
		WithInterceptorFuncs(interceptor.Funcs{
			// Another actor deletes the module between the Get and the status update.
			SubResourceUpdate: func(_ context.Context, _ client.Client, _ string, obj client.Object, _ ...client.SubResourceUpdateOption) error {
				return apierrors.NewNotFound(v1alpha1.ModuleGVR.GroupResource(), obj.GetName())
			},
		}).
		Build()

	l := &Loader{
		client:              cl,
		logger:              log.NewNop(),
		registries:          make(map[string]*addonmodules.Registry),
		dependencyContainer: dependency.NewDependencyContainer(),
	}

	require.NoError(t, l.cleanupDeletedModules(context.Background()),
		"a Module deleted concurrently during status update must not fail cleanup")
}
