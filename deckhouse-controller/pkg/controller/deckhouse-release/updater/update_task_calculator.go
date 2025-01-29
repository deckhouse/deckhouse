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

package d8updater

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
)

type TaskCalculator struct {
	k8sclient client.Client

	log *log.Logger
}

func NewTaskCalculator(k8sclient client.Client, logger *log.Logger) *TaskCalculator {
	return &TaskCalculator{
		k8sclient: k8sclient,
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

// CalculatePendingReleaseOrder calculate task with information about current reconcile
//
// calculating flow:
// 1) find forced release. if current release has a lower version - skip
// 2) find deployed release. if current release has a lower version - skip
func (p *TaskCalculator) CalculatePendingReleaseOrder(ctx context.Context, dr *v1alpha1.DeckhouseRelease) (*Task, error) {
	if dr.GetPhase() != v1alpha1.DeckhouseReleasePhasePending {
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

	slices.SortFunc(releases, func(a, b v1alpha1.DeckhouseRelease) int {
		return a.GetVersion().Compare(b.GetVersion())
	})

	forcedReleaseInfo := p.getLatestForcedReleaseInfo(releases)

	// if we have a forced release
	if forcedReleaseInfo != nil {
		// if forced version is greater than the pending one, this pending release should be skipped
		if forcedReleaseInfo.Version.GreaterThan(dr.GetVersion()) {
			return &Task{
				TaskType: Skip,
			}, nil
		}
	}

	deployedReleaseInfo := p.getFirstReleaseInfoByPhase(releases, v1alpha1.DeckhouseReleasePhaseDeployed)

	// if we have a deployed release
	if deployedReleaseInfo != nil {
		// if deployed version is greater than the pending one, this pending release should be skipped
		if deployedReleaseInfo.Version.GreaterThan(dr.GetVersion()) {
			return &Task{
				TaskType: Skip,
			}, nil
		}

		// if we patch between reconcile start and calculating
		if deployedReleaseInfo.Version.Equal(dr.GetVersion()) {
			return nil, ErrReleaseIsAlreadyDeployed
		}
	}

	releaseIdx, _ := slices.BinarySearchFunc(releases, dr.GetVersion(), func(a v1alpha1.DeckhouseRelease, b *semver.Version) int {
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
		if dr.GetVersion().Major() > prevRelease.GetVersion().Major() ||
			dr.GetVersion().Minor() > prevRelease.GetVersion().Minor() {
			isPatch = false

			// it must await if previous release has Deployed state
			if prevRelease.GetPhase() != v1alpha1.DeckhouseReleasePhaseDeployed {
				msg := prevRelease.Status.Message
				if !strings.Contains(msg, "awaiting") {
					msg = fmt.Sprintf("awaiting for v%s release to be deployed", prevRelease.GetVersion().String())
				}

				return &Task{
					TaskType:            Await,
					Message:             msg,
					DeployedReleaseInfo: deployedReleaseInfo,
				}, nil
			}

			// it must await if deployed release has minor version more than one
			if deployedReleaseInfo != nil && dr.GetVersion().Minor()-1 > deployedReleaseInfo.Version.Minor() {
				return &Task{
					TaskType:            Await,
					Message:             fmt.Sprintf("minor version is more than deployed v%s by one", prevRelease.GetVersion().String()),
					DeployedReleaseInfo: deployedReleaseInfo,
				}, nil
			}
		}
	}

	// check next release
	// patch calculate logic
	if len(releases)-1 > releaseIdx {
		nextRelease := releases[releaseIdx+1]

		// if nextRelease version is greater in major or minor version
		// current release is definitely greatest at patch version
		//
		// "isPatch" value could be false, if we have versions like:
		// 1.65.0 (Deployed)
		// 1.66.0 (Pending) - is greatest patch now, bot must handle like minor version bump
		// 1.67.0 (Pending)
		if dr.GetVersion().Major() < nextRelease.GetVersion().Major() ||
			dr.GetVersion().Minor() < nextRelease.GetVersion().Minor() {
			return &Task{
				TaskType:            Process,
				IsPatch:             isPatch,
				IsLatest:            isLatestRelease,
				DeployedReleaseInfo: deployedReleaseInfo,
				QueueDepth:          releaseQueueDepth,
			}, nil
		}

		return &Task{
			TaskType: Skip,
			IsPatch:  isPatch,
		}, nil
	}

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

func (p *TaskCalculator) listReleases(ctx context.Context) ([]v1alpha1.DeckhouseRelease, error) {
	var releases v1alpha1.DeckhouseReleaseList
	err := p.k8sclient.List(ctx, &releases)
	if err != nil {
		return nil, fmt.Errorf("get deckhouse releases: %w", err)
	}

	return releases.Items, nil
}

// getFirstReleaseInfoByPhase
// releases slice must be sorted asc
func (p *TaskCalculator) getFirstReleaseInfoByPhase(releases []v1alpha1.DeckhouseRelease, phase string) *ReleaseInfo {
	idx := slices.IndexFunc(releases, func(a v1alpha1.DeckhouseRelease) bool {
		return a.Status.Phase == phase
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
func (p *TaskCalculator) getLatestForcedReleaseInfo(releases []v1alpha1.DeckhouseRelease) *ReleaseInfo {
	for _, dr := range slices.Backward(releases) {
		if !dr.GetForce() {
			continue
		}

		return &ReleaseInfo{
			Name:    dr.GetName(),
			Version: dr.GetVersion(),
		}
	}

	return nil
}
