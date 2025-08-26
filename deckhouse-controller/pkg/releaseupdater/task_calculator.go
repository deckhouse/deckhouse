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
	QueueDepth          *ReleaseQueueDepthDelta
}

type ReleaseInfo struct {
	Name    string
	Version *semver.Version
}

// ReleaseQueueDepthDelta represents the difference between deployed and latest releases
type ReleaseQueueDepthDelta struct {
	Major int // Major versions delta, not used right now, for future usage
	Minor int // Minor versions delta
	Patch int // Patch versions delta
}

// GetReleaseQueueDepth calculates the effective queue depth for monitoring and alerting purposes.
// This method transforms the internal delta representation into a simplified metric value that
// represents how many releases are pending deployment.
//
// Queue depth calculation logic:
//   - If minor version differences exist: return the minor delta count
//   - If only patch version differences exist: return 1 (normalized patch indicator)
//   - If no differences exist: return 0 (up to date)
//
// The function prioritizes minor version gaps over patch version gaps because:
//  1. Minor version updates typically contain more significant changes
//  2. Multiple patch versions are normalized to a single indicator (1)
//  3. This provides a cleaner metric for alerting thresholds
//
// Examples:
//   - Delta{Major: 0, Minor: 3, Patch: 0} → Returns: 3 (3 minor versions behind)
//   - Delta{Major: 0, Minor: 0, Patch: 5} → Returns: 1 (patch updates available, normalized)
//   - Delta{Major: 1, Minor: 2, Patch: 0} → Returns: 2 (focuses on minor gap, major handled separately)
//   - Delta{Major: 0, Minor: 0, Patch: 0} → Returns: 0 (up to date)
//
// Note: Major version deltas are intentionally excluded from this calculation.
// A separate alerting mechanism is planned for major version updates due to their
// potentially breaking nature and different handling requirements.
func (d *ReleaseQueueDepthDelta) GetReleaseQueueDepth() int {
	if d.Minor > 0 {
		return d.Minor
	}

	if d.Patch > 0 {
		return 1
	}

	return 0
}

var ErrReleasePhaseIsNotPending = errors.New("release phase is not pending")
var ErrReleaseIsAlreadyDeployed = errors.New("release is already deployed")

func isPatchRelease(a, b *semver.Version) bool {
	if b.Major() == a.Major() && b.Minor() == a.Minor() && b.Patch() > a.Patch() {
		return true
	}

	return false
}

// calculateReleaseQueueDepthDelta computes the version gap between the currently deployed release
// and the latest available release. This delta is used for monitoring, alerting, and metrics
// to understand how far behind the deployed version is from the latest available version.
//
// The function implements a hierarchical priority system for version differences:
//
// 1. MAJOR VERSION PRIORITY: When a major version difference exists, it takes precedence.
//   - Calculate major version delta (latest.major - deployed.major)
//   - Additionally calculate minor version delta within the deployed major version range
//   - This helps track both the major jump and any skipped minors within the current major
//
// 2. MINOR VERSION PRIORITY: When major versions are equal but minor differs.
//   - Calculate only minor version delta (latest.minor - deployed.minor)
//   - Patch differences are ignored when minor differences exist
//
// 3. PATCH VERSION FALLBACK: When major and minor are identical.
//   - Calculate patch version delta (latest.patch - deployed.patch)
//
// Example scenarios:
//
//	Scenario 1 - Major version jump with minor tracking:
//	Deployed: 1.67.5
//	Releases: 1.68.2, 1.69.7, 1.70.0, 2.0.1, 2.1.5
//	Latest: 2.1.5
//	Result: { Major: 1, Minor: 3, Patch: 0 }
//	Explanation: 1 major jump (1→2), 3 minor versions within major 1 (67→70)
//
//	Scenario 2 - Minor version difference:
//	Deployed: 1.67.5
//	Latest: 1.70.3
//	Result: { Major: 0, Minor: 3, Patch: 0 }
//	Explanation: Same major (1), 3 minor versions difference (67→70)
//
//	Scenario 3 - Patch version difference:
//	Deployed: 1.67.5
//	Latest: 1.67.8
//	Result: { Major: 0, Minor: 0, Patch: 3 }
//	Explanation: Same major.minor (1.67), 3 patch versions difference (5→8)
func calculateReleaseQueueDepthDelta(releases []v1alpha1.Release, deployedReleaseInfo *ReleaseInfo) *ReleaseQueueDepthDelta {
	delta := &ReleaseQueueDepthDelta{}

	if deployedReleaseInfo == nil || len(releases) == 0 {
		return delta
	}

	deployed := deployedReleaseInfo.Version
	latestRelease := releases[len(releases)-1]
	latest := latestRelease.GetVersion()

	// major delta exists
	if latest.Major() > deployed.Major() {
		delta.Major = int(latest.Major() - deployed.Major())

		var latestInSameMajor *semver.Version

		// find the latest release in the same major version as deployed
		for i := len(releases) - 1; i >= 0; i-- {
			if releases[i].GetVersion().Major() == deployed.Major() {
				latestInSameMajor = releases[i].GetVersion()
				break
			}
		}

		if latestInSameMajor != nil && latestInSameMajor.Minor() > deployed.Minor() {
			delta.Minor = int(latestInSameMajor.Minor() - deployed.Minor())
		}

		return delta
	}

	// skip Patch in case Minor delta exists
	if latest.Minor() > deployed.Minor() {
		delta.Minor = int(latest.Minor() - deployed.Minor())
		return delta
	}

	if latest.Patch() > deployed.Patch() {
		delta.Patch = int(latest.Patch() - deployed.Patch())
	}

	return delta
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

	queueDepthDelta := calculateReleaseQueueDepthDelta(releases, deployedReleaseInfo)
	isLatestRelease := queueDepthDelta.GetReleaseQueueDepth() == 0
	isPatch := true

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
					DeployedReleaseInfo: deployedReleaseInfo,
					QueueDepth:          queueDepthDelta,
				}, nil
			}

			// logic for equal major versions
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
						DeployedReleaseInfo: deployedReleaseInfo,
						QueueDepth:          queueDepthDelta,
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
						DeployedReleaseInfo: deployedReleaseInfo,
						QueueDepth:          queueDepthDelta,
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
						DeployedReleaseInfo: deployedReleaseInfo,
						QueueDepth:          queueDepthDelta,
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
				DeployedReleaseInfo: deployedReleaseInfo,
				QueueDepth:          queueDepthDelta,
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
		DeployedReleaseInfo: deployedReleaseInfo,
		QueueDepth:          queueDepthDelta,
	}, nil
}

func (p *TaskCalculator) listReleases(ctx context.Context, moduleName string) ([]v1alpha1.Release, error) {
	return p.listFunc(ctx, p.k8sclient, moduleName)
}

// getFirstReleaseInfoByPhase
// releases slice must be sorted asc
func getFirstReleaseInfoByPhase(releases []v1alpha1.Release, phase string) *ReleaseInfo {
	idx := slices.IndexFunc(releases, func(a v1alpha1.Release) bool {
		return a.GetPhase() == phase
	})

	if idx == -1 {
		return nil
	}

	filteredDR := releases[idx]

	return &ReleaseInfo{
		Name:    filteredDR.GetName(),
		Version: filteredDR.GetVersion(),
	}
}

// getLatestForcedReleaseInfo
// releases slice must be sorted asc
func getLatestForcedReleaseInfo(releases []v1alpha1.Release) *ReleaseInfo {
	for _, release := range slices.Backward(releases) {
		if !release.GetForce() {
			continue
		}

		return &ReleaseInfo{
			Name:    release.GetName(),
			Version: release.GetVersion(),
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
	err := c.List(ctx, releases, client.MatchingLabels{v1alpha1.ModuleReleaseLabelModule: moduleName})
	if err != nil {
		return nil, fmt.Errorf("get module releases: %w", err)
	}

	result := make([]v1alpha1.Release, 0, len(releases.Items))

	for _, release := range releases.Items {
		result = append(result, &release)
	}

	return result, nil
}
