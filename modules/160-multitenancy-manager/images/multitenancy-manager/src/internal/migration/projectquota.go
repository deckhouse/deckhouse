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

// Package migration removes state left by earlier versions of multitenancy-manager.
package migration

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"controller/apis/deckhouse.io/v1alpha1"
	"controller/apis/deckhouse.io/v1alpha3"
)

const retryInterval = 30 * time.Second

var legacyQuotaGVK = schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "ProjectQuota"}

// ProjectQuota removes the automatic ProjectQuota objects after the controller has restored their
// replacement from Project.spec.quota. Both reads and writes bypass the cache, including the optional
// old kind, so its absence does not start an informer or prevent the manager's cache from syncing.
type ProjectQuota struct {
	Client client.Client
	Logger logr.Logger
}

// Register adds the migration to the manager, alongside the controllers that restore the new status.
func Register(mgr manager.Manager) error {
	c, err := client.New(mgr.GetConfig(), client.Options{
		Scheme: mgr.GetScheme(), HTTPClient: mgr.GetHTTPClient(), Mapper: mgr.GetRESTMapper(),
	})
	if err != nil {
		return fmt.Errorf("create the migration client: %w", err)
	}
	return mgr.Add(&ProjectQuota{Client: c, Logger: mgr.GetLogger().WithName("projectquota-migration")})
}

// NeedLeaderElection restricts the migration to the controller's leader.
func (*ProjectQuota) NeedLeaderElection() bool { return true }

// Start retries incomplete passes until the old objects are gone or the manager stops.
func (m *ProjectQuota) Start(ctx context.Context) error {
	for {
		if err := m.Run(ctx); err == nil {
			m.Logger.Info("automatic ProjectQuota migration completed")
			return nil
		} else if ctx.Err() == nil {
			m.Logger.Error(err, "retry the ProjectQuota migration")
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(retryInterval):
		}
	}
}

// Run migrates the automatic legacy objects. The old CRD is optional and remains installed.
func (m *ProjectQuota) Run(ctx context.Context) error {
	list := new(unstructured.UnstructuredList)
	list.SetGroupVersionKind(legacyQuotaGVK.GroupVersion().WithKind("ProjectQuotaList"))
	if err := m.Client.List(ctx, list, client.MatchingLabels{
		v1alpha3.ResourceLabelManagedBy: v1alpha3.ManagedByController,
	}); err != nil {
		if meta.IsNoMatchError(err) || apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("list legacy ProjectQuotas: %w", err)
	}
	if len(list.Items) == 0 {
		return nil
	}
	quotas := new(corev1.ResourceQuotaList)
	if err := m.Client.List(ctx, quotas); err != nil {
		return fmt.Errorf("list ResourceQuotas: %w", err)
	}
	var errs []error
	for i := range list.Items {
		if err := m.remove(ctx, &list.Items[i], quotas.Items); err != nil {
			errs = append(errs, fmt.Errorf("migrate ProjectQuota %s: %w", list.Items[i].GetName(), err))
		}
	}
	return errors.Join(errs...)
}

func (m *ProjectQuota) remove(ctx context.Context, old *unstructured.Unstructured, quotas []corev1.ResourceQuota) error {
	// Automatic objects carry the managed-by label written by reconcileQuota. A different object
	// under the old API needs an explicit decision about its limits.
	if old.GetLabels()[v1alpha3.ResourceLabelManagedBy] != v1alpha3.ManagedByController {
		return fmt.Errorf("legacy quota is not controller-managed")
	}
	project := new(v1alpha3.Project)
	err := m.Client.Get(ctx, client.ObjectKey{Name: old.GetName()}, project)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("get the project: %w", err)
	}
	if err == nil && project.DeletionTimestamp.IsZero() && len(project.Spec.Quota) > 0 {
		status := new(v1alpha1.ProjectQuotaStatus)
		if err := m.Client.Get(ctx, client.ObjectKey{Name: project.Name}, status); err != nil {
			return fmt.Errorf("wait for the replacement quota status: %w", err)
		}
		if !status.DeletionTimestamp.IsZero() ||
			!equality.Semantic.DeepEqual(status.Spec.Hard, project.Spec.Quota) ||
			!metav1.IsControlledBy(status, project) {
			return fmt.Errorf("wait for the replacement quota status to match the project")
		}
	}

	// Detach the old owner before completing its deletion. A quota already being deleted by GC
	// can disappear during the patch, and the normal reconciler restores it from the new status.
	for i := range quotas {
		rq := &quotas[i]
		patch := client.MergeFromWithOptions(rq.DeepCopy(), client.MergeFromWithOptimisticLock{})
		refs := slices.DeleteFunc(rq.OwnerReferences, func(ref metav1.OwnerReference) bool {
			return ref.UID == old.GetUID() && ref.Kind == legacyQuotaGVK.Kind &&
				ref.APIVersion == legacyQuotaGVK.GroupVersion().String()
		})
		if len(refs) == len(rq.OwnerReferences) {
			continue
		}
		rq.OwnerReferences = refs
		if err := m.Client.Patch(ctx, rq, patch); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("detach ResourceQuota %s/%s: %w", rq.Namespace, rq.Name, err)
		}
	}

	// Mark deletion before dropping the finalizer. Both steps are retried after a restart, and the
	// UID precondition prevents deleting a different object that reused the name.
	uid := old.GetUID()
	if old.GetDeletionTimestamp().IsZero() {
		if err := m.Client.Delete(ctx, old, client.Preconditions{UID: &uid},
			client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete the legacy quota: %w", err)
		}
	}
	current := new(unstructured.Unstructured)
	current.SetGroupVersionKind(legacyQuotaGVK)
	if err := m.Client.Get(ctx, client.ObjectKeyFromObject(old), current); err != nil {
		return client.IgnoreNotFound(err)
	}
	if current.GetUID() != old.GetUID() {
		return fmt.Errorf("legacy quota was replaced during migration")
	}
	if controllerutil.ContainsFinalizer(current, v1alpha1.ProjectQuotaFinalizer) {
		patch := client.MergeFromWithOptions(current.DeepCopy(), client.MergeFromWithOptimisticLock{})
		controllerutil.RemoveFinalizer(current, v1alpha1.ProjectQuotaFinalizer)
		if err := m.Client.Patch(ctx, current, patch); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("remove the legacy quota finalizer: %w", err)
		}
	}
	// A GC finalizer or another writer's finalizer can still hold the object. Check again until
	// it is gone, so a successful pass means every automatic old quota has actually been removed.
	if err := m.Client.Get(ctx, client.ObjectKeyFromObject(old), current); err != nil {
		return client.IgnoreNotFound(err)
	}
	return fmt.Errorf("wait for legacy quota deletion")
}
