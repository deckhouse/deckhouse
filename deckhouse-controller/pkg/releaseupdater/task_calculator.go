/*
Copyright 2024 Flant JSC

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

package releaseupdater

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"go.opentelemetry.io/otel"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	taskCalculatorServiceName = "task-calculator"
	maxMinorVersionDiffForLTS = 10
)

type TaskCalculator struct {
	k8sclient client.Client

	listFunc func(ctx context.Context, c client.Client, moduleName string) ([]v1alpha1.Release, error)

	log *log.Logger

	releaseChannel string
}

func NewDeckhouseReleaseTaskCalculator(k8sclient client.Client, logger *log.Logger, releaseChannel string) *TaskCalculator {
	return &TaskCalculator{
		k8sclient:      k8sclient,
		listFunc:       listDeckhouseReleases,
		log:            logger,
		releaseChannel: releaseChannel,
	}
}

func NewModuleReleaseTaskCalculator(k8sclient client.Client, logger *log.Logger) *TaskCalculator {
	return &TaskCalculator{
		k8sclient:      k8sclient,
		listFunc:       listModuleReleases,
		log:            logger,
		releaseChannel: "",
	}
}

type TaskType int

const (
	Skip TaskType = iota
	Await
	Process
)

type Task struct {
	TaskType TaskType
	Message  string

	IsPatch  bool
	IsSingle bool
	IsLatest bool

	DeployedReleaseInfo *ReleaseInfo
	QueueDepth          int
}

type ReleaseInfo struct {
	Name    string
	Version *semver.Version
}

// inner structure to save inner logic
type releaseInfo struct {
	IndexInReleaseList int
	Name               string
	Version            *semver.Version
}

func (ri *releaseInfo) RemapToReleaseInfo() *ReleaseInfo {
	if ri == nil {
		return nil
	}

	return &ReleaseInfo{
		Name:    ri.Name,
		Version: ri.Version,
	}
}

var ErrReleasePhaseIsNotPending = errors.New("release phase is not pending")
var ErrReleaseIsAlreadyDeployed = errors.New("release is already deployed")

// isPatchRelease returns true if b is greater only in terms of the patch versions.
func isPatchRelease(a, b *semver.Version) bool {
	if b.Major() == a.Major() && b.Minor() == a.Minor() && b.Patch() > a.Patch() {
		return true
	}

	return false
}

const ltsReleaseChannel = "lts"

// CalculatePendingReleaseTask calculate task with information about current reconcile
//
// calculating flow:
// 1) find forced release. if current release has a lower version - skip
// 2) find deployed release. if current release has a lower version - skip
func (p *TaskCalculator) CalculatePendingReleaseTask(ctx context.Context, release v1alpha1.Release) (*Task, error) {
	ctx, span := otel.Tracer(taskCalculatorServiceName).Start(ctx, "calculatePendingReleaseTask")
	defer span.End()

	logger := p.log.With(slog.String("release_name", release.GetName()))

	if release.GetPhase() != v1alpha1.DeckhouseReleasePhasePending {
		return nil, ErrReleasePhaseIsNotPending
	}

	releases, err := p.listReleases(ctx, release.GetModuleName())
	if err != nil {
		return nil, fmt.Errorf("list releases: %w", err)
	}

	if len(releases) == 1 {
		return &Task{
			TaskType: Process,
			IsSingle: true,
			IsLatest: true,
		}, nil
	}

	sort.Sort(ByVersion[v1alpha1.Release](releases))

	forcedReleaseInfo := getLatestForcedReleaseInfo(releases)

	// if we have a forced release
	if forcedReleaseInfo != nil {
		logger = logger.With(logger.WithGroup("forced_release").With(slog.String("name", forcedReleaseInfo.Name), slog.String("version", forcedReleaseInfo.Version.Original())))

		logger.Debug("forced release found", slog.String("name", forcedReleaseInfo.Name), slog.String("version", forcedReleaseInfo.Version.Original()))

		// if forced version is greater than the pending one, this pending release should be skipped
		if forcedReleaseInfo.Version.GreaterThan(release.GetVersion()) {
			logger.Debug("release must be skipped because force release is greater")

			return &Task{
				TaskType: Skip,
			}, nil
		}
	}

	deployedReleaseInfo := getFirstReleaseInfoByPhase(releases, v1alpha1.DeckhouseReleasePhaseDeployed)

	// if we have a deployed release
	if deployedReleaseInfo != nil {
		logger = logger.WithGroup("deployed_release").With(slog.String("name", deployedReleaseInfo.Name), slog.String("version", deployedReleaseInfo.Version.Original()))

		logger.Debug("deployed release found")

		// if deployed version is greater than the pending one, this pending release should be skipped
		if deployedReleaseInfo.Version.GreaterThan(release.GetVersion()) {
			logger.Debug("release must be skipped, because deployed release is greater")

			return &Task{
				TaskType: Skip,
			}, nil
		}

		// if we patch between reconcile start and calculating
		if deployedReleaseInfo.Version.Equal(release.GetVersion()) {
			logger.Debug("release version are equal deployed version")

			return nil, ErrReleaseIsAlreadyDeployed
		}
	}

	releaseIdx, _ := slices.BinarySearchFunc(releases, release.GetVersion(), func(a v1alpha1.Release, b *semver.Version) int {
		return a.GetVersion().Compare(b)
	})

	// max value for release queue depth is 3 due to the alert's logic, having queue depth greater than 3 breaks this logic
	// compute depth including current release (off-by-one fix): len(releases) - releaseIdx
	releaseQueueDepth := min(len(releases)-releaseIdx, 3)
	isLatestRelease := releaseQueueDepth == 0
	isPatch := true

	// If update constraints allow jumping to a final endpoint, skip intermediate pendings and process endpoint as minor.
	if deployedReleaseInfo != nil {
		endpointIdx := p.findConstraintEndpointIndex(releases, deployedReleaseInfo, logger)

		logger.Debug("from-to release found", slog.Int("release_index", releaseIdx), slog.Int("constraint_endpoint_index", endpointIdx))

		// If current release is the endpoint, process it.
		// And if current is after endpoint – proceed with normal flow below
		if releaseIdx == endpointIdx {
			logger.Debug("processing as endpoint due to update constraints")

			return &Task{
				TaskType:            Process,
				IsPatch:             false,
				IsLatest:            endpointIdx == len(releases)-1,
				DeployedReleaseInfo: deployedReleaseInfo.RemapToReleaseInfo(),
				QueueDepth:          min(len(releases)-releaseIdx, 3),
			}, nil
		}
	}

	// check previous release
	// only for awaiting purpose
	if releaseIdx > 0 {
		prevRelease := releases[releaseIdx-1]

		// if release version is greater in major or minor version than previous release
		if !isPatchRelease(prevRelease.GetVersion(), release.GetVersion()) ||
			(deployedReleaseInfo != nil && !isPatchRelease(deployedReleaseInfo.Version, release.GetVersion())) {
			isPatch = false

			// it must await if previous release has Deployed state
			// truncate all not deployed phase releases
			if prevRelease.GetPhase() == v1alpha1.DeckhouseReleasePhasePending {
				msg := prevRelease.GetMessage()
				if !strings.Contains(msg, "awaiting") {
					msg = fmt.Sprintf("awaiting for v%s release to be deployed", prevRelease.GetVersion().String())
				}

				logger.Debug("release awaiting", slog.String("reason", msg))

				return &Task{
					TaskType:            Await,
					Message:             msg,
					DeployedReleaseInfo: deployedReleaseInfo.RemapToReleaseInfo(),
				}, nil
			}

			// logic for equal major versions (unless constraints endpoint is ahead)
			if release.GetVersion().Major() == prevRelease.GetVersion().Major() {
				// here we have only Deployed phase releases in prevRelease
				ltsRelease := strings.EqualFold(p.releaseChannel, ltsReleaseChannel)

				// it must await if deployed release has minor version more than one
				if !ltsRelease &&
					release.GetVersion().Minor()-1 > prevRelease.GetVersion().Minor() {
					msg := fmt.Sprintf(
						"minor version is greater than deployed %s by one",
						prevRelease.GetVersion().Original(),
					)

					logger.Debug("release awaiting", slog.String("channel", p.releaseChannel), slog.String("reason", msg))

					return &Task{
						TaskType:            Await,
						Message:             msg,
						DeployedReleaseInfo: deployedReleaseInfo.RemapToReleaseInfo(),
					}, nil
				}

				// it must await if deployed release has minor version more than acceptable LTS channel limitation
				if ltsRelease && release.GetVersion().Minor() > prevRelease.GetVersion().Minor()+maxMinorVersionDiffForLTS {
					msg := fmt.Sprintf(
						"minor version is greater than deployed %s by %d, it's more than acceptable channel limitation",
						prevRelease.GetVersion().Original(),
						release.GetVersion().Minor()-prevRelease.GetVersion().Minor(),
					)

					logger.Debug("release awaiting", slog.String("channel", p.releaseChannel), slog.String("reason", msg))

					return &Task{
						TaskType:            Await,
						Message:             msg,
						DeployedReleaseInfo: deployedReleaseInfo.RemapToReleaseInfo(),
					}, nil
				}
			}

			// logic for greater major versions
			if release.GetVersion().Major() > prevRelease.GetVersion().Major() {
				// it must await if trying to update major version other than 0->1
				if prevRelease.GetVersion().Major() != 0 || release.GetVersion().Major() != 1 {
					msg := fmt.Sprintf(
						"major version is greater than deployed %s",
						prevRelease.GetVersion().Original(),
					)

					logger.Debug("release awaiting", slog.String("channel", p.releaseChannel), slog.String("reason", msg))

					return &Task{
						TaskType:            Await,
						Message:             msg,
						DeployedReleaseInfo: deployedReleaseInfo.RemapToReleaseInfo(),
					}, nil
				}
			}
		}
	}

	logger.With(slog.Bool("is_patch", isPatch), slog.Bool("is_latest", isLatestRelease))

	// check next release
	// patch calculate logic
	if len(releases)-1 > releaseIdx {
		nextRelease := releases[releaseIdx+1]

		// if nextRelease version is greater in major or minor version
		// current release is definitely greatest at patch version
		//
		// "isPatch" value could be false, if we have versions like:
		// 1.65.0 (Deployed)
		// 1.66.0 (Pending) - is greatest patch now, but must handle like minor version bump
		// 1.67.0 (Pending)
		if release.GetVersion().Major() < nextRelease.GetVersion().Major() ||
			release.GetVersion().Minor() < nextRelease.GetVersion().Minor() {
			logger.Debug("processing")

			return &Task{
				TaskType:            Process,
				IsPatch:             isPatch,
				IsLatest:            isLatestRelease,
				DeployedReleaseInfo: deployedReleaseInfo.RemapToReleaseInfo(),
				QueueDepth:          releaseQueueDepth,
			}, nil
		}

		logger.Debug("skipping")

		return &Task{
			TaskType: Skip,
			IsPatch:  isPatch,
		}, nil
	}

	logger.Debug("processing")

	// neighbours checks passed
	// only minor/major releases must be here
	return &Task{
		TaskType:            Process,
		IsLatest:            isLatestRelease,
		IsPatch:             isPatch,
		DeployedReleaseInfo: deployedReleaseInfo.RemapToReleaseInfo(),
		QueueDepth:          releaseQueueDepth,
	}, nil
}

func (p *TaskCalculator) listReleases(ctx context.Context, moduleName string) ([]v1alpha1.Release, error) {
	return p.listFunc(ctx, p.k8sclient, moduleName)
}

// findConstraintEndpointIndex determines the index of the final endpoint release allowed by updateConstraints
// Rules:
// - Look into the processing release's spec.update (if exists)
// - For the deployed version D and current processing release P, find a constraint where D in range [from, to].
// - If no endpoint found in current constraints, return -1.
func (p *TaskCalculator) findConstraintEndpointIndex(releases []v1alpha1.Release, deployed *releaseInfo, logEntry *log.Logger) int {
	compliantRelease := -1

	// Pick constraints from the highest pending release that has them.
	for i := len(releases) - 1; i >= 0; i-- {
		// compliant release can not be lower or equal deployed
		if i <= deployed.IndexInReleaseList {
			break
		}

		r := releases[i]

		if r.GetPhase() != v1alpha1.ModuleReleasePhasePending {
			continue
		}

		if r.GetUpdateSpec() == nil || len(r.GetUpdateSpec().Versions) == 0 {
			continue
		}

		compliantRelease = p.getFirstCompliantRelease(releases, r.GetUpdateSpec().Versions, deployed, logEntry)
	}

	return compliantRelease
}

// getFirstCompliantRelease determines the index of the first update constraints compliant release
// Rules:
// - For the deployed version D and current processing release P, find a constraint where D in range [from, to].
// - If no endpoint found in current constraints, return -1.
func (p *TaskCalculator) getFirstCompliantRelease(releases []v1alpha1.Release, constraints []v1alpha1.UpdateConstraint, deployed *releaseInfo, logEntry *log.Logger) int {
	bestIdx := -1

	// Check each constraint for inclusion of deployed version
	for _, c := range constraints {
		fromVer, err := semver.NewVersion(c.From)
		if err != nil {
			logEntry.Warn("parse semver", slog.String("version_from", c.From), log.Err(err))

			continue
		}

		toVer, err := semver.NewVersion(c.To)
		if err != nil {
			logEntry.Warn("parse semver", slog.String("version_to", c.To), log.Err(err))

			continue
		}

		// if deployed version is lower than "from" constraint
		if deployed.Version.Compare(fromVer) < 0 {
			logEntry.Debug("skip from constraint because deployed version is lower", slog.String("from_version", "v"+fromVer.String()))

			continue
		}

		// if deployed version is greater or equal "to" constraint
		if deployed.Version.Compare(toVer) >= 0 {
			logEntry.Debug("skip to constraint because deployed version higher or equal", slog.String("to_version", "v"+toVer.String()))

			continue
		}

		// Find highest patch within target minor/major
		for idx, r := range releases {
			if idx <= deployed.IndexInReleaseList {
				continue
			}

			rv := r.GetVersion()

			// trying to get first version with the same Major and Minor version as "to" constraint
			if rv.Major() == toVer.Major() && rv.Minor() == toVer.Minor() {
				if bestIdx == -1 {
					bestIdx = idx

					continue
				}
			}

			if bestIdx == -1 {
				continue
			}

			bestIdxVersion := releases[bestIdx].GetVersion()

			if bestIdxVersion.Patch() < rv.Patch() ||
				bestIdxVersion.Minor() < rv.Minor() ||
				bestIdxVersion.Major() < rv.Major() {
				bestIdx = idx
			}
		}
	}

	return bestIdx
}

// getFirstReleaseInfoByPhase
// releases slice must be sorted asc
func getFirstReleaseInfoByPhase(releases []v1alpha1.Release, phase string) *releaseInfo {
	idx := slices.IndexFunc(releases, func(a v1alpha1.Release) bool {
		return a.GetPhase() == phase
	})

	if idx == -1 {
		return nil
	}

	filteredDR := releases[idx]

	return &releaseInfo{
		IndexInReleaseList: idx,
		Name:               filteredDR.GetName(),
		Version:            filteredDR.GetVersion(),
	}
}

// getLatestForcedReleaseInfo
// releases slice must be sorted asc
func getLatestForcedReleaseInfo(releases []v1alpha1.Release) *releaseInfo {
	for idx, release := range slices.Backward(releases) {
		if !release.GetForce() {
			continue
		}

		return &releaseInfo{
			IndexInReleaseList: idx,
			Name:               release.GetName(),
			Version:            release.GetVersion(),
		}
	}

	return nil
}

func listDeckhouseReleases(ctx context.Context, c client.Client, _ string) ([]v1alpha1.Release, error) {
	releases := new(v1alpha1.DeckhouseReleaseList)

	if err := c.List(ctx, releases); err != nil {
		return nil, fmt.Errorf("get deckhouse releases: %w", err)
	}

	result := make([]v1alpha1.Release, 0, len(releases.Items))

	for _, release := range releases.Items {
		result = append(result, &release)
	}

	return result, nil
}

func listModuleReleases(ctx context.Context, c client.Client, moduleName string) ([]v1alpha1.Release, error) {
	releases := new(v1alpha1.ModuleReleaseList)
	// Do not rely on label presence; list all and filter by spec.moduleName
	if err := c.List(ctx, releases); err != nil {
		return nil, fmt.Errorf("get module releases: %w", err)
	}

	result := make([]v1alpha1.Release, 0, len(releases.Items))
	for i := range releases.Items {
		rel := &releases.Items[i]
		if rel.GetModuleName() != moduleName {
			continue
		}
		result = append(result, rel)
	}

	return result, nil
}
