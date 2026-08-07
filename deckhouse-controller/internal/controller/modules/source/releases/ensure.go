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

package releases

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"go.opentelemetry.io/otel"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	releaseUpdater "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/releaseupdater"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// ltsReleaseChannel is the channel whose consumers take one jump instead of walking every minor.
const ltsReleaseChannel = "lts"

// errModuleIsCorrupted marks a version whose metadata could not be read, which is reported as a
// different failure from a chain that merely has a hole in it.
var errModuleIsCorrupted = errors.New("module is corrupted")

// Ensure brings the module's ModuleRelease chain up to the request's target version, creating the
// intermediate releases the updater needs to step through. It is idempotent: releases that already
// exist are left alone, so a partially built chain is completed rather than rebuilt.
func (s *Service) Ensure(ctx context.Context, req Request) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "Ensure")
	defer span.End()

	e := &ensurer{
		Service: s,
		req:     req,
		logger: s.logger.With(
			slog.String("module_name", req.ModuleName),
			slog.String("source_name", req.Source.Name)),
	}

	return e.run(ctx)
}

// ensurer runs one module's fetch, holding the request beside the shared collaborators.
type ensurer struct {
	*Service

	req    Request
	logger *log.Logger
}

// run resolves where the chain currently ends and walks it up to the target.
func (e *ensurer) run(ctx context.Context) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "run")
	defer span.End()

	inCluster, err := e.list(ctx, e.req.ModuleName)
	if err != nil {
		return err
	}

	sort.Sort(sort.Reverse(releaseUpdater.ByVersion[*v1alpha1.ModuleRelease](inCluster)))

	// The walk starts at the newest deployed release. Without one, every release in the cluster is
	// still pending, so the walk starts at the oldest of them - the last element, the list being
	// sorted newest first.
	var startFrom *v1alpha1.ModuleRelease
	chain := make([]*v1alpha1.ModuleRelease, 0, len(inCluster))

	if idx, deployed := latestDeployedAt(inCluster); idx != -1 {
		chain = inCluster[:idx+1]
		startFrom = deployed
	} else if len(inCluster) > 0 {
		chain = inCluster
		startFrom = inCluster[len(inCluster)-1]
	}

	target, err := semver.NewVersion(e.req.Target.Version)
	if err != nil {
		// TODO: maybe set something like v1.0.0-{meta.Version} for developing purpose
		return fmt.Errorf("parse target version %q: %w", e.req.Target.Version, err)
	}

	if target.Prerelease() != "" {
		return fmt.Errorf("pre-release versions are not supported: %s", target.Original())
	}

	// The channel metadata may carry a partial definition, so the target's own release image is the
	// authority on its update constraints - which the walk below depends on.
	targetMeta, err := e.loader.ReleaseMetadata(ctx, e.req.ModuleName, e.req.Target.Version)
	if err != nil {
		return fmt.Errorf("load target release metadata: %w", err)
	}

	e.req.Target.Definition = targetMeta.Definition

	sort.Sort(releaseUpdater.ByVersion[*v1alpha1.ModuleRelease](chain))

	e.logger.Debug("start ensure releases",
		slog.Bool("deployed_release_found", startFrom != nil),
		slog.String("module_version", target.String()))

	if err = e.ensure(ctx, startFrom, chain, target); err != nil {
		return fmt.Errorf("ensure releases: %w", err)
	}

	return nil
}

// ensure creates the ModuleReleases between the chain's end and the channel's target.
//
// Flow:
//  1. nothing in the cluster - create the release from the channel;
//  2. an LTS channel - create the release from the channel, no step-by-step walk;
//  3. otherwise walk step by step:
//     3.1 start from the deployed release, or from the end of the chain when every release in the
//     cluster is already sequential;
//     3.2 take every new version between the start and the channel's target, including the highest
//     patch of the current minor so no migration is skipped;
//     3.3 nothing new means the start is already at or past the target - no-op;
//     3.4 create the releases in order; a gap the from-to rules do not bridge is recorded and the
//     walk continues, so one hole does not hide the rest.
func (e *ensurer) ensure(
	ctx context.Context,
	startFrom *v1alpha1.ModuleRelease,
	chain []*v1alpha1.ModuleRelease,
	target *semver.Version,
) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "ensure")
	defer span.End()

	metricLabels := map[string]string{
		metrics.LabelModule:   e.req.ModuleName,
		metrics.LabelVersion:  e.req.Target.Version,
		metrics.LabelRegistry: e.req.Source.Spec.Registry.Repo,
	}

	if len(chain) == 0 {
		e.logger.Debug("no release in cluster")

		if err := e.ensureRelease(ctx, e.req.Target, "no releases in cluster"); err != nil {
			return fmt.Errorf("create release %s: %w", e.req.Target.Version, err)
		}

		return nil
	}

	if strings.EqualFold(e.req.ReleaseChannel, ltsReleaseChannel) {
		e.logger.Debug("lts channel, create the release without intermediate versions",
			slog.String("channel", e.req.ReleaseChannel))

		if err := e.ensureRelease(ctx, e.req.Target, "LTS channel - direct release"); err != nil {
			return fmt.Errorf("create LTS release %s: %w", e.req.Target.Version, err)
		}

		return nil
	}

	// A chain that is already sequential end to end can be walked from its newest release; one with
	// a hole has to be walked from the deployed release, so the hole is filled rather than skipped.
	actual := startFrom
	if sequential(chain) {
		actual = chain[len(chain)-1]
	}

	metricLabels[metrics.LabelActualVersion] = "v" + actual.GetVersion().String()

	versions, err := e.newVersions(ctx, actual.GetVersion(), target)
	if err != nil {
		return fmt.Errorf("get new versions: %w", err)
	}

	// nothing new means actual is already at or past the target
	if len(versions) == 0 {
		return nil
	}

	var walkErr error

	current := actual.GetVersion()
	for _, version := range versions {
		if err = e.step(ctx, current, version); err != nil {
			walkErr = errors.Join(walkErr, err)

			metricLabels[metrics.LabelVersion] = "v" + version.String()

			metric := metrics.D8ModuleUpdatingBrokenSequence
			if errors.Is(err, errModuleIsCorrupted) {
				metric = metrics.D8ModuleUpdatingModuleIsNotValid
			}

			e.metricStorage.Grouped().GaugeSet(e.req.MetricGroup, metric, 1, metricLabels)
		}

		current = version
	}

	if walkErr != nil {
		e.logger.Error("step by step update failed", log.Err(walkErr))

		return fmt.Errorf("step by step update failed: %w", walkErr)
	}

	return nil
}

// step creates the release for one version of the walk and checks that it may follow the previous
// one, either naturally or through a from-to rule declared on the new release or on the target.
func (e *ensurer) step(ctx context.Context, current, version *semver.Version) error {
	e.logger.Debug("ensure module release", slog.String("version", version.String()))

	tag := "v" + version.String()

	meta, err := e.loader.ReleaseMetadata(ctx, e.req.ModuleName, tag)
	if err != nil {
		e.logger.Error("failed to load the release metadata",
			slog.String("module_version", tag), log.Err(err))

		return fmt.Errorf("load release metadata %s: %w: %w", tag, err, errModuleIsCorrupted)
	}

	if err = e.ensureRelease(ctx, meta, "step-by-step"); err != nil {
		e.logger.Error("failed to ensure the module release",
			slog.String("module_version", tag), log.Err(err))

		return fmt.Errorf("ensure module release %s: %w", tag, err)
	}

	// a from-to rule on the new release governs how it may be reached
	if hasFromTo(meta) {
		if err = admittedByFromTo(current, meta.Definition.Update.Versions); err != nil {
			return fmt.Errorf("from-to check from ensured module: not sequential version: %w", err)
		}

		return nil
	}

	if isUpdatingSequence(current, version) {
		return nil
	}

	// not adjacent - the target's own from-to rules are the last thing that can bridge it
	if !hasFromTo(e.req.Target) {
		e.logger.Warn("version sequence is broken",
			slog.String("previous", "v"+current.String()), slog.String("next", tag))

		return fmt.Errorf("not sequential version: prev 'v%s' next '%s'", current.String(), tag)
	}

	if err = admittedByFromTo(current, e.req.Target.Definition.Update.Versions); err != nil {
		e.logger.Warn("from-to check from target module: not sequential version",
			slog.String("previous", "v"+current.String()), log.Err(err))

		return fmt.Errorf("from-to check from target module: not sequential version: prev 'v%s' next '%s': %w",
			current.String(), tag, err)
	}

	e.logger.Info("from-to check from target module: version is in sequence")

	return nil
}

// ensureRelease creates the ModuleRelease for the metadata, or refreshes a still-pending one in
// place. createProcess records what caused the creation and reaches the release's change-cause
// annotation.
func (e *ensurer) ensureRelease(ctx context.Context, meta *Metadata, createProcess string) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "ensureRelease")
	defer span.End()

	changeCause := "check release"
	if createProcess != "" {
		changeCause += " (" + createProcess + ")"
	}

	name := fmt.Sprintf("%s-%s", e.req.ModuleName, meta.Version)

	release := new(v1alpha1.ModuleRelease)
	err := e.client.Get(ctx, client.ObjectKey{Name: name}, release)
	if err == nil {
		return e.refreshRelease(ctx, release, meta, changeCause)
	}

	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("get module release '%s': %w", name, err)
	}

	release = &v1alpha1.ModuleRelease{
		TypeMeta: metav1.TypeMeta{
			Kind:       v1alpha1.ModuleReleaseGVK.Kind,
			APIVersion: v1alpha1.ModuleReleaseGVK.GroupVersion().String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				v1alpha1.ModuleReleaseAnnotationChangeCause: changeCause,
			},
			Labels: map[string]string{
				v1alpha1.ModuleReleaseLabelModule: e.req.ModuleName,
				v1alpha1.ModuleReleaseLabelSource: e.req.Source.GetName(),
				// the digest is 64 characters and a label value caps at 63, so it is hashed
				v1alpha1.ModuleReleaseLabelReleaseChecksum: fmt.Sprintf("%x", md5.Sum([]byte(meta.Checksum))),
				v1alpha1.ModuleReleaseLabelUpdatePolicy:    e.req.UpdatePolicy,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: v1alpha1.ModuleSourceGVK.GroupVersion().String(),
					Kind:       v1alpha1.ModuleSourceGVK.Kind,
					Name:       e.req.Source.GetName(),
					UID:        e.req.Source.GetUID(),
					Controller: ptr.To(true),
				},
			},
		},
		Spec: releaseSpec(e.req.ModuleName, meta),
	}

	if meta.Definition != nil && meta.Definition.Requirements != nil {
		release.Spec.Requirements = &v1alpha1.ModuleReleaseRequirements{
			ModuleReleasePlatformRequirements: v1alpha1.ModuleReleasePlatformRequirements{
				Deckhouse:  meta.Definition.Requirements.Deckhouse,
				Kubernetes: meta.Definition.Requirements.Kubernetes,
			},
			ParentModules: meta.Definition.Requirements.ParentModules,
		}
	}

	// A module's very first release must land without waiting for an update window or a manual
	// approval, so it is marked to apply now when no other release for the module exists.
	first, err := e.isFirstRelease(ctx)
	if err != nil {
		return err
	}

	if first {
		release.Annotations[v1alpha1.ModuleReleaseAnnotationApplyNow] = "true"
	}

	if err = e.client.Create(ctx, release); err != nil {
		return fmt.Errorf("create module release '%s': %w", name, err)
	}

	return nil
}

// refreshRelease re-applies the metadata onto a release that has not deployed yet. A deployed or
// suspended release is left alone: rewriting its spec would move a version already in use.
func (e *ensurer) refreshRelease(ctx context.Context, release *v1alpha1.ModuleRelease, meta *Metadata, changeCause string) error {
	if release.Status.Phase != v1alpha1.ModuleReleasePhasePending {
		return nil
	}

	patch := client.MergeFrom(release.DeepCopy())

	if release.Annotations == nil {
		release.Annotations = make(map[string]string, 1)
	}

	release.Annotations[v1alpha1.ModuleReleaseAnnotationChangeCause] = changeCause
	release.Spec = releaseSpec(e.req.ModuleName, meta)

	if err := e.client.Patch(ctx, release, patch); err != nil {
		return fmt.Errorf("patch module release '%s': %w", release.Name, err)
	}

	return nil
}

// isFirstRelease reports whether the module has no ModuleRelease at all yet.
func (e *ensurer) isFirstRelease(ctx context.Context) (bool, error) {
	releases := new(v1alpha1.ModuleReleaseList)
	if err := e.client.List(ctx, releases,
		client.MatchingLabels{v1alpha1.ModuleReleaseLabelModule: e.req.ModuleName},
		client.Limit(1)); err != nil {
		return false, fmt.Errorf("list the '%s' module releases: %w", e.req.ModuleName, err)
	}

	return len(releases.Items) == 0, nil
}

// releaseSpec builds the spec a release carries for one version's metadata.
func releaseSpec(moduleName string, meta *Metadata) v1alpha1.ModuleReleaseSpec {
	spec := v1alpha1.ModuleReleaseSpec{
		ModuleName: moduleName,
		Version:    semver.MustParse(meta.Version).String(),
		Changelog:  v1alpha1.MakeMappedFields(meta.Changelog),
	}

	if meta.Definition != nil {
		spec.Weight = meta.Definition.Weight
	}

	if hasFromTo(meta) {
		spec.UpdateSpec = meta.Definition.Update.ToV1Alpha1()
	}

	return spec
}

// hasFromTo reports whether the version declares any from-to transition rule.
func hasFromTo(meta *Metadata) bool {
	return meta != nil && meta.Definition != nil &&
		meta.Definition.Update != nil && len(meta.Definition.Update.Versions) > 0
}

// sequential reports whether every neighbouring pair in the chain may follow the previous one.
func sequential(chain []*v1alpha1.ModuleRelease) bool {
	for i := 1; i < len(chain); i++ {
		if !isSequentialPair(chain[i-1], chain[i]) {
			return false
		}
	}

	return true
}

// latestDeployedAt returns the newest deployed release and its index in a newest-first list, or -1
// and nil when the module has never deployed one.
func latestDeployedAt(releases []*v1alpha1.ModuleRelease) (int, *v1alpha1.ModuleRelease) {
	for idx, release := range releases {
		if release.GetPhase() == v1alpha1.ModuleReleasePhaseDeployed {
			return idx, release
		}
	}

	return -1, nil
}
