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
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
)

type TaskCalculator struct {
	k8sclient client.Client

	listFunc func(ctx context.Context, c client.Client) ([]v1alpha1.Release, error)

	log *log.Logger
}

func NewDeckhouseReleaseTaskCalculator(k8sclient client.Client, logger *log.Logger) *TaskCalculator {
	return &TaskCalculator{
		k8sclient: k8sclient,
		listFunc:  listDeckhouseReleases,
		log:       logger,
	}
}

func NewModuleReleaseTaskCalculator(k8sclient client.Client, logger *log.Logger) *TaskCalculator {
	return &TaskCalculator{
		k8sclient: k8sclient,
		listFunc:  listModuleReleases,
		log:       logger,
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

var ErrReleasePhaseIsNotPending = errors.New("release phase is not pending")
var ErrReleaseIsAlreadyDeployed = errors.New("release is already deployed")

// CalculatePendingReleaseTask calculate task with information about current reconcile
//
// calculating flow:
// 1) find forced release. if current release has a lower version - skip
// 2) find deployed release. if current release has a lower version - skip
func (p *TaskCalculator) CalculatePendingReleaseTask(ctx context.Context, release v1alpha1.Release) (*Task, error) {
	logger := p.log.With(slog.String("release", release.GetName()))

	if release.GetPhase() != v1alpha1.DeckhouseReleasePhasePending {
		return nil, ErrReleasePhaseIsNotPending
	}

	releases, err := p.listReleases(ctx)
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

	forcedReleaseInfo := p.getLatestForcedReleaseInfo(releases)

	// if we have a forced release
	if forcedReleaseInfo != nil {
		logger = logger.With(logger.WithGroup("forced_release").With(slog.String("name", forcedReleaseInfo.Name), slog.String("version", forcedReleaseInfo.Version.Original())))

		logger.Debug("forced release found", slog.String("name", forcedReleaseInfo.Name), slog.String("version", forcedReleaseInfo.Version.Original()))

		// if forced version is greater than the pending one, this pending release should be skipped
		if forcedReleaseInfo.Version.GreaterThan(release.GetVersion()) {
			return &Task{
				TaskType: Skip,
			}, nil
		}
	}

	deployedReleaseInfo := p.getFirstReleaseInfoByPhase(releases, v1alpha1.DeckhouseReleasePhaseDeployed)

	// if we have a deployed release
	if deployedReleaseInfo != nil {
		logger = logger.With(logger.WithGroup("deployed_release").With(slog.String("name", deployedReleaseInfo.Name), slog.String("version", deployedReleaseInfo.Version.Original())))

		logger.Debug("deployed release found")

		// if deployed version is greater than the pending one, this pending release should be skipped
		if deployedReleaseInfo.Version.GreaterThan(release.GetVersion()) {
			return &Task{
				TaskType: Skip,
			}, nil
		}

		// if we patch between reconcile start and calculating
		if deployedReleaseInfo.Version.Equal(release.GetVersion()) {
			return nil, ErrReleaseIsAlreadyDeployed
		}
	}

	releaseIdx, _ := slices.BinarySearchFunc(releases, release.GetVersion(), func(a v1alpha1.Release, b *semver.Version) int {
		return a.GetVersion().Compare(b)
	})

	releaseQueueDepth := len(releases) - 1 - releaseIdx
	isLatestRelease := releaseQueueDepth == 0
	isPatch := true

	// check previous release
	// only for awaiting purpose
	if releaseIdx > 0 {
		prevRelease := releases[releaseIdx-1]

		// if release version is greater in major or minor version than previous release
		if release.GetVersion().Major() > prevRelease.GetVersion().Major() ||
			release.GetVersion().Minor() > prevRelease.GetVersion().Minor() {
			isPatch = false

			// it must await if previous release has Deployed state
			// truncate all not deployed phase releases
			if prevRelease.GetPhase() != v1alpha1.DeckhouseReleasePhaseDeployed {
				msg := prevRelease.GetMessage()
				if !strings.Contains(msg, "awaiting") {
					msg = fmt.Sprintf("awaiting for v%s release to be deployed", prevRelease.GetVersion().String())
				}

				logger.Debug("release awaiting", slog.String("reason", msg))

				return &Task{
					TaskType:            Await,
					Message:             msg,
					DeployedReleaseInfo: deployedReleaseInfo,
				}, nil
			}

			// here we have only Deployed phase releases in prevRelease
			// it must await if deployed release has minor version more than one
			if release.GetVersion().Minor()-1 > prevRelease.GetVersion().Minor() {
				msg := fmt.Sprintf("minor version is greater than deployed %s by one", prevRelease.GetVersion().Original())

				logger.Debug("release awaiting", slog.String("reason", msg))

				return &Task{
					TaskType:            Await,
					Message:             msg,
					DeployedReleaseInfo: deployedReleaseInfo,
				}, nil
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
		DeployedReleaseInfo: deployedReleaseInfo,
		QueueDepth:          releaseQueueDepth,
	}, nil
}

func (p *TaskCalculator) listReleases(ctx context.Context) ([]v1alpha1.Release, error) {
	return p.listFunc(ctx, p.k8sclient)
}

// getFirstReleaseInfoByPhase
// releases slice must be sorted asc
func (p *TaskCalculator) getFirstReleaseInfoByPhase(releases []v1alpha1.Release, phase string) *ReleaseInfo {
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
func (p *TaskCalculator) getLatestForcedReleaseInfo(releases []v1alpha1.Release) *ReleaseInfo {
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

func listDeckhouseReleases(ctx context.Context, c client.Client) ([]v1alpha1.Release, error) {
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

func listModuleReleases(ctx context.Context, c client.Client) ([]v1alpha1.Release, error) {
	releases := new(v1alpha1.ModuleReleaseList)
	if err := c.List(ctx, releases); err != nil {
		return nil, fmt.Errorf("get deckhouse releases: %w", err)
	}

	result := make([]v1alpha1.Release, 0, len(releases.Items))

	for _, release := range releases.Items {
		result = append(result, &release)
	}

	return result, nil
}
