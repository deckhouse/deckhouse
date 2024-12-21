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

type OrderCalculator struct {
	k8sclient client.Client

	log *log.Logger
}

func NewOrderCalculator(k8sclient client.Client, logger *log.Logger) *OrderCalculator {
	return &OrderCalculator{
		k8sclient: k8sclient,
		log:       logger,
	}
}

type Order int

const (
	Skip Order = iota
	Process
	Await
)

type CalculatingResult struct {
	Order   Order
	Message string

	IsPatch  bool
	IsSingle bool
	IsLatest bool

	DeployedReleaseInfo *ReleaseInfo
}

type ReleaseInfo struct {
	Name    string
	Version *semver.Version
}

var ErrReleasePhaseIsNotPending = errors.New("release phase is not pending")

func (p *OrderCalculator) CalculatePendingReleaseOrder(ctx context.Context, dr *v1alpha1.DeckhouseRelease) (*CalculatingResult, error) {
	if dr.GetPhase() != v1alpha1.DeckhouseReleasePhasePending {
		return nil, ErrReleasePhaseIsNotPending
	}

	releases, err := p.listReleases(ctx)
	if err != nil {
		return nil, fmt.Errorf("list releases: %w", err)
	}

	if len(releases) == 1 {
		return &CalculatingResult{
			Order:    Process,
			IsSingle: true,
			IsLatest: true,
		}, nil
	}

	slices.SortFunc(releases, func(a, b v1alpha1.DeckhouseRelease) int {
		return a.GetVersion().Compare(b.GetVersion())
	})

	deployedReleaseInfo := p.getDeployedReleaseInfo(releases)

	// if we have a deployed a release
	if deployedReleaseInfo != nil {
		// if deployed version is greater than the pending one, this pending release should be superseded
		if deployedReleaseInfo.Version.GreaterThan(dr.GetVersion()) {
			return &CalculatingResult{
				Order: Skip,
			}, nil
		}
	}

	relIdx, _ := slices.BinarySearchFunc(releases, dr.GetVersion(), func(a v1alpha1.DeckhouseRelease, b *semver.Version) int {
		return a.GetVersion().Compare(b)
	})

	isLatestRelease := relIdx == len(releases)-1

	// check previous release
	// only for awaiting purpose
	if relIdx > 0 {
		prevRelease := releases[relIdx-1]

		// if release version is greater in major or minor version than previous release
		// it must await for release Deployed state
		if (dr.GetVersion().Major() > prevRelease.GetVersion().Major() ||
			dr.GetVersion().Minor() > prevRelease.GetVersion().Minor()) &&
			prevRelease.GetPhase() != v1alpha1.DeckhouseReleasePhaseDeployed {
			msg := prevRelease.Status.Message
			if !strings.Contains(msg, "Awaiting") {
				msg = fmt.Sprintf("Awaiting for %s release to be deployed", prevRelease.GetVersion().String())
			}

			return &CalculatingResult{
				Order:   Await,
				Message: msg,
			}, nil
		}
	}

	// check next release
	// patch calculate logic
	if len(releases)-1 > relIdx {
		nextRelease := releases[relIdx+1]

		// if nextRelease version is greater in major or minor version
		// current release is definitely greatest at patch version
		if dr.GetVersion().Major() < nextRelease.GetVersion().Major() ||
			dr.GetVersion().Minor() < nextRelease.GetVersion().Minor() {
			return &CalculatingResult{
				Order:               Process,
				IsPatch:             true,
				IsLatest:            isLatestRelease,
				DeployedReleaseInfo: deployedReleaseInfo,
			}, nil
		}

		return &CalculatingResult{
			Order:   Skip,
			IsPatch: true,
		}, nil
	}

	// neighbours checks passed
	// only minor/major releases must be here
	return &CalculatingResult{
		Order:               Process,
		IsLatest:            isLatestRelease,
		DeployedReleaseInfo: deployedReleaseInfo,
	}, nil
}

func (p *OrderCalculator) listReleases(ctx context.Context) ([]v1alpha1.DeckhouseRelease, error) {
	var releases v1alpha1.DeckhouseReleaseList
	err := p.k8sclient.List(ctx, &releases)
	if err != nil {
		return nil, fmt.Errorf("get deckhouse releases: %w", err)
	}

	return releases.Items, nil
}

func (p *OrderCalculator) getDeployedReleaseInfo(releases []v1alpha1.DeckhouseRelease) *ReleaseInfo {
	deployedIdx := slices.IndexFunc(releases, func(a v1alpha1.DeckhouseRelease) bool {
		return a.Status.Phase == v1alpha1.DeckhouseReleasePhaseDeployed
	})

	if deployedIdx == -1 {
		return nil
	}

	deployedDR := releases[deployedIdx]

	return &ReleaseInfo{
		Name:    deployedDR.GetName(),
		Version: deployedDR.GetVersion(),
	}
}
