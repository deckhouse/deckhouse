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

// Package machinesetrevision caps the MCM MachineSet revision-history annotation.
//
// The machine-controller-manager records every rollout revision in the
// deployment.kubernetes.io/revision-history annotation as an ever-growing
// comma-separated list. This controller collapses it to the first revision once
// it exceeds a small length bound, so the annotation cannot grow without limit.
//
// This replaces the shell-operator hook hooks/trim_machine_set_revision_history.go.
// The hook watched MachineSet events; the controller reconciles a MachineSet
// reactively on its own changes, keeping identical trimming behavior.
package machinesetrevision

import (
	"context"
	"encoding/json"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/register"
)

const (
	revisionHistoryKey       = "deployment.kubernetes.io/revision-history"
	revisionHistoryMaxLength = 16
)

var machineSetGVK = schema.GroupVersionKind{
	Group: "machine.sapcloud.io", Version: "v1alpha1", Kind: "MachineSet",
}

func newMachineSet() *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(machineSetGVK)
	return u
}

func init() {
	register.RegisterController("node-machineset-revision-trim", newMachineSet(), &Reconciler{})
}

type Reconciler struct {
	register.Base
}

func (r *Reconciler) SetupWatches(register.Watcher) {}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// The hook scoped its MachineSet binding to d8-cloud-instance-manager; keep parity.
	if req.Namespace != nodecommon.MachineNamespace {
		return ctrl.Result{}, nil
	}

	machineSet := newMachineSet()
	if err := r.Client.Get(ctx, req.NamespacedName, machineSet); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	revisionHistory := machineSet.GetAnnotations()[revisionHistoryKey]
	if len(revisionHistory) <= revisionHistoryMaxLength {
		return ctrl.Result{}, nil
	}

	trimmed := trimRevisionHistory(revisionHistory)
	if trimmed == revisionHistory {
		return ctrl.Result{}, nil
	}

	if err := r.patchRevisionHistory(ctx, machineSet, trimmed); err != nil {
		log.FromContext(ctx).Error(err, "failed to trim MachineSet revision-history", "machineSet", req.NamespacedName)
		return ctrl.Result{}, err
	}

	log.FromContext(ctx).Info("trimmed MachineSet revision-history", "machineSet", req.NamespacedName)
	return ctrl.Result{}, nil
}

// trimRevisionHistory keeps only the first revision (before the first comma),
// mirroring the hook's strings.Cut on the comma.
func trimRevisionHistory(revisionHistory string) string {
	first, _, _ := strings.Cut(revisionHistory, ",")
	return first
}

// patchRevisionHistory rewrites the single annotation with a JSON merge patch,
// mirroring the hook's PatchWithMerge and leaving other annotations intact.
func (r *Reconciler) patchRevisionHistory(ctx context.Context, machineSet *unstructured.Unstructured, revisionHistory string) error {
	body := map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]string{
				revisionHistoryKey: revisionHistory,
			},
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	return r.Client.Patch(ctx, machineSet, client.RawPatch(types.MergePatchType, raw))
}
