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

package dexauthenticator

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"user-authn-controller/internal/adoption"
	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

// ReferenceGrantGVK is the ReferenceGrant of the Gateway API. The Gateway API versions alb accepts
// serve it as v1 and v1beta1, one object stored as v1beta1; v1beta1 is the one upstream drops.
var ReferenceGrantGVK = schema.GroupVersionKind{Group: gatewayGroup, Version: "v1", Kind: "ReferenceGrant"}

// referenceGrantApp is the app label of the ReferenceGrant objects of the controller.
const referenceGrantApp = "dex-authenticator-reference-grant"

// referenceGrantName is the name of the ReferenceGrant of a namespace in d8-user-authn.
func referenceGrantName(namespace string) string {
	return safeName("dex-authenticator-" + namespace).name
}

// referenceGrantLabels are the labels of the ReferenceGrant of a namespace: the controller finds
// its grants by them and tells their namespace by LabelNamespace.
func referenceGrantLabels(namespace string) map[string]string {
	return map[string]string{
		labelHeritage:           heritageValue,
		labelModule:             moduleValue,
		labelApp:                referenceGrantApp,
		LabelNamespace:          namespace,
		adoption.ManagedByLabel: adoption.ManagedByValue,
	}
}

// grantedInstances are the instances whose Service the HTTPRoute objects of a namespace are
// granted, by namespace: the instances that serve a DexAuthenticator of the namespace.
func grantedInstances(entries map[types.NamespacedName]Entry) map[string][]string {
	out := map[string][]string{}
	for key, entry := range entries {
		if entry.Record == nil || slices.Contains(out[key.Namespace], entry.Instance) {
			continue
		}
		out[key.Namespace] = append(out[key.Namespace], entry.Instance)
	}
	for _, instances := range out {
		slices.Sort(instances)
	}
	return out
}

// referenceGrantSpec lets the HTTPRoute objects of the namespace refer to the Services of the
// instances. The Service of a DexAuthenticator is an ExternalName on the Service of its instance in
// d8-user-authn. alb follows an ExternalName in the auth-url of an HTTPRoute only to a Service of the
// cluster named by its full DNS name, which instanceServiceHost is, and only with a ReferenceGrant
// from the namespace of the route to the namespace of the target. Without the grant the auth check
// of every application behind alb fails.
func referenceGrantSpec(namespace string, instances []string) map[string]any {
	to := make([]any, 0, len(instances))
	for _, instance := range instances {
		to = append(to, map[string]any{"group": "", "kind": "Service", "name": instance})
	}
	return map[string]any{
		"from": []any{map[string]any{"group": gatewayGroup, "kind": "HTTPRoute", "namespace": namespace}},
		"to":   to,
	}
}

// syncReferenceGrants keeps a ReferenceGrant in d8-user-authn for every namespace with a served
// DexAuthenticator, and removes the grants of the namespaces that have none any more. Only the
// grants with the labels of the controller are touched. The informer triggers reconciliation,
// which lists the current grants directly from the API in d8-user-authn.
func (r *TableReconciler) syncReferenceGrants(ctx context.Context, entries map[types.NamespacedName]Entry) error {
	want := grantedInstances(entries)

	list := controller.List(ReferenceGrantGVK)
	if err := r.reader.List(ctx, list, client.InNamespace(naming.DexNamespace),
		client.MatchingLabels{labelApp: referenceGrantApp, adoption.ManagedByLabel: adoption.ManagedByValue}); err != nil {
		return fmt.Errorf("list the ReferenceGrants: %w", err)
	}

	var errs []error
	have := map[string]bool{}
	for i := range list.Items {
		current := &list.Items[i]
		namespace := current.GetLabels()[LabelNamespace]
		instances, wanted := want[namespace]
		if !wanted || have[namespace] || current.GetName() != referenceGrantName(namespace) {
			if err := r.client.Delete(ctx, current); err != nil && !apierrors.IsNotFound(err) {
				errs = append(errs, fmt.Errorf("delete ReferenceGrant %s: %w", current.GetName(), err))
			}
			continue
		}
		have[namespace] = true
		spec := referenceGrantSpec(namespace, instances)
		labels := referenceGrantLabels(namespace)
		if reflect.DeepEqual(current.Object["spec"], spec) && maps.Equal(current.GetLabels(), labels) {
			continue
		}
		updated := current.DeepCopy()
		updated.Object["spec"] = spec
		updated.SetLabels(labels)
		if err := r.client.Update(ctx, updated); err != nil {
			errs = append(errs, fmt.Errorf("update ReferenceGrant %s: %w", current.GetName(), err))
		}
	}

	for namespace, instances := range want {
		if have[namespace] {
			continue
		}
		obj := controller.Object(ReferenceGrantGVK)
		obj.SetNamespace(naming.DexNamespace)
		obj.SetName(referenceGrantName(namespace))
		obj.SetLabels(referenceGrantLabels(namespace))
		obj.Object["spec"] = referenceGrantSpec(namespace, instances)
		switch err := r.client.Create(ctx, obj); {
		case apierrors.IsAlreadyExists(err):
			// A ReferenceGrant of somebody else under the name: it is left alone.
			r.log.Info("a ReferenceGrant that the controller did not make has the name of the grant of a namespace",
				"referenceGrant", obj.GetName(), "namespace", namespace)
		case err != nil:
			errs = append(errs, fmt.Errorf("create ReferenceGrant %s: %w", obj.GetName(), err))
		}
	}
	return errors.Join(errs...)
}
