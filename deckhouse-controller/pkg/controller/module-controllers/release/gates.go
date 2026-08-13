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
	"os"
	"path/filepath"

	"github.com/Masterminds/semver/v3"
	"github.com/deckhouse/d8sql"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/ctrlutils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/releasegates"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// runReleaseGates runs the SQL gates shipped with the downloaded release: first
// the validations, then the migrations. It is called between download and
// install, so a rejected release never reaches the filesystem and never becomes
// Deployed.
//
// A failing gate leaves the release Pending with the failure in its status (and
// in the module condition), so the next reconcile retries it. Migrations are
// expected to be idempotent - a retry replays the failed one.
func (r *reconciler) runReleaseGates(ctx context.Context, release *v1alpha1.ModuleRelease, modulePath string, logger *log.Logger) error {
	if err := r.runReleaseValidations(ctx, release, modulePath, logger); err != nil {
		return err
	}

	return r.runReleaseMigrations(ctx, release, modulePath, logger)
}

func (r *reconciler) runReleaseValidations(ctx context.Context, release *v1alpha1.ModuleRelease, modulePath string, logger *log.Logger) error {
	files, err := releasegates.Validations(modulePath)
	if err != nil {
		return fmt.Errorf("list validations: %w", err)
	}

	if len(files) == 0 {
		return nil
	}

	engine, err := r.sqlGateEngine()
	if err != nil {
		return fmt.Errorf("sql engine: %w", err)
	}

	for _, file := range files {
		if _, err = releasegates.Run(ctx, engine, file); err != nil {
			logger.Warn("release validation failed", slog.String("file", filepath.Base(file)), log.Err(err))

			return r.failReleaseGate(ctx, release, "validations failed: "+err.Error(),
				"ReleaseValidationsCheck", "ModuleRelease could not be applied, release validations failed", err)
		}

		logger.Debug("release validation passed", slog.String("file", filepath.Base(file)))
	}

	return nil
}

func (r *reconciler) runReleaseMigrations(ctx context.Context, release *v1alpha1.ModuleRelease, modulePath string, logger *log.Logger) error {
	candidate, err := releasegates.Migrations(modulePath)
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}

	// the applied migrations travel with the deployed release, so they are read
	// from it and carried over into the release being deployed
	journal, deployedVersion, err := r.deployedMigrations(ctx, release)
	if err != nil {
		return fmt.Errorf("get deployed migrations: %w", err)
	}

	pending := releasegates.PendingUp(candidate, appliedVersion(journal))

	// a downgrade (only reachable through the force annotation) rolls back what
	// the newer, still installed version applied, so the down files are read
	// from the installed version, not from the release being deployed
	if deployedVersion != nil && release.GetVersion().LessThan(deployedVersion) {
		installed, err := releasegates.Migrations(r.installedModulePath(release.GetModuleName(), deployedVersion))
		if err != nil {
			return fmt.Errorf("list installed migrations: %w", err)
		}

		pending = releasegates.PendingDown(installed, releasegates.MaxVersion(candidate))
	}

	if len(pending) == 0 {
		return r.updateReleaseMigrations(ctx, release, journal)
	}

	engine, err := r.sqlGateEngine()
	if err != nil {
		return fmt.Errorf("sql engine: %w", err)
	}

	for _, migration := range pending {
		entry := v1alpha1.ModuleReleaseMigration{
			Name:               migration.Name,
			Version:            migration.Version,
			Direction:          string(migration.Direction),
			Status:             v1alpha1.ModuleReleaseMigrationSucceeded,
			LastTransitionTime: metav1.NewTime(r.dependencyContainer.GetClock().Now().UTC()),
		}

		changed, runErr := releasegates.Run(ctx, engine, migration.Path)
		if runErr != nil {
			entry.Status = v1alpha1.ModuleReleaseMigrationFailed
			entry.Message = runErr.Error()
		}

		entry.Affected = changed

		journal = append(journal, entry)

		if runErr != nil {
			logger.Warn("release migration failed", slog.String("migration", migration.Name), log.Err(runErr))

			if err = r.updateReleaseMigrations(ctx, release, journal); err != nil {
				return err
			}

			return r.failReleaseGate(ctx, release, "migrations failed: "+runErr.Error(),
				"ReleaseMigrationsCheck", "ModuleRelease could not be applied, release migrations failed", runErr)
		}

		logger.Info("release migration applied", slog.String("migration", migration.Name), slog.Int("affected", changed))
	}

	return r.updateReleaseMigrations(ctx, release, journal)
}

// deployedMigrations returns the migration journal and the version of the
// currently deployed release of the same module, if there is one.
func (r *reconciler) deployedMigrations(ctx context.Context, release *v1alpha1.ModuleRelease) ([]v1alpha1.ModuleReleaseMigration, *semver.Version, error) {
	releases := new(v1alpha1.ModuleReleaseList)
	if err := r.client.List(ctx, releases, client.MatchingLabels{v1alpha1.ModuleReleaseLabelModule: release.GetModuleName()}); err != nil {
		return nil, nil, fmt.Errorf("list module releases: %w", err)
	}

	for _, deployed := range releases.Items {
		if deployed.GetName() == release.GetName() || deployed.Status.Phase != v1alpha1.ModuleReleasePhaseDeployed {
			continue
		}

		return deployed.Status.Migrations, deployed.GetVersion(), nil
	}

	return nil, nil, nil
}

// installedModulePath returns the directory of the module version currently
// unpacked on the filesystem, falling back to the active symlink.
func (r *reconciler) installedModulePath(moduleName string, version *semver.Version) string {
	path := filepath.Join(r.downloadedModulesDir, moduleName, "v"+version.String())
	if _, err := os.Stat(path); err != nil {
		return filepath.Join(r.symlinksDir, moduleName)
	}

	return path
}

// appliedVersion is the highest migration version currently applied: for every
// version the last recorded successful run wins, so a successful Down cancels
// the Up recorded before it.
func appliedVersion(journal []v1alpha1.ModuleReleaseMigration) int {
	state := make(map[int]string, len(journal))
	for _, entry := range journal {
		if entry.Status != v1alpha1.ModuleReleaseMigrationSucceeded {
			continue
		}

		state[entry.Version] = entry.Direction
	}

	applied := 0
	for version, direction := range state {
		if direction == string(releasegates.DirectionUp) && version > applied {
			applied = version
		}
	}

	return applied
}

func (r *reconciler) updateReleaseMigrations(ctx context.Context, release *v1alpha1.ModuleRelease, journal []v1alpha1.ModuleReleaseMigration) error {
	if len(journal) == 0 {
		return nil
	}

	if err := ctrlutils.UpdateStatusWithRetry(ctx, r.client, release, func() error {
		release.Status.Migrations = journal

		return nil
	}); err != nil {
		return fmt.Errorf("update migrations status: %w", err)
	}

	return nil
}

// failReleaseGate records a gate failure on the release and on the module and
// returns the error aborting the deploy. The release is kept Pending on purpose:
// the gate is retried on the next reconcile.
func (r *reconciler) failReleaseGate(ctx context.Context, release *v1alpha1.ModuleRelease, message, reason, condMessage string, cause error) error {
	if err := r.updateReleaseStatus(ctx, release, &v1alpha1.ModuleReleaseStatus{
		Phase:   v1alpha1.ModuleReleasePhasePending,
		Message: message,
	}); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	if err := r.updateModuleLastReleaseDeployedStatus(ctx, release, condMessage, reason, false); err != nil {
		return fmt.Errorf("update module last release deployed status: %w", err)
	}

	return fmt.Errorf("the '%s:v%s' module release gates: %w", release.GetModuleName(), release.GetVersion().String(), cause)
}

// sqlGateEngine lazily builds the engine used by the release gates. It is built
// once per controller: resource resolution and discovery are cached on it.
func (r *reconciler) sqlGateEngine() (*d8sql.Engine, error) {
	r.sqlEngineOnce.Do(func() {
		// already injected (tests)
		if r.sqlEngine != nil {
			return
		}

		k8sClient, err := r.dependencyContainer.GetK8sClient()
		if err != nil {
			r.sqlEngineErr = fmt.Errorf("get k8s client: %w", err)

			return
		}

		platform := releasegates.Platform{}
		if r.edition != nil {
			platform.DeckhouseVersion = r.edition.Version
			platform.DeckhouseEdition = r.edition.Name
			platform.DeckhouseBundle = r.edition.Bundle
		}

		if version, err := k8sClient.Discovery().ServerVersion(); err != nil {
			r.log.Warn("failed to discover the kubernetes version for release gates", log.Err(err))
		} else {
			platform.KubernetesVersion = releasegates.NormalizeVersion(version.GitVersion)
		}

		r.sqlEngine = d8sql.New(k8sClient.Dynamic(), r.client.RESTMapper(), platform.Option())
	})

	return r.sqlEngine, r.sqlEngineErr
}
