// Copyright 2026 Flant JSC
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

package release

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/modules"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/ctrlutils"
	releaseUpdater "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/releaseupdater"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// dryRunTimeout bounds the detached dry-run pass, whose parent context may already be gone.
const dryRunTimeout = time.Minute

// applyRelease deploys the release. A nil task means there is no release to supersede.
func (r *reconciler) applyRelease(ctx context.Context, mr *v1alpha1.ModuleRelease, task *releaseUpdater.Task) error {
	ctx, span := otel.Tracer(controllerName).Start(ctx, "applyRelease")
	defer span.End()

	var deployedReleaseInfo *releaseUpdater.ReleaseInfo
	if task != nil {
		deployedReleaseInfo = task.DeployedReleaseInfo
	}

	if err := r.runReleaseDeploy(ctx, mr, deployedReleaseInfo); err != nil {
		return fmt.Errorf("run release deploy: %w", err)
	}

	return nil
}

// runReleaseDeploy hands the version to the runtime, supersedes the release it replaces and
// marks this one Deployed. Metadata and status are written separately, each with its own
// conflict retry.
func (r *reconciler) runReleaseDeploy(ctx context.Context, mr *v1alpha1.ModuleRelease, deployedReleaseInfo *releaseUpdater.ReleaseInfo) error {
	ctx, span := otel.Tracer(controllerName).Start(ctx, "runReleaseDeploy")
	defer span.End()

	r.logger.Info("applying release", slog.String("release", mr.GetName()))

	if err := r.deployModule(ctx, mr); err != nil {
		return fmt.Errorf("deploy module: %w", err)
	}

	if deployedReleaseInfo != nil {
		err := r.updateReleaseStatus(ctx, newModuleReleaseWithName(deployedReleaseInfo.Name), &v1alpha1.ModuleReleaseStatus{
			Phase:   v1alpha1.ModuleReleasePhaseSuperseded,
			Message: "",
		})
		if err != nil {
			r.logger.Error("update status", slog.String("release", deployedReleaseInfo.Name), log.Err(err))
		}
	}

	err := ctrlutils.UpdateWithRetry(ctx, r.client, mr, func() error {
		if len(mr.Annotations) == 0 {
			mr.Annotations = make(map[string]string, 2)
		}

		mr.Annotations[v1alpha1.ModuleReleaseAnnotationIsUpdating] = "true"
		mr.Annotations[v1alpha1.ModuleReleaseAnnotationNotified] = "true"

		if len(mr.ObjectMeta.Labels) == 0 {
			mr.ObjectMeta.Labels = make(map[string]string, 1)
		}

		mr.ObjectMeta.Labels[v1alpha1.ModuleReleaseLabelStatus] = v1alpha1.ModuleReleaseLabelDeployed

		// the one-shot annotations that asked for this deploy are spent
		if mr.GetApplyNow() {
			delete(mr.Annotations, v1alpha1.ModuleReleaseAnnotationApplyNow)
		}

		if mr.GetForce() {
			delete(mr.Annotations, v1alpha1.ModuleReleaseAnnotationForce)
		}

		if mr.GetReinstall() {
			delete(mr.Annotations, v1alpha1.ModuleReleaseAnnotationReinstall)
		}

		controllerutil.AddFinalizer(mr, v1alpha1.ModuleReleaseFinalizerExistOnFs)

		return nil
	}, ctrlutils.WithRetryOnConflictBackoff(retryBackoff()))
	if err != nil {
		return fmt.Errorf("update with retry: %w", err)
	}

	err = ctrlutils.UpdateStatusWithRetry(ctx, r.client, mr, func() error {
		mr.Status.Phase = v1alpha1.ModuleReleasePhaseDeployed
		mr.Status.Message = ""

		return nil
	}, ctrlutils.WithRetryOnConflictBackoff(retryBackoff()))
	if err != nil {
		return fmt.Errorf("update status with retry: %w", err)
	}

	return nil
}

// deployModule hands the release's version to the package runtime, which pulls the image, places
// it on disk, parses the definition and reloads the module. Settings are not passed: they belong
// to the module config controller, which pushes them separately.
func (r *reconciler) deployModule(ctx context.Context, mr *v1alpha1.ModuleRelease) error {
	ctx, span := otel.Tracer(controllerName).Start(ctx, "deployModule")
	defer span.End()

	// dryrun for testing purpose
	if mr.GetDryRun() {
		go r.runDryRunDeploy(mr)

		return nil
	}

	source := new(v1alpha1.ModuleSource)
	if err := r.client.Get(ctx, client.ObjectKey{Name: mr.GetModuleSource()}, source); err != nil {
		return fmt.Errorf("get the '%s' module source: %w", mr.GetModuleSource(), err)
	}

	// Not forced: a release pins an immutable version tag, so a copy the runtime already has of
	// that version is the right one.
	r.updateModule(source, mr, false)

	return nil
}

// updateModule registers the release's version with the runtime. force makes the runtime redeploy
// a version it already tracks, which is needed when only the registry behind it changed.
func (r *reconciler) updateModule(source *v1alpha1.ModuleSource, mr *v1alpha1.ModuleRelease, force bool) {
	r.logger.Debug("update module in the package runtime",
		slog.String("module", mr.GetModuleName()),
		slog.String("version", mr.GetModuleVersion()),
		slog.Bool("force", force))

	r.manager.UpdateModule(registry.BuildRemote(source), runtime.Module{
		Name: mr.GetModuleName(),
		Definition: modules.Definition{
			Version: mr.GetModuleVersion(),
		},
	}, force)
}

// runDryRunDeploy nudges the module's other pending releases so they requeue and observe the
// dry-run outcome. It runs detached from the reconcile, on its own bounded context.
func (r *reconciler) runDryRunDeploy(mr *v1alpha1.ModuleRelease) {
	r.logger.Debug("dryrun start soon...")

	time.Sleep(3 * time.Second)

	r.logger.Debug("dryrun started")

	ctx, cancel := context.WithTimeout(context.Background(), dryRunTimeout)
	defer cancel()

	releases := new(v1alpha1.ModuleReleaseList)
	if err := r.client.List(ctx, releases, client.MatchingLabels{v1alpha1.ModuleReleaseLabelModule: mr.GetModuleName()}); err != nil {
		r.logger.Error("dryrun list module releases", slog.String("module_name", mr.GetModuleName()), log.Err(err))

		return
	}

	for i := range releases.Items {
		pending := &releases.Items[i]

		if pending.GetName() == mr.GetName() || pending.Status.Phase != v1alpha1.ModuleReleasePhasePending {
			continue
		}

		// update releases to trigger their requeue
		err := ctrlutils.UpdateWithRetry(ctx, r.client, pending, func() error {
			if len(pending.Annotations) == 0 {
				pending.Annotations = make(map[string]string, 1)
			}

			pending.Annotations[v1alpha1.ModuleReleaseAnnotationTriggeredByDryrun] = mr.GetName()

			return nil
		})
		if err != nil {
			r.logger.Error("dryrun update release to requeue", log.Err(err))

			continue
		}

		r.logger.Debug("dryrun release successfully updated", slog.String("release", pending.Name))
	}
}
