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

// Package handover moves objects from the module chart to user-authn-controller.
//
// When the chart stops rendering an object, the release that ships the change prunes it, and
// the controller, deployed by that same release, cannot be running yet to keep it. For a Dex
// Password that is not a gap but a loss: the controller creates it anew from User.spec.password,
// so the password the user changed in Dex, the password history and the lockout state are gone.
// Both release engines honour the `helm.sh/resource-policy: keep` annotation on the live object,
// so Protect stamps it on every object of a Kind the release still owns, right before the release
// runs, and fails the run if any is left unprotected. The controller keeps the annotation on
// every Password it reconciles.
//
// Moving another kind to the controller means adding a Kind to the list the module hooks pass
// here; neither the hooks nor this package change.
//
// Rollback: deckhouse runs nelm with forced adoption, so an older chart that renders the objects
// again takes them back over. One path keeps a leftover: if the release fails after Protect and
// deckhouse goes back to the previous version, the old chart renders the Passwords with the keep
// policy Protect set. A User deleted on that version then leaves its Password behind, and the
// user can still log in until the next update, whose controller deletes the Passwords of deleted
// Users. To remove one at once, delete the Password of the user in d8-user-authn as well.
package handover

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

const (
	// ReleaseName is the Helm release of the module.
	ReleaseName = "user-authn"

	ReleaseNameAnnotation    = "meta.helm.sh/release-name"
	ResourcePolicyAnnotation = "helm.sh/resource-policy"
	ResourcePolicyKeep       = "keep"

	listPageSize = 500
)

// Ref is one object.
type Ref struct {
	Namespace string
	Name      string
}

// Kind is one kind of objects the chart used to render and the controller owns now.
type Kind struct {
	// Name is how the kind appears in errors.
	Name     string
	Resource schema.GroupVersionResource

	// Rendered returns the objects the chart may have rendered. Only those the release really
	// owns are touched, so returning more is harmless; returning fewer loses objects. Derive them
	// from the source objects rather than from labels: labels of a rendered object may come from
	// user input and not be what the chart set by itself.
	Rendered func(ctx context.Context, c dynamic.Interface) ([]Ref, error)
}

// Result counts the objects of the release Protect found: Owned is every one the release still
// owns, protected or not, and Stamped those it protected now.
type Result struct {
	Owned   int
	Stamped int
}

// Protect stamps the keep policy on every object of kinds still owned by the release. It fails if
// an object could not be protected: the release would prune it.
func Protect(ctx context.Context, c dynamic.Interface, kinds []Kind) (Result, error) {
	patch, err := json.Marshal(map[string]any{
		"metadata": map[string]any{"annotations": map[string]any{ResourcePolicyAnnotation: ResourcePolicyKeep}},
	})
	if err != nil {
		return Result{}, fmt.Errorf("marshal patch: %w", err)
	}

	var res Result
	var errs []error
	for _, kind := range kinds {
		refs, err := kind.Rendered(ctx, c)
		if err != nil {
			errs = append(errs, fmt.Errorf("list rendered %s: %w", kind.Name, err))
			continue
		}
		for _, ref := range refs {
			client := resource(c, kind.Resource, ref.Namespace)
			obj, err := client.Get(ctx, ref.Name, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				continue
			}
			if err != nil {
				errs = append(errs, fmt.Errorf("get %s %s: %w", kind.Name, ref, err))
				continue
			}
			if obj.GetAnnotations()[ReleaseNameAnnotation] != ReleaseName {
				continue
			}
			res.Owned++
			if !needsKeep(obj) {
				continue
			}

			patched, err := client.Patch(ctx, ref.Name, types.MergePatchType, patch, metav1.PatchOptions{})
			if apierrors.IsNotFound(err) {
				continue
			}
			if err != nil {
				errs = append(errs, fmt.Errorf("patch %s %s: %w", kind.Name, ref, err))
				continue
			}
			if needsKeep(patched) {
				errs = append(errs, fmt.Errorf("keep policy is not set on %s %s: refusing to proceed to avoid prune", kind.Name, ref))
				continue
			}
			res.Stamped++
		}
	}
	return res, errors.Join(errs...)
}

// needsKeep reports whether obj belongs to the release and is not protected yet.
func needsKeep(obj *unstructured.Unstructured) bool {
	annotations := obj.GetAnnotations()
	return annotations[ReleaseNameAnnotation] == ReleaseName && annotations[ResourcePolicyAnnotation] != ResourcePolicyKeep
}

// List pages through the objects of gvr in namespace (all namespaces when empty) matching
// selector and returns the refs of those passing filter (all when nil). A resource whose CRD is
// not installed has no objects.
func List(ctx context.Context, c dynamic.Interface, gvr schema.GroupVersionResource, namespace, selector string, filter func(*unstructured.Unstructured) bool) ([]Ref, error) {
	var refs []Ref
	opts := metav1.ListOptions{LabelSelector: selector, Limit: listPageSize}
	for {
		list, err := resource(c, gvr, namespace).List(ctx, opts)
		if meta.IsNoMatchError(err) || apierrors.IsNotFound(err) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		for i := range list.Items {
			if filter == nil || filter(&list.Items[i]) {
				refs = append(refs, Ref{Namespace: list.Items[i].GetNamespace(), Name: list.Items[i].GetName()})
			}
		}
		if list.GetContinue() == "" {
			return refs, nil
		}
		opts.Continue = list.GetContinue()
	}
}

func (r Ref) String() string {
	if r.Namespace == "" {
		return r.Name
	}
	return r.Namespace + "/" + r.Name
}

func resource(c dynamic.Interface, gvr schema.GroupVersionResource, namespace string) dynamic.ResourceInterface {
	if namespace == "" {
		return c.Resource(gvr)
	}
	return c.Resource(gvr).Namespace(namespace)
}
