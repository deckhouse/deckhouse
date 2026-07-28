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

// Package nodebootstrap is the Cluster API bootstrap provider for immutable
// NodeGroups. For every Machine, the CAPI MachineSet clones a
// NodeBootstrapConfig from the group's NodeBootstrapConfigTemplate; this
// controller renders that machine's NodeConfig userdata — with the node name
// already filled in — into a Secret and advertises it through the config's
// status, which the CAPI Machine controller waits on before handing the
// userdata to the infrastructure provider.
package nodebootstrap

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	bootstrapv1alpha1 "github.com/deckhouse/node-controller/api/bootstrap.deckhouse.io/v1alpha1"
	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/register"
)

func init() {
	register.RegisterController(controllerName, &bootstrapv1alpha1.NodeBootstrapConfig{}, &Reconciler{})
}

type Reconciler struct {
	register.Base

	// reader is uncached: the token and cluster inputs the userdata is rendered
	// from live outside the manager's cache, and the decision whether the secret
	// already exists must see the live state — a stale cache read caused
	// duplicate objects on the nodeoperation controller before.
	reader client.Reader
}

func (r *Reconciler) Setup(mgr ctrl.Manager) error {
	r.reader = mgr.GetAPIReader()
	return nil
}

func (r *Reconciler) SetupWatches(w register.Watcher) {
	// A Machine gaining its owner reference on the config, or being paused, must
	// re-run the config cloned for it.
	w.Watches(&capiv1beta2.Machine{}, handler.EnqueueRequestsFromMapFunc(machineToConfig))
}

// machineToConfig enqueues the NodeBootstrapConfig a Machine boots from.
func machineToConfig(_ context.Context, obj client.Object) []reconcile.Request {
	machine, ok := obj.(*capiv1beta2.Machine)
	if !ok {
		return nil
	}
	ref := machine.Spec.Bootstrap.ConfigRef
	if ref.Kind != nodeBootstrapConfigKind || ref.APIGroup != bootstrapv1alpha1.GroupVersion.Group || ref.Name == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: machine.Namespace, Name: ref.Name}}}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	config := &bootstrapv1alpha1.NodeBootstrapConfig{}
	if err := r.Client.Get(ctx, req.NamespacedName, config); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if isPaused(config) {
		return ctrl.Result{}, nil
	}

	machine, err := r.ownerMachine(ctx, config)
	if err != nil {
		return ctrl.Result{}, err
	}
	if machine == nil {
		// The MachineSet sets the Machine owner shortly after cloning; the
		// config update that carries it re-enqueues this pass, and the requeue
		// is a backstop.
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}
	if isPaused(machine) {
		return ctrl.Result{}, nil
	}

	ngName := machine.Labels[machineNodeGroupLabel]
	if ngName == "" {
		logger.Info("machine carries no node-group label yet, waiting", "machine", machine.Name)
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	ng := &v1.NodeGroup{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: ngName}, ng); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	// Defensive: bashible groups never reference this template, so a
	// non-immutable group here means a misconfiguration, not work to do.
	if ng.Spec.SystemType != v1.SystemTypeImmutable {
		logger.Info("node-group is not immutable, skipping", "nodeGroup", ngName)
		return ctrl.Result{}, nil
	}

	secretName := machine.Name + dataSecretSuffix

	// Bootstrap data is consumed once. If the secret already exists, do not
	// re-render it: rotating the group token must not churn a live machine's
	// userdata. Only make sure the status still points at it.
	existing := &corev1.Secret{}
	err = r.reader.Get(ctx, types.NamespacedName{Namespace: config.Namespace, Name: secretName}, existing)
	if err == nil {
		return ctrl.Result{}, r.ensureStatus(ctx, config, secretName)
	}
	if !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("get bootstrap secret %s: %w", secretName, err)
	}

	userdata, err := renderBootstrapData(ctx, r.Client, r.reader, ng, machine.Name)
	if err != nil {
		// The CA, token or digests may not be published yet; requeue with backoff.
		return ctrl.Result{}, fmt.Errorf("render bootstrap data for %s: %w", machine.Name, err)
	}

	secret := buildSecret(config, secretName, ngName, userdata)
	if err := r.Client.Create(ctx, secret); err != nil && !apierrors.IsAlreadyExists(err) {
		return ctrl.Result{}, fmt.Errorf("create bootstrap secret %s: %w", secretName, err)
	}
	logger.Info("bootstrap secret rendered", "machine", machine.Name, "secret", secretName)

	return ctrl.Result{}, r.ensureStatus(ctx, config, secretName)
}

// ownerMachine returns the Machine that controls the config, or nil when the
// MachineSet has not re-parented it from itself yet.
func (r *Reconciler) ownerMachine(ctx context.Context, config *bootstrapv1alpha1.NodeBootstrapConfig) (*capiv1beta2.Machine, error) {
	for _, ref := range config.OwnerReferences {
		if ref.Kind != machineKind {
			continue
		}
		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil || gv.Group != capiv1beta2.GroupVersion.Group {
			continue
		}
		machine := &capiv1beta2.Machine{}
		if err := r.Client.Get(ctx, types.NamespacedName{Namespace: config.Namespace, Name: ref.Name}, machine); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, nil
			}
			return nil, fmt.Errorf("get owner machine %s: %w", ref.Name, err)
		}
		return machine, nil
	}
	return nil, nil
}

// ensureStatus advertises the rendered secret through the config's status: the
// v1beta2 bootstrap contract the CAPI Machine controller waits on.
func (r *Reconciler) ensureStatus(ctx context.Context, config *bootstrapv1alpha1.NodeBootstrapConfig, secretName string) error {
	updated := config.DeepCopy()
	updated.Status.DataSecretName = ptr.To(secretName)
	updated.Status.Initialization = &bootstrapv1alpha1.NodeBootstrapInitializationStatus{DataSecretCreated: true}
	apimeta.SetStatusCondition(&updated.Status.Conditions, metav1.Condition{
		Type:               conditionDataSecretAvailable,
		Status:             metav1.ConditionTrue,
		Reason:             reasonRendered,
		Message:            "bootstrap userdata rendered from cluster state",
		ObservedGeneration: config.Generation,
	})

	if apiequality.Semantic.DeepEqual(config.Status, updated.Status) {
		return nil
	}
	if err := r.Client.Status().Patch(ctx, updated, client.MergeFrom(config)); err != nil {
		return fmt.Errorf("patch NodeBootstrapConfig status %s: %w", config.Name, err)
	}
	return nil
}

// buildSecret wraps the rendered userdata in the Secret capdvp reads as the
// machine's user-data. It is owned by the config so it is garbage-collected
// together with it (Machine -> NodeBootstrapConfig -> Secret).
func buildSecret(config *bootstrapv1alpha1.NodeBootstrapConfig, name, ngName string, userdata []byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: config.Namespace,
			Labels:    map[string]string{machineNodeGroupLabel: ngName},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         bootstrapv1alpha1.GroupVersion.String(),
				Kind:               nodeBootstrapConfigKind,
				Name:               config.Name,
				UID:                config.UID,
				Controller:         ptr.To(true),
				BlockOwnerDeletion: ptr.To(true),
			}},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			secretValueKey:  userdata,
			secretFormatKey: []byte(secretFormatCloudConfig),
		},
	}
}

func isPaused(obj client.Object) bool {
	_, paused := obj.GetAnnotations()[capiv1beta2.PausedAnnotation]
	return paused
}
