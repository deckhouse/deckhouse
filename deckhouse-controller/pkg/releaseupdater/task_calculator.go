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
	deckhouseModuleName       = "" // Empty string indicates Deckhouse release (not a module)
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
func NewModuleReleaseTaskCalculator(k8sclient client.Client, releaseChannel string, logger *log.Logger) *TaskCalculator {
	return &TaskCalculator{
		k8sclient:      k8sclient,
		listFunc:       listModuleReleases,
		log:            logger,
		releaseChannel: releaseChannel,
	}
}

type TaskType int

const (
	Skip TaskType = iota
	Await
	Process
)

// String returns the string representation of TaskType
func (t TaskType) String() string {
	switch t {
	case Skip:
		return "skip"
	case Await:
		return "await"
	case Process:
		return "process"
	default:
		return "unknown"
	}
}

type Task struct {
	TaskType TaskType
	Message  string

	IsMajor  bool
	IsFromTo bool
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

// ReleaseQueueDepthDelta represents the difference between deployed and latest releases
type ReleaseQueueDepthDelta struct {
	Major int // Major versions delta
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
	if d == nil {
		return 0
	}

	if d.Minor > 0 {
		return d.Minor
	}

	if d.Patch > 0 {
		return 1
	}

	return 0
}

// GetMajorReleaseDepth returns the major version difference for monitoring and alerting purposes.
// This method provides a dedicated metric for tracking major version updates separately from
// minor and patch updates due to their potentially breaking nature.
//
// Major release depth calculation logic:
//   - If major version differences exist: return the major delta count
//   - If no major differences exist: return 0 (up to date on major version)
//
// Examples:
//   - Delta{Major: 2, Minor: 3, Patch: 1} → Returns: 2 (2 major versions behind)
//   - Delta{Major: 1, Minor: 0, Patch: 0} → Returns: 1 (1 major version behind)
//   - Delta{Major: 0, Minor: 5, Patch: 2} → Returns: 0 (up to date on major version)
func (d *ReleaseQueueDepthDelta) GetMajorReleaseDepth() int {
	if d == nil || d.Major <= 0 {
		return 0
	}
	return d.Major
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
func calculateReleaseQueueDepthDelta(releases []v1alpha1.Release, deployedReleaseInfo *releaseInfo) *ReleaseQueueDepthDelta {
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

const (
	ltsReleaseChannel = "lts"
)

var ErrReleasePhaseIsNotPending = errors.New("release phase is not pending")
var ErrReleaseIsAlreadyDeployed = errors.New("release is already deployed")

func isPatchRelease(a, b *semver.Version) bool {
	if b.Major() == a.Major() && b.Minor() == a.Minor() && b.Patch() > a.Patch() {
		return true
	}

	return false
}

// CalculatePendingReleaseTask determines the appropriate action for a pending release within
// the context of all available releases. This is the main orchestration function that evaluates
// release precedence, version constraints, and deployment readiness to produce a task decision.
//
// Decision Flow Architecture:
//  1. Validation: Ensure release is in pending state
//  2. Force Release Check: Handle administratively forced releases
//  3. Deployed Release Analysis: Consider currently deployed version
//  4. Constraint Evaluation: Check for version jumping opportunities
//  5. Sequential Logic: Apply standard version progression rules
//  6. Neighbor Analysis: Determine if release should process or wait
//
// Task Types Returned:
//   - Skip: Release should be bypassed (superseded by force/deployed/newer release)
//   - Await: Release must wait for dependencies (previous releases, constraints)
//   - Process: Release is ready for deployment
//
// Key Features:
//   - Version Jumping: Supports constraint-based skipping of intermediate releases
//   - Channel-aware Logic: Different rules for LTS vs regular channels
//   - Major Version Control: Special handling for 0→1 transitions vs breaking changes
//   - Queue Depth Calculation: Provides metrics for monitoring and alerting
//   - Force Release Priority: Administrative overrides take precedence
//
// Version Progression Rules:
//  1. FORCE RELEASE: Always takes precedence, skips any lower versions
//  2. DEPLOYED VERSION: Cannot deploy older than currently deployed
//  3. MAJOR VERSION RESTRICTIONS:
//     - 0→1: Allowed (development to stable transition)
//     - 1→2+: Blocked (requires manual intervention)
//  4. MINOR VERSION LIMITS:
//     - Regular channels: Sequential (+1 minor at a time)
//     - LTS channels: Up to +10 minor versions allowed
//  5. PATCH VERSIONS: No restrictions, highest patch wins
//
// Constraint-Based Version Jumping:
//
//	When update constraints are present, releases can jump to specified endpoints:
//	- Deployed: 1.67.5, Constraint: {from: "1.67", to: "1.70"}
//	- Result: Direct jump to 1.70.x (highest patch), skipping 1.68.x, 1.69.x
//	- Endpoint releases are processed as minor updates (not patches)
//
// Examples:
//
//	Scenario 1 - Force Release Override:
//	Input: Pending v1.68.0, Force v1.70.0 exists
//	Result: Skip (v1.68.0 < v1.70.0)
//
//	Scenario 2 - Sequential Minor Update:
//	Input: Pending v1.68.0, Deployed v1.67.5, No constraints
//	Result: Process (valid +1 minor progression)
//
//	Scenario 3 - Major Version Block:
//	Input: Pending v2.0.0, Deployed v1.67.5
//	Result: Await (major version jump requires manual approval)
//
//	Scenario 4 - Constraint Jumping:
//	Input: Pending v1.70.0, Deployed v1.67.5, Constraint: {from: "1.67", to: "1.70"}
//	Result: Process (constraint allows jumping to v1.70.0)
//
//	Scenario 5 - Patch Priority:
//	Input: Pending v1.67.3, Next v1.67.5 exists
//	Result: Skip (higher patch available)
//
// Queue Depth Calculation:
//   - Counts releases between current and latest (max 3 for alerting)
//   - Used for monitoring release lag and alert thresholds
//   - Includes current release in count (off-by-one correction)
//
// Channel-Specific Behavior:
//   - Regular Channels: Strict +1 minor version progression
//   - LTS Channels: Allow up to +10 minor version jumps for stability
//   - All Channels: No restrictions on patch version progression
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
	isMajor := false

	// If update constraints allow jumping to a final endpoint, skip intermediate pendings and process endpoint as minor.
	if deployedReleaseInfo != nil {
		isMajor := release.GetVersion().Major() > deployedReleaseInfo.Version.Major()
		endpointIdx := p.findConstraintEndpointIndex(releases, deployedReleaseInfo, logger)

		if endpointIdx >= 0 {
			logger.Debug("from-to release found", slog.String("constraint_endpoint_version", "v"+releases[endpointIdx].GetVersion().String()))
		}

		// If current release is the endpoint, process it.
		// And if current is after endpoint – proceed with normal flow below
		if releaseIdx == endpointIdx {
			logger.Debug("processing as endpoint due to update constraints")

			return &Task{
				TaskType:            Process,
				IsPatch:             false,
				IsMajor:             isMajor,
				IsFromTo:            true,
				IsLatest:            endpointIdx == len(releases)-1,
				DeployedReleaseInfo: deployedReleaseInfo.RemapToReleaseInfo(),
				QueueDepth:          queueDepthDelta,
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
					QueueDepth:          queueDepthDelta,
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
						QueueDepth:          queueDepthDelta,
					}, nil
				}

				isDeckhouseRelease := release.GetModuleName() == deckhouseModuleName
				// it must await if deployed release has minor version more than acceptable LTS channel limitation
				// For modules, skip this check (allow any minor version jump)
				if ltsRelease && isDeckhouseRelease && release.GetVersion().Minor() > prevRelease.GetVersion().Minor()+maxMinorVersionDiffForLTS {
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
						IsMajor:             isMajor,
						Message:             msg,
						DeployedReleaseInfo: deployedReleaseInfo.RemapToReleaseInfo(),
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
				DeployedReleaseInfo: deployedReleaseInfo.RemapToReleaseInfo(),
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
		DeployedReleaseInfo: deployedReleaseInfo.RemapToReleaseInfo(),
		QueueDepth:          queueDepthDelta,
	}, nil
}

func (p *TaskCalculator) listReleases(ctx context.Context, moduleName string) ([]v1alpha1.Release, error) {
	return p.listFunc(ctx, p.k8sclient, moduleName)
}

// findConstraintEndpointIndex locates the best constraint endpoint for version jumping.
// This function orchestrates the constraint evaluation process by:
//  1. Scanning all pending releases with update constraints (highest priority first)
//  2. Delegating constraint validation to getFirstCompliantRelease()
//  3. Selecting the highest valid endpoint across all constraint sources
//
// Search Strategy:
//   - Examines releases in reverse order (latest first) to prioritize newer constraints
//   - Only considers pending releases that come after the deployed release
//   - Aggregates results from multiple constraint sources within the same module
//
// Example:
//
//	Release A (v1.70.0): constraints [{from: "1.67", to: "1.69"}] → endpoint at index 5
//	Release B (v1.75.0): constraints [{from: "1.67", to: "1.72"}] → endpoint at index 8
//	Result: index 8 (highest endpoint wins)
func (p *TaskCalculator) findConstraintEndpointIndex(releases []v1alpha1.Release, deployed *releaseInfo, logEntry *log.Logger) int {
	compliantRelease := -1

	// Pick constraints from the highest pending release that has them.
	// compliant release can not be lower or equal deployed
	for i := len(releases) - 1; i > deployed.IndexInReleaseList; i-- {
		r := releases[i]

		if r.GetPhase() != v1alpha1.ModuleReleasePhasePending {
			continue
		}

		if r.GetUpdateSpec() == nil || len(r.GetUpdateSpec().Versions) == 0 {
			continue
		}

		releaseIndex := p.getFirstCompliantRelease(releases, r.GetUpdateSpec().Versions, deployed, r.GetVersion(), logEntry)
		if releaseIndex > compliantRelease {
			compliantRelease = releaseIndex
		}
	}

	return compliantRelease
}

// getFirstCompliantRelease determines the index of the highest patch release that satisfies
// update constraints for version jumping. This function is the core logic for finding valid
// constraint endpoints that allow skipping intermediate releases.
//
// The function implements a constraint-based release selection algorithm that:
// 1. Validates each constraint against the deployed version and target release
// 2. Ensures deployed version falls within the constraint range [from, to)
// 3. Finds the highest patch version within the target major.minor specified by "to"
// 4. Returns the index of the best matching release for constraint-based jumping
//
// Constraint Validation Rules:
//   - Deployed version must be >= constraint.from (inclusive lower bound)
//   - Deployed version must be < constraint.to (exclusive upper bound)
//   - Target release (constraintedReleaseVersion) must match constraint.to major.minor
//   - Only considers releases that come after the deployed release in the sorted list
//
// Selection Logic:
//   - Searches for releases with same major.minor as constraint.to
//   - Prefers the highest index (latest patch) within the target version
//   - Updates bestIdx only when finding a higher index than previously found
//   - Handles multiple constraints by selecting the highest valid endpoint
//
// Examples:
//
//	Scenario 1 - Single constraint match:
//	Deployed: 1.67.5 (index 2)
//	Constraint: {from: "1.67", to: "1.70"}
//	Releases: [..., 1.70.0 (index 5), 1.70.1 (index 6), 1.70.3 (index 7), ...]
//	Result: index 7 (highest patch in 1.70.x series)
//
//	Scenario 2 - Multiple constraints, highest wins:
//	Deployed: 1.67.5
//	Constraints: [{from: "1.67", to: "1.69"}, {from: "1.67", to: "1.72"}]
//	Releases: [..., 1.69.2 (index 5), 1.72.0 (index 8), ...]
//	Result: index 8 (1.72.0 is higher than 1.69.2)
//
//	Scenario 3 - No valid constraints:
//	Deployed: 1.75.0
//	Constraint: {from: "1.67", to: "1.70"}  // deployed > constraint range
//	Result: -1 (no valid constraint match)
func (p *TaskCalculator) getFirstCompliantRelease(
	releases []v1alpha1.Release,
	constraints []v1alpha1.UpdateConstraint,
	deployed *releaseInfo,
	constraintedReleaseVersion *semver.Version,
	logEntry *log.Logger,
) int {
	bestIdx := -1

	// Check each constraint for inclusion of deployed version
	for _, c := range constraints {
		logEntry.Debug("constrains found",
			slog.String("from_ver", c.From),
			slog.String("to_ver", c.To),
		)

		toVer, err := semver.NewVersion(c.To)
		if err != nil {
			logEntry.Warn("parse semver", slog.String("version_to", c.To), log.Err(err))

			continue
		}

		if constraintedReleaseVersion.Major() != toVer.Major() ||
			constraintedReleaseVersion.Minor() != toVer.Minor() {
			logEntry.Debug("skip constraint because major or minor version does not match to constrainted release version",
				slog.String("from_ver", c.From),
				slog.String("to_ver", c.To),
				slog.String("constrainted_release_version", "v"+constraintedReleaseVersion.String()),
			)
			continue
		}

		fromVer, err := semver.NewVersion(c.From)
		if err != nil {
			logEntry.Warn("parse semver", slog.String("version_from", c.From), log.Err(err))

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

		// starting scan index must be more than deployed index and already calculated compliant index
		startScanIdx := deployed.IndexInReleaseList
		if startScanIdx < bestIdx {
			startScanIdx = bestIdx
		}

		// Find highest patch within target minor/major
		for i := startScanIdx; i < len(releases); i++ {
			rv := releases[i].GetVersion()

			// trying to get first version with the same Major and Minor version as "to" constraint
			if rv.Major() == toVer.Major() && rv.Minor() == toVer.Minor() {
				if bestIdx < i {
					bestIdx = i
					logEntry.Debug("found most suitable index for from-to releaseleap",
						slog.String("suitable_version", "v"+releases[bestIdx].GetVersion().String()),
						slog.String("from_ver", c.From),
						slog.String("to_ver", c.To),
					)
				}
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
