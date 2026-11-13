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
// the controller, deployed by that same release, cannot be running yet to recreate it. For a
// Secret with generated credentials that is not a gap but a loss: a new value breaks every
// application configured with the old one. Both release engines honour the
// `helm.sh/resource-policy: keep` annotation on the live object, so Protect stamps it on every
// object of a Kind the release still owns, right before the release runs, and fails the run if
// any is left unprotected. The controller adopts the Secrets and OAuth2Clients and removes the
// annotation together with the ownership metadata, after which Protect leaves them alone.
//
// Dex Passwords, which the chart of 1.76 rendered, are not adopted: the controller keeps the keep
// policy on every Password and leaves the release annotations in place. Their Kind therefore
// returns only the Passwords that are not protected yet, and a protected one does not count as
// owned. A Password the controller would recreate comes from User.spec.password, the password the
// User was created with, so pruning one loses the password changed in Dex, the password history
// and the lockout state.
//
// Nothing is removed when the module is disabled: the OAuth2Clients and Passwords go with the
// d8-user-authn namespace, and the Secrets of DexClients are owned by their DexClients. A Secret
// that outlives a disabled module keeps its value, so the applications using it survive
// re-enabling.
//
// Moving another kind to the controller means adding a Kind to the list the module hooks pass
// here; neither the hooks nor this package change.
//
// The release waits for Protect, so its cost is what a cluster with hundreds of objects pays on
// every start of deckhouse. Protect reads the objects only in lists of their metadata and decides
// by what the lists return: an object costs a request of its own only when it has to be stamped,
// and the stamps go out several at a time. A run that has nothing to stamp is the lists alone.
//
// Rollback: deckhouse runs nelm with forced adoption, so an older chart that renders the objects
// again takes them back over, with the content the controller left. One path keeps a leftover: if
// the release fails after Protect and deckhouse goes back to the previous version, the old chart
// renders the objects with the keep policy Protect set, since only the controller removes it. A
// DexClient deleted on that version then leaves its Secret and OAuth2Clients behind, and the client
// keeps working in Dex until the next update, whose controller removes the objects of deleted
// DexClients. To remove them at once, delete the Secret dex-client-<name> in the namespace of the
// DexClient and its OAuth2Clients in d8-user-authn, or remove the helm.sh/resource-policy
// annotation from them before deleting the DexClient. The same holds for a User deleted on that
// version: its Password stays, and the user can still log in until the next update, whose
// controller deletes the Passwords of deleted Users. To remove it at once, delete the Password of
// the user in d8-user-authn as well.
package handover

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/metadata"
)

const (
	// ReleaseName is the Helm release of the module.
	ReleaseName = "user-authn"

	ReleaseNameAnnotation    = "meta.helm.sh/release-name"
	ResourcePolicyAnnotation = "helm.sh/resource-policy"
	ResourcePolicyKeep       = "keep"

	listPageSize = 500

	// stampWorkers is how many stamps are in flight at once. A stamp is a merge patch of the
	// metadata, 16 ms on the API server of a test cluster for a Secret. One at a time, the 1800
	// objects of 600 DexClients would take about 30 seconds of round trips alone; with ten in
	// flight even a stamp of 200 ms reaches the request rate of the client of the hook (50 a
	// second), which then sets the time.
	stampWorkers = 10
)

// Ref is one object.
type Ref struct {
	Namespace string
	Name      string
}

// Object is an object as a list of its metadata returned it: what Protect decides by.
type Object struct {
	Ref
	Annotations map[string]string
}

// Kind is one kind of objects the chart used to render and the controller owns now.
type Kind struct {
	// Name is how the kind appears in errors.
	Name     string
	Resource schema.GroupVersionResource

	// Rendered returns the objects the chart may have rendered, as List returns them. Only those
	// the release really owns are touched, so returning more is harmless; returning fewer loses
	// objects. Derive them from the source objects rather than from labels: labels of a rendered
	// object may come from user input and not be what the chart set by itself. It must not read
	// the objects one by one: a kind has hundreds of them.
	Rendered func(ctx context.Context, c metadata.Interface) ([]Object, error)
}

// Result counts the objects of the release Protect found: Owned is every one the release still
// owns, protected or not, and Stamped those it protected now.
type Result struct {
	Owned   int
	Stamped int
}

// stamp is one object to protect.
type stamp struct {
	kind Kind
	ref  Ref
}

// Protect stamps the keep policy on every object of kinds still owned by the release. It fails if
// an object could not be protected: the release would prune it.
func Protect(ctx context.Context, c metadata.Interface, kinds []Kind) (Result, error) {
	patch, err := json.Marshal(map[string]any{
		"metadata": map[string]any{"annotations": map[string]any{ResourcePolicyAnnotation: ResourcePolicyKeep}},
	})
	if err != nil {
		return Result{}, fmt.Errorf("marshal patch: %w", err)
	}

	var res Result
	var errs []error
	var stamps []stamp
	for _, kind := range kinds {
		objects, err := kind.Rendered(ctx, c)
		if err != nil {
			errs = append(errs, fmt.Errorf("list rendered %s: %w", kind.Name, err))
			continue
		}
		for _, obj := range objects {
			if !OwnedByRelease(obj) {
				continue
			}
			res.Owned++
			if needsKeep(obj) {
				stamps = append(stamps, stamp{kind: kind, ref: obj.Ref})
			}
		}
	}

	stamped, stampErrs := applyStamps(ctx, c, stamps, patch)
	res.Stamped = stamped
	return res, errors.Join(append(errs, stampErrs...)...)
}

// applyStamps patches the objects, stampWorkers at a time, and returns how many it protected and
// what went wrong with the others, in the order of stamps.
func applyStamps(ctx context.Context, c metadata.Interface, stamps []stamp, patch []byte) (int, []error) {
	results := make([]error, len(stamps))
	stamped := make([]bool, len(stamps))
	next := make(chan int)

	var workers sync.WaitGroup
	for range min(stampWorkers, len(stamps)) {
		workers.Go(func() {
			for i := range next {
				stamped[i], results[i] = applyStamp(ctx, c, stamps[i], patch)
			}
		})
	}
	for i := range stamps {
		next <- i
	}
	close(next)
	workers.Wait()

	count := 0
	var errs []error
	for i := range stamps {
		if stamped[i] {
			count++
		}
		if results[i] != nil {
			errs = append(errs, results[i])
		}
	}
	return count, errs
}

// applyStamp patches one object and reports whether it is protected now. An object that is gone
// needs no protection.
func applyStamp(ctx context.Context, c metadata.Interface, s stamp, patch []byte) (bool, error) {
	patched, err := resource(c, s.kind.Resource, s.ref.Namespace).Patch(ctx, s.ref.Name, types.MergePatchType, patch, metav1.PatchOptions{})
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("patch %s %s: %w", s.kind.Name, s.ref, err)
	}
	if needsKeep(objectOf(patched)) {
		return false, fmt.Errorf("keep policy is not set on %s %s: refusing to proceed to avoid prune", s.kind.Name, s.ref)
	}
	return true, nil
}

func objectOf(obj *metav1.PartialObjectMetadata) Object {
	return Object{
		Ref:         Ref{Namespace: obj.GetNamespace(), Name: obj.GetName()},
		Annotations: obj.GetAnnotations(),
	}
}

// OwnedByRelease reports whether obj carries the ownership annotation of the release of the
// module.
func OwnedByRelease(obj Object) bool {
	return obj.Annotations[ReleaseNameAnnotation] == ReleaseName
}

// needsKeep reports whether obj belongs to the release and is not protected yet.
func needsKeep(obj Object) bool {
	return OwnedByRelease(obj) && obj.Annotations[ResourcePolicyAnnotation] != ResourcePolicyKeep
}

// List pages through the metadata of the objects of gvr in namespace (all namespaces when empty)
// matching selector and returns those passing filter (all when nil). Only the metadata is asked
// for, so a list of every Secret of the cluster carries no secret and stays small. A resource
// whose CRD is not installed has no objects.
func List(ctx context.Context, c metadata.Interface, gvr schema.GroupVersionResource, namespace, selector string, filter func(Object) bool) ([]Object, error) {
	var objects []Object
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
			if obj := objectOf(&list.Items[i]); filter == nil || filter(obj) {
				objects = append(objects, obj)
			}
		}
		if list.GetContinue() == "" {
			return objects, nil
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

func resource(c metadata.Interface, gvr schema.GroupVersionResource, namespace string) metadata.ResourceInterface {
	if namespace == "" {
		return c.Resource(gvr)
	}
	return c.Resource(gvr).Namespace(namespace)
}
