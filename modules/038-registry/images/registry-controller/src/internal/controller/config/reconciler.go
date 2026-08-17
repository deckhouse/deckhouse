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

// Package config reconciles RegistryConfig, the resolved registry configuration
// of the cluster.
//
// The module renders the object from ModuleConfig in the normal case, but the CR
// is a contract rather than Deckhouse property: an administrator edits it
// directly when the Deckhouse operator is down. So this reconciler treats the
// object as untrusted input and reports what it finds in the status, instead of
// assuming a well-formed spec produced by a template.
package config

import (
	"context"
	"fmt"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"

	"github.com/deckhouse/registry-controller/internal/register"
)

// ControllerName identifies this controller in logs, events and the
// --disable-controllers flag.
const ControllerName = "registry-config"

func init() {
	register.RegisterController(ControllerName, &registryv1alpha1.RegistryConfig{}, &Reconciler{})
}

// Reconciler validates RegistryConfig and reports its health.
type Reconciler struct {
	register.Base
}

var _ register.Reconciler = (*Reconciler)(nil)

// SetupWatches declares no secondary watches yet. The preflight probe of the
// primary upstream will add them.
func (r *Reconciler) SetupWatches(_ register.Watcher) {}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	cfg := &registryv1alpha1.RegistryConfig{}
	if err := r.Client.Get(ctx, req.NamespacedName, cfg); err != nil {
		// A deleted object needs no status. Components keep running on their last
		// applied configuration, which is the point of the design.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	condition := metav1.Condition{
		Type:               registryv1alpha1.ConditionValid,
		ObservedGeneration: cfg.Generation,
	}

	if errs := Validate(cfg); len(errs) > 0 {
		condition.Status = metav1.ConditionFalse
		condition.Reason = registryv1alpha1.ReasonInvalidSpec
		condition.Message = errs.ToAggregate().Error()
		log.Info("RegistryConfig is invalid", "errors", condition.Message)
	} else {
		condition.Status = metav1.ConditionTrue
		condition.Reason = registryv1alpha1.ReasonResolved
		condition.Message = "The configuration is coherent."
	}

	if err := r.patchStatus(ctx, cfg, condition); err != nil {
		return ctrl.Result{}, fmt.Errorf("patching status of RegistryConfig %s: %w", cfg.Name, err)
	}

	// No requeue on an invalid spec: nothing changes until someone edits the
	// object, and a retry loop would only add noise.
	return ctrl.Result{}, nil
}

func (r *Reconciler) patchStatus(
	ctx context.Context, cfg *registryv1alpha1.RegistryConfig, condition metav1.Condition,
) error {
	patch := client.MergeFrom(cfg.DeepCopy())

	generationChanged := cfg.Status.ObservedGeneration != cfg.Generation
	conditionChanged := apimeta.SetStatusCondition(&cfg.Status.Conditions, condition)
	cfg.Status.ObservedGeneration = cfg.Generation

	if !generationChanged && !conditionChanged {
		// Skip the write when nothing changed. Status churn on an object every
		// agent and syncer watches costs an etcd write and a cluster-wide wake-up.
		return nil
	}

	return r.Client.Status().Patch(ctx, cfg, patch)
}
