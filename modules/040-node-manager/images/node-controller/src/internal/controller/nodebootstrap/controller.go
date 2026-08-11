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
// NodeGroups: it renders each Machine's NodeConfig userdata into a Secret and
// advertises it through the NodeBootstrapConfig status CAPI waits on.
package nodebootstrap

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
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

	// reader is uncached: render inputs live outside the manager's cache, and
	// the does-the-secret-exist decision must see live state — a stale cache
	// read caused duplicate objects on the nodeoperation controller before.
	reader client.Reader
}

func (r *Reconciler) Setup(_ context.Context, mgr ctrl.Manager) error {
	r.reader = mgr.GetAPIReader()
	return nil
}

func (r *Reconciler) SetupWatches(w register.Watcher) {
	// A Machine gaining its owner reference on the config, or being paused, must
	// re-run the config cloned for it. Predicated because CAPI rewrites Machine
	// status constantly and every event here re-renders the whole userdata.
	w.Watches(&capiv1beta2.Machine{}, handler.EnqueueRequestsFromMapFunc(machineToConfig),
		builder.WithPredicates(predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				return machineRenderInputsChanged(e.ObjectOld, e.ObjectNew)
			},
		}))
}

// machineRenderInputsChanged reports whether an update touched anything a pass
// reads off its Machine: which config it boots from, which group it belongs to,
// whether it is paused, and whether the bootstrap data has been consumed.
func machineRenderInputsChanged(before, after client.Object) bool {
	oldMachine, okOld := before.(*capiv1beta2.Machine)
	newMachine, okNew := after.(*capiv1beta2.Machine)
	if !okOld || !okNew {
		return true
	}
	if oldMachine.Spec.Bootstrap.ConfigRef != newMachine.Spec.Bootstrap.ConfigRef {
		return true
	}
	if oldMachine.Labels[machineNodeGroupLabel] != newMachine.Labels[machineNodeGroupLabel] {
		return true
	}
	if isPaused(oldMachine) != isPaused(newMachine) {
		return true
	}
	return consumed(oldMachine) != consumed(newMachine)
}

// machineToConfig enqueues the NodeBootstrapConfig a Machine boots from.
func machineToConfig(_ context.Context, obj client.Object) []reconcile.Request {
	machine, ok := obj.(*capiv1beta2.Machine)
	if !ok {
		return nil
	}
	ref := machine.Spec.Bootstrap.ConfigRef
	if ref.Kind != nodeBootstrapConfigKind || ref.APIGroup != bootstrapv1alpha1.GroupVersion.Group {
		return nil
	}
	if ref.Name == "" {
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

	// Paused is the one wait this controller does not write a condition for:
	// paused means leave the object alone, status included, and the annotation
	// already says why nothing is happening.
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
		return ctrl.Result{RequeueAfter: ownerWaitInterval},
			r.notReady(ctx, config, reasonWaitingForOwner, "the MachineSet has not made a Machine the owner of this config yet")
	}
	if isPaused(machine) {
		return ctrl.Result{}, nil
	}

	ngName := machine.Labels[machineNodeGroupLabel]
	if ngName == "" {
		logger.Info("machine carries no node-group label yet, waiting", "machine", machine.Name)
		return ctrl.Result{RequeueAfter: ownerWaitInterval},
			r.notReady(ctx, config, reasonWaitingForOwner,
				fmt.Sprintf("machine %s carries no %s label yet", machine.Name, machineNodeGroupLabel))
	}

	// Both answers below are requeued rather than left to an event: this
	// controller watches Machines, not NodeGroups, so a group created or
	// switched to Immutable later would only be noticed if its Machine changed.
	ng := &v1.NodeGroup{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: ngName}, ng); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: ownerWaitInterval}, r.notReady(ctx, config, reasonNodeGroupUnusable,
				fmt.Sprintf("NodeGroup %s does not exist yet", ngName))
		}
		return ctrl.Result{}, err
	}
	// Defensive: bashible groups never reference this template, so a
	// non-immutable group here means a misconfiguration, not work to do.
	if ng.Spec.SystemType != v1.SystemTypeImmutable {
		logger.Info("node-group is not immutable, skipping", "nodeGroup", ngName)
		return ctrl.Result{RequeueAfter: ownerWaitInterval}, r.notReady(ctx, config, reasonNodeGroupUnusable,
			fmt.Sprintf("NodeGroup %s is not immutable, so there is no bootstrap data to render yet", ngName))
	}

	return r.ensureBootstrapData(ctx, config, ng, machine, logger)
}

// ensureBootstrapData renders the machine's userdata into its bootstrap secret
// and advertises the secret through the config's status.
func (r *Reconciler) ensureBootstrapData(ctx context.Context, config *bootstrapv1alpha1.NodeBootstrapConfig, ng *v1.NodeGroup, machine *capiv1beta2.Machine, logger logr.Logger) (ctrl.Result, error) {
	secretName := machine.Name + dataSecretSuffix

	existing := &corev1.Secret{}
	err := r.reader.Get(ctx, types.NamespacedName{Namespace: config.Namespace, Name: secretName}, existing)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("get bootstrap secret %s: %w", secretName, err)
	}
	rendered := err == nil

	// The userdata embeds the group's bootstrap token, which expires four hours
	// after issue, so keep re-rendering until the data is consumed (VM built
	// from it or node joined) — after that nobody reads the Secret again.
	if rendered && consumed(machine) {
		return ctrl.Result{}, r.ensureStatus(ctx, config, secretName)
	}

	userdata, err := renderBootstrapData(ctx, r.Client, r.reader, ng, machine.Name)
	if err != nil {
		// The CA, token or digests may not be published yet; requeue with backoff.
		renderErr := fmt.Errorf("render bootstrap data for %s: %w", machine.Name, err)
		if statusErr := r.notReady(ctx, config, reasonRenderFailed, renderErr.Error()); statusErr != nil {
			logger.Error(statusErr, "record why the bootstrap data is unavailable", "config", config.Name)
		}
		return ctrl.Result{}, renderErr
	}

	secret := buildSecret(config, secretName, ng.Name, userdata)
	switch {
	case !rendered:
		if err := r.Client.Create(ctx, secret); err != nil && !apierrors.IsAlreadyExists(err) {
			return ctrl.Result{}, fmt.Errorf("create bootstrap secret %s: %w", secretName, err)
		}
		logger.Info("bootstrap secret rendered", "machine", machine.Name, "secret", secretName)

	case !apiequality.Semantic.DeepEqual(existing.Data, secret.Data):
		// Only the one label this controller owns: the Secret may carry others
		// that are not ours to drop.
		if existing.Labels == nil {
			existing.Labels = map[string]string{}
		}
		existing.Labels[machineNodeGroupLabel] = ng.Name
		existing.Data = secret.Data
		if err := r.Client.Update(ctx, existing); err != nil {
			return ctrl.Result{}, fmt.Errorf("refresh bootstrap secret %s: %w", secretName, err)
		}
		logger.V(1).Info("bootstrap secret refreshed while the node has not registered", "machine", machine.Name, "secret", secretName)
	}

	if err := r.ensureStatus(ctx, config, secretName); err != nil {
		return ctrl.Result{}, err
	}
	// Nothing watches the group's token secret, so the refresh has to be driven
	// by the clock until the node shows up.
	return ctrl.Result{RequeueAfter: bootstrapRefreshInterval}, nil
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

	return r.patchStatus(ctx, config, updated)
}

// notReady records why the bootstrap data is not available yet. Without it an
// operator looking at a Machine stuck on WaitingForBootstrapScript finds an
// empty condition list and no sign of which input is missing.
func (r *Reconciler) notReady(ctx context.Context, config *bootstrapv1alpha1.NodeBootstrapConfig, reason, message string) error {
	updated := config.DeepCopy()
	apimeta.SetStatusCondition(&updated.Status.Conditions, metav1.Condition{
		Type:               conditionDataSecretAvailable,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: config.Generation,
	})

	return r.patchStatus(ctx, config, updated)
}

func (r *Reconciler) patchStatus(ctx context.Context, config, updated *bootstrapv1alpha1.NodeBootstrapConfig) error {
	if apiequality.Semantic.DeepEqual(config.Status, updated.Status) {
		return nil
	}
	if err := r.Client.Status().Patch(ctx, updated, client.MergeFrom(config)); err != nil {
		return fmt.Errorf("patch NodeBootstrapConfig status %s: %w", config.Name, err)
	}
	return nil
}

// buildSecret wraps the rendered userdata in the Secret capdvp reads, owned by
// the config for GC (Machine -> NodeBootstrapConfig -> Secret). Deliberately no
// BlockOwnerDeletion: OwnerReferencesPermissionEnforcement would refuse Create.
func buildSecret(config *bootstrapv1alpha1.NodeBootstrapConfig, name, ngName string, userdata []byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: config.Namespace,
			Labels:    map[string]string{machineNodeGroupLabel: ngName},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: bootstrapv1alpha1.GroupVersion.String(),
				Kind:       nodeBootstrapConfigKind,
				Name:       config.Name,
				UID:        config.UID,
				Controller: ptr.To(true),
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

// consumed reports whether the bootstrap data has been handed over: the
// provider built the VM from it, or the node registered. Rests on the CAPI
// contract that a provider reads the secret once, at machine creation.
func consumed(machine *capiv1beta2.Machine) bool {
	if machine.Status.NodeRef.IsDefined() {
		return true
	}
	return ptr.Deref(machine.Status.Initialization.InfrastructureProvisioned, false)
}
