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

package helm

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ManagedByHelm is the value Helm writes on objects it created (or was told it created).
const ManagedByHelm = "Helm"

// ReleaseStorageNamespace is where project Helm release Secrets live. The
// client does not set install.Namespace, so Helm usually stamps
// release-namespace as empty; this constant is still treated as ours in case
// a later apply fills it from the action config.
const ReleaseStorageNamespace = "d8-multitenancy-manager"

// ErrForeignRelease is returned when a namespace already names a Helm release
// that is not this project's. Adoption must not overwrite that metadata.
var ErrForeignRelease = errors.New("namespace is owned by a different Helm release")

// ForeignRelease is the Helm release already on ns when it is not this
// project's. Helm identity is the (release-name, release-namespace) pair.
// Empty means the namespace is free to stamp (no release, or ours).
//
// Ours: release-name is ReleaseName(project) (or the raw 61-char name we used
// to stamp) and release-namespace is empty or the module storage namespace.
// helm install foo -n foo is foreign even when the names match: its
// release-namespace is the user namespace, not ours.
func ForeignRelease(ns *corev1.Namespace, releaseName string) string {
	if ns == nil {
		return ""
	}
	name := ns.Annotations[ResourceAnnotationReleaseName]
	if name == "" {
		return ""
	}
	relNS := ns.Annotations[ResourceAnnotationReleaseNamespace]
	if oursReleaseName(ns, name, releaseName) && oursReleaseNamespace(relNS) {
		return ""
	}
	if relNS != "" {
		return name + "/" + relNS
	}
	return name
}

func oursReleaseName(ns *corev1.Namespace, current, releaseName string) bool {
	if current == releaseName {
		return true
	}
	// A 61-character project used to be stamped with the raw name. Helm itself
	// cannot store a 61-character release, so this cannot be a user chart.
	return current == ns.Name && releaseName == ReleaseName(ns.Name) && current != releaseName
}

func oursReleaseNamespace(relNS string) bool {
	return relNS == "" || relNS == ReleaseStorageNamespace
}

// ApplyReleaseOwnership writes Helm ownership metadata onto ns in memory. It
// returns true when the object changed and must be persisted. A terminating
// namespace is left alone: there is nothing to stamp. A namespace that already
// names a different release is left alone too.
func ApplyReleaseOwnership(ns *corev1.Namespace, releaseName string) bool {
	if ns == nil || !ns.DeletionTimestamp.IsZero() {
		return false
	}
	if ForeignRelease(ns, releaseName) != "" {
		return false
	}
	if ns.Labels == nil {
		ns.Labels = make(map[string]string, 1)
	}
	if ns.Annotations == nil {
		ns.Annotations = make(map[string]string, 2)
	}
	if ns.Labels[ResourceLabelManagedBy] == ManagedByHelm &&
		ns.Annotations[ResourceAnnotationReleaseName] == releaseName &&
		ns.Annotations[ResourceAnnotationReleaseNamespace] == "" {
		return false
	}
	ns.Labels[ResourceLabelManagedBy] = ManagedByHelm
	ns.Annotations[ResourceAnnotationReleaseName] = releaseName
	ns.Annotations[ResourceAnnotationReleaseNamespace] = ""
	return true
}

// StampReleaseOwnership writes the labels and annotations Helm requires to take over a
// Namespace it did not create. Without them the project release fails with an ownership
// conflict. projectName is the namespace name (and the Project name); the Helm
// release-name annotation is ReleaseName(projectName), which may be shorter than 53
// characters. A missing or terminating namespace is a no-op: there is nothing to stamp.
func StampReleaseOwnership(ctx context.Context, c client.Client, projectName string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		namespace := new(corev1.Namespace)
		if err := c.Get(ctx, client.ObjectKey{Name: projectName}, namespace); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("get the '%s' namespace: %w", projectName, err)
		}
		ours := ReleaseName(projectName)
		if foreign := ForeignRelease(namespace, ours); foreign != "" {
			return fmt.Errorf("%w %q", ErrForeignRelease, foreign)
		}
		if !ApplyReleaseOwnership(namespace, ours) {
			return nil
		}
		if err := c.Update(ctx, namespace); err != nil {
			return fmt.Errorf("stamp helm ownership on the '%s' namespace: %w", projectName, err)
		}
		return nil
	})
}
