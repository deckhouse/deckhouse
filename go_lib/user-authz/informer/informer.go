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

// Package informer builds the informer factory the authorization webhook and permission-browser
// share: the RBAC objects and Namespaces they cache, without the parts neither of them reads.
package informer

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

// NewFactory returns a shared informer factory whose informers store objects without the parts
// neither the webhook nor permission-browser reads (see StripUnread). resync is passed to the factory
// as is.
func NewFactory(client kubernetes.Interface, resync time.Duration) informers.SharedInformerFactory {
	return informers.NewSharedInformerFactoryWithOptions(client, resync, informers.WithTransform(StripUnread))
}

// StripUnread is an informer transform that drops from the object, before it is stored, what neither
// the webhook nor permission-browser reads; anything that is not an object passes through:
//   - metadata.managedFields, about a third of an RBAC object. It is set to nil rather than to an empty
//     list: an object written back with an empty list would reset its managed fields on the API
//     server, one written back without them keeps them.
//   - the annotation kubectl.kubernetes.io/last-applied-configuration, a copy of the whole object that
//     kubectl apply and client-side apply of Argo CD leave. The webhook reads no annotations, and
//     permission-browser reads only its own keys. Only that key is deleted: the other annotations and
//     the map itself stay, even when it is left empty.
//
// The cached objects are never written back. The informer hands the transform a freshly decoded
// object and never a tombstone, so the object is changed in place.
func StripUnread(obj any) (any, error) {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return obj, nil //nolint:nilerr // not an object: nothing to strip, and the informer must keep it
	}
	if accessor.GetManagedFields() != nil {
		accessor.SetManagedFields(nil)
	}
	// The map of a freshly decoded object is its own.
	delete(accessor.GetAnnotations(), corev1.LastAppliedConfigAnnotation)
	return obj, nil
}
