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

// Package instanceclassusage records which NodeGroups consume each cloud
// InstanceClass in InstanceClass.status.nodeGroupConsumers.
//
// This protects an in-use InstanceClass from deletion: the validating webhook
// refuses to delete a class whose status.nodeGroupConsumers is non-empty.
//
// This replaces the shell-operator hook hooks/set_instance_class_ng_usage.go.
// The hook's only active trigger was the NodeGroup binding (its InstanceClass and
// cloud-provider-Secret bindings were passive, ExecuteHookOnEvents=false), so a
// primary NodeGroup watch reproduces its triggers; every reconcile recomputes the
// consumer lists for all InstanceClasses of the active kind.
package instanceclassusage

import (
	"context"
	"encoding/json"
	"reflect"
	"sort"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/register"
)

const (
	cloudProviderSecretNamespace = "kube-system"
	cloudProviderSecretName      = "d8-node-manager-cloud-provider"
	instanceClassKindKey         = "instanceClassKind"

	// InstanceClass CRDs serve deckhouse.io/v1alpha1 uniformly (v1-only kinds also
	// serve v1alpha1 via conversion), so a single version lists and patches every
	// provider kind. status is a plain field (no status subresource), so the main
	// resource is patched directly.
	instanceClassGroup   = "deckhouse.io"
	instanceClassVersion = "v1alpha1"

	consumersField = "nodeGroupConsumers"
)

func init() {
	register.RegisterController("node-instanceclass-ng-usage", &deckhousev1.NodeGroup{}, &Reconciler{})
}

type Reconciler struct {
	register.Base
}

func (r *Reconciler) SetupWatches(register.Watcher) {}

func (r *Reconciler) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	kindInUse, err := r.instanceClassKind(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}
	if kindInUse == "" {
		return ctrl.Result{}, nil
	}

	consumers, err := r.nodeGroupConsumers(ctx, kindInUse)
	if err != nil {
		return ctrl.Result{}, err
	}

	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{Group: instanceClassGroup, Version: instanceClassVersion, Kind: kindInUse + "List"})
	if err := r.Client.List(ctx, list); err != nil {
		return ctrl.Result{}, err
	}

	for i := range list.Items {
		ic := &list.Items[i]
		desired := consumers[ic.GetName()]
		if desired == nil {
			desired = []string{}
		}
		sort.Strings(desired)

		current, _, _ := unstructured.NestedStringSlice(ic.Object, "status", consumersField)
		if slicesEqual(current, desired) {
			continue
		}

		if err := r.patchConsumers(ctx, kindInUse, ic.GetName(), desired); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			logger.Error(err, "failed to patch InstanceClass consumers", "kind", kindInUse, "name", ic.GetName())
			return ctrl.Result{}, err
		}
		logger.Info("updated InstanceClass consumers", "kind", kindInUse, "name", ic.GetName(), "consumers", desired)
	}

	return ctrl.Result{}, nil
}

// instanceClassKind reads the active provider InstanceClass kind from the
// d8-node-manager-cloud-provider Secret, mirroring the hook's dynamic binding kind.
func (r *Reconciler) instanceClassKind(ctx context.Context) (string, error) {
	secret := &corev1.Secret{}
	if err := r.Client.Get(ctx, types.NamespacedName{Namespace: cloudProviderSecretNamespace, Name: cloudProviderSecretName}, secret); err != nil {
		if apierrors.IsNotFound(err) {
			return "", nil
		}
		return "", err
	}
	return string(secret.Data[instanceClassKindKey]), nil
}

// nodeGroupConsumers maps an InstanceClass name to the CloudEphemeral NodeGroups
// that reference it via spec.cloudInstances.classReference of the active kind.
func (r *Reconciler) nodeGroupConsumers(ctx context.Context, kindInUse string) (map[string][]string, error) {
	ngList := &deckhousev1.NodeGroupList{}
	if err := r.Client.List(ctx, ngList); err != nil {
		return nil, err
	}

	consumers := make(map[string][]string)
	for i := range ngList.Items {
		ng := &ngList.Items[i]
		if ng.Spec.NodeType != deckhousev1.NodeTypeCloudEphemeral || ng.Spec.CloudInstances == nil {
			continue
		}
		ref := ng.Spec.CloudInstances.ClassReference
		if ref.Kind != kindInUse {
			continue
		}
		consumers[ref.Name] = append(consumers[ref.Name], ng.Name)
	}
	return consumers, nil
}

// patchConsumers rewrites status.nodeGroupConsumers on the main resource with a JSON
// merge patch. InstanceClass has no status subresource, so the plain object is patched.
func (r *Reconciler) patchConsumers(ctx context.Context, kind, name string, consumers []string) error {
	ic := &unstructured.Unstructured{}
	ic.SetGroupVersionKind(schema.GroupVersionKind{Group: instanceClassGroup, Version: instanceClassVersion, Kind: kind})
	ic.SetName(name)

	body := map[string]any{"status": map[string]any{consumersField: consumers}}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	return r.Client.Patch(ctx, ic, client.RawPatch(types.MergePatchType, raw))
}

func slicesEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}
