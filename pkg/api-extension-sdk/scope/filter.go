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

package scope

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
)

// The helpers below restrict what the storage serves. Apply them in the REST
// storage (List, Get, Watch), never on the encoded response: the handler
// converts the result to a Table (kubectl asks for as=Table) after the storage
// returns, and a watch is encoded per event.

// Allowed reports whether an object in namespace may be served to the request
// of ctx. Without a decision everything is allowed; with one, only objects in a
// namespace of the set, and never a cluster-scoped object (namespace "").
func Allowed(ctx context.Context, namespace string) bool {
	d, ok := FromContext(ctx)
	if !ok {
		return true
	}
	return d.Allows(namespace)
}

// Restricted returns the set the request of ctx is restricted to, so a storage
// can query only those namespaces. ok is false when nothing is restricted.
func Restricted(ctx context.Context) ([]string, bool) {
	d, ok := FromContext(ctx)
	if !ok {
		return nil, false
	}
	return sets.List(d.Namespaces), true
}

// Filter returns the objects of objs the request of ctx may see. Objects
// without object metadata are dropped when a decision applies.
func Filter(ctx context.Context, objs []runtime.Object) []runtime.Object {
	if _, ok := FromContext(ctx); !ok {
		return objs
	}
	out := make([]runtime.Object, 0, len(objs))
	for _, obj := range objs {
		if objectAllowed(ctx, obj) {
			out = append(out, obj)
		}
	}
	return out
}

// FilterList removes from list (any list type meta.ExtractList understands)
// the items the request of ctx may not see. It does not touch the list
// metadata, so resourceVersion and continue tokens stay as the storage set
// them.
func FilterList(ctx context.Context, list runtime.Object) error {
	if _, ok := FromContext(ctx); !ok {
		return nil
	}
	items, err := meta.ExtractList(list)
	if err != nil {
		return err
	}
	return meta.SetList(list, Filter(ctx, items))
}

// WatchFilter drops the events of objects the request of ctx may not see.
// Bookmark and error events pass. The decision is fixed for the life of the
// stream: kube-apiserver ends the stream when the set changes.
func WatchFilter(ctx context.Context, w watch.Interface) watch.Interface {
	if _, ok := FromContext(ctx); !ok {
		return w
	}
	return watch.Filter(w, func(e watch.Event) (watch.Event, bool) {
		switch e.Type {
		case watch.Bookmark, watch.Error:
			return e, true
		}
		return e, objectAllowed(ctx, e.Object)
	})
}

// CheckGet returns NotFound when the request of ctx may not see an object
// named name in namespace, before the storage looks it up: the answer is the
// same whether the object exists or not and names neither the scope nor the
// set. Call it at the start of Get.
func CheckGet(ctx context.Context, gr schema.GroupResource, namespace, name string) error {
	if Allowed(ctx, namespace) {
		return nil
	}
	return apierrors.NewNotFound(gr, name)
}

func objectAllowed(ctx context.Context, obj runtime.Object) bool {
	acc, err := meta.Accessor(obj)
	if err != nil {
		return false
	}
	return Allowed(ctx, acc.GetNamespace())
}
