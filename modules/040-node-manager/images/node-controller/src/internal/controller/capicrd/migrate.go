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

package capicrd

import (
	"context"
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// migrateStorage rewrites every CAPI object at the target storage version and
// trims status.storedVersions down to the target version. It is idempotent: a
// CRD that already reports only the target version is skipped.
func (m *Manager) migrateStorage(ctx context.Context) error {
	for i := range m.crds {
		if err := m.migrateCRD(ctx, &m.crds[i]); err != nil {
			return fmt.Errorf("migrate %s: %w", m.crds[i].Name, err)
		}
	}
	return nil
}

func (m *Manager) migrateCRD(ctx context.Context, crd *apiextensionsv1.CustomResourceDefinition) error {
	var live apiextensionsv1.CustomResourceDefinition
	if err := m.client.Get(ctx, client.ObjectKey{Name: crd.Name}, &live); err != nil {
		return fmt.Errorf("get crd: %w", err)
	}

	if len(live.Status.StoredVersions) == 1 && live.Status.StoredVersions[0] == targetStorageVersion {
		return nil
	}

	gvr := schema.GroupVersionResource{
		Group:    crd.Spec.Group,
		Version:  targetStorageVersion,
		Resource: crd.Spec.Names.Plural,
	}
	if err := m.rewriteObjects(ctx, gvr); err != nil {
		return fmt.Errorf("rewrite objects: %w", err)
	}

	live.Status.StoredVersions = []string{targetStorageVersion}
	if err := m.client.Status().Update(ctx, &live); err != nil {
		return fmt.Errorf("update storedVersions: %w", err)
	}

	log.Info("storage migrated", "crd", crd.Name, "version", targetStorageVersion)
	return nil
}

// rewriteObjects reads every object of gvr and writes it back unchanged, which
// forces the apiserver to persist it at the current storage version.
func (m *Manager) rewriteObjects(ctx context.Context, gvr schema.GroupVersionResource) error {
	list, err := m.dynamic.Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list %s: %w", gvr.Resource, err)
	}

	for i := range list.Items {
		obj := &list.Items[i]
		ri := m.dynamic.Resource(gvr).Namespace(obj.GetNamespace())

		if _, err := ri.Update(ctx, obj, metav1.UpdateOptions{}); err != nil {
			// A conflict means another writer already rewrote the object at the
			// new storage version, so the migration goal is met regardless.
			if apierrors.IsConflict(err) {
				continue
			}
			return fmt.Errorf("rewrite %s/%s: %w", obj.GetNamespace(), obj.GetName(), err)
		}
	}

	return nil
}
