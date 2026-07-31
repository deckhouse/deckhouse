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

package common

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

const instanceClassKindSuffix = "InstanceClass"

// InstanceClassAPIVersion returns the API version every InstanceClass read must use. The cloud
// provider module publishes it in the registration Secret next to instanceClassKind, and it is
// always the CRD's storage version, so a read never goes through a conversion webhook. An empty
// result means the Secret is not published yet; callers must wait rather than pick a version of
// their own.
//
// The version is deliberately not resolved from discovery. Two independent things make a
// non-pinned read return different values for the same unchanged object:
//
//   - Which version a group resolves to depends on whichever version the RESTMapper happened to
//     load first, and it is then cached for the whole process lifetime (controller-runtime
//     pkg/client/apiutil/restmapper.go, "Prepend if preferred version, else append"). Two pods of
//     the same build can disagree, permanently.
//   - Reading a non-storage version also changes answer the moment the CRD's conversion webhook is
//     wired. Deckhouse CRDs ship without spec.conversion and get it patched in at runtime, so
//     early in a cluster's life the same read returns the raw stored value instead of the
//     converted one.
//
// Either difference changes the instance-class checksum. That checksum names an immutable
// infrastructure MachineTemplate, so a changed checksum renames the template, and the rename
// recreates every node in the NodeGroup.
func InstanceClassAPIVersion(ctx context.Context, r client.Reader) (string, error) {
	secret := &corev1.Secret{}
	key := types.NamespacedName{Namespace: CloudProviderSecretNamespace, Name: CloudProviderSecretName}
	if err := r.Get(ctx, key, secret); err != nil {
		if apierrors.IsNotFound(err) {
			return "", nil
		}
		return "", fmt.Errorf("get %s secret: %w", CloudProviderSecretName, err)
	}
	return string(secret.Data[InstanceClassAPIVersionKey]), nil
}

// ServedInstanceClassKinds discovers the InstanceClass kinds the cluster actually serves, so the
// controllers can watch them. The kind is provider-specific (AWSInstanceClass, DVPInstanceClass,
// …) and therefore cannot be compiled in; discovery is done once at controller setup.
//
// Without these watches an InstanceClass edit reaches the nodes only on the next resync — up to
// ten minutes during which the rendered MachineClass, the machine template and the bashible
// context all describe the previous instance type. The helm implementation reacted immediately.
//
// Only the pinned version is enumerated, never the group's preferred one: the derived-status
// service reads these objects at exactly that version, and a watch on any other version would both
// miss the informer it is supposed to share and observe values a conversion webhook rewrote.
func ServedInstanceClassKinds(ctx context.Context, r client.Reader, cfg *rest.Config) ([]schema.GroupVersionKind, error) {
	version, err := InstanceClassAPIVersion(ctx, r)
	if err != nil {
		return nil, err
	}
	// The provider module has not registered yet. The resync still covers InstanceClass objects.
	if version == "" {
		return nil, nil
	}

	dc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("build discovery client: %w", err)
	}

	gv := schema.GroupVersion{Group: v1.GroupVersion.Group, Version: version}
	list, err := dc.ServerResourcesForGroupVersion(gv.String())
	if err != nil {
		// The provider's CRDs may not be applied yet; the resync still covers their objects.
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list resources of %s: %w", gv.String(), err)
	}

	var gvks []schema.GroupVersionKind
	for _, res := range list.APIResources {
		// Subresources repeat the parent's Kind ("…instanceclasses/status") and would register
		// the same watch twice.
		if strings.Contains(res.Name, "/") {
			continue
		}
		if strings.HasSuffix(res.Kind, instanceClassKindSuffix) && res.Kind != instanceClassKindSuffix {
			gvks = append(gvks, gv.WithKind(res.Kind))
		}
	}
	return gvks, nil
}

// InstanceClassToNodeGroups maps an InstanceClass event to the NodeGroups whose classReference
// points at it. Matching on both kind and name keeps an edit of one provider's class from
// re-rendering NodeGroups that reference another.
func InstanceClassToNodeGroups(ctx context.Context, r client.Reader, obj client.Object) []reconcile.Request {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil
	}

	ngList := &v1.NodeGroupList{}
	if err := r.List(ctx, ngList); err != nil {
		log.FromContext(ctx).Error(err, "list nodegroups for instance class event", "kind", u.GetKind(), "name", u.GetName())
		return nil
	}

	requests := make([]reconcile.Request, 0, 1)
	for i := range ngList.Items {
		ng := &ngList.Items[i]
		if ng.Spec.CloudInstances == nil {
			continue
		}
		ref := ng.Spec.CloudInstances.ClassReference
		if ref.Kind == u.GetKind() && ref.Name == u.GetName() {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: ng.Name}})
		}
	}
	return requests
}
