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

package hooks

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

// removedQuotaCRDName is the ClusterResourceGrant CRD removed in the references redesign (object
// quota was dropped from this module). Deckhouse does not garbage-collect CRDs removed from a module,
// so a one-shot delete on startup removes the orphan and (cascading) any leftover ClusterResourceGrant
// objects from clusters that ran the quota-bearing version.
const removedQuotaCRDName = "clusterresourcegrants.multitenancy.deckhouse.io"

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 10},
	Queue:     "/modules/160-multitenancy-manager",
}, dependency.WithExternalDependencies(cleanupRemovedQuotaCRD))

func cleanupRemovedQuotaCRD(_ context.Context, _ *go_hook.HookInput, dc dependency.Container) error {
	kube, err := dc.GetK8sClient()
	if err != nil {
		return err
	}
	crdGVR := schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}
	err = kube.Dynamic().Resource(crdGVR).Delete(context.TODO(), removedQuotaCRDName, v1.DeleteOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("delete orphaned %s CRD: %w", removedQuotaCRDName, err)
	}
	return nil
}
