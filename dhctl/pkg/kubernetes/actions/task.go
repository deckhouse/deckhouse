// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package actions

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/kubeerrors"
)

type ManifestTask struct {
	Name       string
	CreateFunc func(ctx context.Context, manifest any) error
	UpdateFunc func(ctx context.Context, manifest any) error
	Manifest   func() any
	PatchData  func() any
	PatchFunc  func(ctx context.Context, patchData []byte) error
}

// ErrManifestTaskTransient marks a create/update/patch failure that may succeed on retry
// (e.g. a resource-version conflict or a transient API error), as opposed to a permanent
// authorization or admission-webhook rejection that fails identically on every attempt.
var ErrManifestTaskTransient = fmt.Errorf("manifest task: transient error, may succeed on retry")

// ErrManifestTaskPermanent lets a CreateFunc/UpdateFunc/PatchFunc wrap its own error to mark
// it as a deterministic business-rule violation that will never succeed on retry (e.g. a
// cluster-ownership conflict), even when it isn't a Kubernetes API status error that
// wrapManifestErr would otherwise recognize as permanent.
var ErrManifestTaskPermanent = fmt.Errorf("manifest task: permanent error, will not succeed on retry")

// wrapManifestErr tags err as transient unless it is a permanent authorization failure (or
// explicitly marked via ErrManifestTaskPermanent), so callers can whitelist
// ErrManifestTaskTransient in their retry loop.
//
// Deliberately NOT included: apierrors.IsInvalid. An admission-webhook rejection often checks
// cross-resource state (e.g. "has the ModuleSource created earlier in this same batch been
// reconciled yet?"), not just the manifest's own static content — that's exactly the kind of
// propagation delay these loops exist to ride out, so treating every Invalid as permanent risks
// aborting an operation that would have succeeded a few seconds later.
func wrapManifestErr(ctx context.Context, prefix string, err error) error {
	if kubeerrors.IsPermanentAuthError(ctx, err) || stderrors.Is(err, ErrManifestTaskPermanent) {
		return fmt.Errorf("%s: %w", prefix, err)
	}
	return fmt.Errorf("%w: %s: %w", ErrManifestTaskTransient, prefix, err)
}

// CreateOrUpdate tries to create resource with the CreateFunc. If resource is already
// exists, it updates the resource with the UpdateFunc.
func (task *ManifestTask) CreateOrUpdate(ctx context.Context) error {
	dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf("Manifest for %s", task.Name))
	manifest := task.Manifest()

	createErr := task.CreateFunc(ctx, manifest)
	if createErr == nil {
		return nil
	}
	if !createMayMeanExists(createErr) {
		return wrapManifestErr(ctx, "create resource", createErr)
	}

	dhlog.FromContext(ctx).InfoContext(ctx, strings.TrimRight(fmt.Sprintf("%s already exists. Trying to update ... ", task.Name), "\n"))
	if err := task.UpdateFunc(ctx, manifest); err != nil {
		dhlog.FromContext(ctx).ErrorContext(ctx, "ERROR!")
		return wrapManifestErr(ctx, updateFailurePrefix(createErr), updateFailure(createErr, err))
	}
	dhlog.FromContext(ctx).InfoContext(ctx, "OK!")
	return nil
}

func (task *ManifestTask) CreateOrUpdateSilent(ctx context.Context) error {
	dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Manifest for %s", task.Name))
	manifest := task.Manifest()

	createErr := task.CreateFunc(ctx, manifest)
	if createErr == nil {
		return nil
	}
	if !createMayMeanExists(createErr) {
		return wrapManifestErr(ctx, "create resource", createErr)
	}

	dhlog.FromContext(ctx).DebugContext(ctx, strings.TrimRight(fmt.Sprintf("%s already exists. Trying to update ... ", task.Name), "\n"))
	if err := task.UpdateFunc(ctx, manifest); err != nil {
		dhlog.FromContext(ctx).ErrorContext(ctx, "ERROR!")
		return wrapManifestErr(ctx, updateFailurePrefix(createErr), updateFailure(createErr, err))
	}
	dhlog.FromContext(ctx).DebugContext(ctx, "OK!")
	return nil
}

func (task *ManifestTask) Patch(ctx context.Context) error {
	dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Patch for %s", task.Name))
	patchData := task.PatchData()

	patchBytes, err := json.Marshal(patchData)
	if err != nil {
		return fmt.Errorf("marshal patch data: %v", err)
	}

	err = task.PatchFunc(ctx, patchBytes)
	if err != nil {
		return wrapManifestErr(ctx, "Apply patch", err)
	}

	return nil
}

func (task *ManifestTask) PatchOrCreate(ctx context.Context) error {
	dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Patch or create for %s", task.Name))
	patchData := task.PatchData()

	patchBytes, err := json.Marshal(patchData)
	if err != nil {
		return fmt.Errorf("marshal patch data: %v", err)
	}

	err = task.PatchFunc(ctx, patchBytes)
	if err == nil {
		return nil
	}

	if !errors.IsNotFound(err) {
		return wrapManifestErr(ctx, fmt.Sprintf("Apply patch for '%s'", task.Name), err)
	}

	dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("%s is not found. Trying to create ... ", task.Name))
	manifest := task.Manifest()
	err = task.CreateFunc(ctx, manifest)
	if err != nil {
		return wrapManifestErr(ctx, fmt.Sprintf("Create '%s'", task.Name), err)
	}
	return nil
}

type ModuleConfigTask struct {
	// task without attempts inside, client must retry all tasks by itself
	Do    func(kubeCl *client.KubernetesClient) error
	Title string
	Name  string
}

// createMayMeanExists reports whether a failed create leaves the object's
// existence open. AlreadyExists says it outright; Forbidden says it whenever an
// admission policy answers before the API server can report the conflict, which
// is what Deckhouse's own system-ns.deckhouse.io does to a CREATE of a d8-*
// namespace on every re-run. Either way the update below settles it.
func createMayMeanExists(err error) bool {
	return errors.IsAlreadyExists(err) || errors.IsForbidden(err)
}

// updateFailure picks the error to report when the update after a failed create
// also fails. A create refused as AlreadyExists proves the object is there, so
// the update's own failure is the news; a create refused as Forbidden proves
// nothing, and its refusal is the one the operator has to act on.
func updateFailure(createErr, updateErr error) error {
	if errors.IsAlreadyExists(createErr) {
		return updateErr
	}
	return createErr
}

func updateFailurePrefix(createErr error) string {
	if errors.IsAlreadyExists(createErr) {
		return "update resource"
	}
	return "create resource"
}
