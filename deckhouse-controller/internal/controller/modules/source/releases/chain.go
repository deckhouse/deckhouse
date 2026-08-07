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
	"errors"
	"fmt"
	"sort"

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	moduletypes "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader/types"
)

// ChainComplete reports whether the ModuleReleases already in the cluster form a continuous update
// sequence from the deployed release up to and including the target version. It returns true -
// nothing to bridge - when there is no deployed release yet or the target is not ahead of it; those
// are the regular first-install and no-op flows. A gap yields false, which makes Ensure run again.
//
// The gap rule is shared with Ensure through isSequentialPair, so this reports "complete" for
// exactly the chains Ensure would leave untouched, including the non-adjacent jumps a release's
// from-to spec legitimises. That equivalence is what stops the caller's checksum guard reopening
// the fetch on every reconcile for from-to modules.
func (s *Service) ChainComplete(ctx context.Context, moduleName, targetVersion string) (bool, error) {
	target, err := semver.NewVersion(targetVersion)
	if err != nil {
		return false, fmt.Errorf("parse target version %q: %w", targetVersion, err)
	}

	releases, err := s.list(ctx, moduleName)
	if err != nil {
		return false, err
	}

	// GetVersion, used below and inside isSequentialPair, relies on semver.MustParse and panics on
	// a malformed Spec.Version. This runs on the steady-state path for every installed module, so
	// every release is validated up front: one corrupt object must surface as a handled error, not
	// panic the reconcile.
	for _, release := range releases {
		if _, err = semver.NewVersion(release.Spec.Version); err != nil {
			return false, fmt.Errorf("parse release %q version %q: %w", release.Name, release.Spec.Version, err)
		}
	}

	deployed := latestDeployed(releases)
	if deployed == nil || !target.GreaterThan(deployed.GetVersion()) {
		return true, nil
	}

	// the deployed release plus everything in (deployed, target], in order
	chain := []*v1alpha1.ModuleRelease{deployed}
	for _, release := range releases {
		if version := release.GetVersion(); version.GreaterThan(deployed.GetVersion()) && !version.GreaterThan(target) {
			chain = append(chain, release)
		}
	}

	sort.Slice(chain, func(i, j int) bool {
		return chain[i].GetVersion().LessThan(chain[j].GetVersion())
	})

	// the target itself must close the chain
	if chain[len(chain)-1].GetVersion().Compare(target) != 0 {
		return false, nil
	}

	for i := 1; i < len(chain); i++ {
		if !isSequentialPair(chain[i-1], chain[i]) {
			return false, nil
		}
	}

	return true, nil
}

// latestDeployed returns the newest deployed release, or nil when none is deployed.
func latestDeployed(releases []*v1alpha1.ModuleRelease) *v1alpha1.ModuleRelease {
	var deployed *v1alpha1.ModuleRelease
	for _, release := range releases {
		if release.GetPhase() != v1alpha1.ModuleReleasePhaseDeployed {
			continue
		}

		if deployed == nil || release.GetVersion().GreaterThan(deployed.GetVersion()) {
			deployed = release
		}
	}

	return deployed
}

// isSequentialPair reports whether an update may proceed straight from the lower release to the
// higher one with nothing in between: either the versions are naturally adjacent, or the higher
// release declares a from-to rule that targets itself and admits the lower version.
//
// The from-to rule matches the one the release updater enforces before deploying a jump
// (releaseupdater.getFirstCompliantRelease). Keeping the two identical is what stops this package
// reporting a chain complete that the updater then refuses to walk - the mismatch that left a
// module stuck in Pending with a from-to whose "to" overshot the release's own minor.
func isSequentialPair(lower, higher *v1alpha1.ModuleRelease) bool {
	lowerVersion, higherVersion := lower.GetVersion(), higher.GetVersion()
	if isUpdatingSequence(lowerVersion, higherVersion) {
		return true
	}

	// the from-to rule is declared on the constrained ("to") release
	spec := higher.GetUpdateSpec()
	if spec == nil {
		return false
	}

	for _, constraint := range spec.Versions {
		if fromToBridges(lowerVersion, higherVersion, constraint) {
			return true
		}
	}

	return false
}

// fromToBridges reports whether one from-to constraint lets an update jump straight onto the higher
// release from the lower version: the constraint must target the higher release itself - its
// major.minor equals "to" - and the lower version must fall in the half-open window [from, to).
func fromToBridges(lower, higher *semver.Version, constraint v1alpha1.UpdateConstraint) bool {
	to, err := semver.NewVersion(constraint.To)
	if err != nil {
		return false
	}

	if higher.Major() != to.Major() || higher.Minor() != to.Minor() {
		return false
	}

	from, err := semver.NewVersion(constraint.From)
	if err != nil {
		return false
	}

	return lower.Compare(from) >= 0 && lower.Compare(to) < 0
}

// isUpdatingSequence reports whether an update may go straight from 'a' to 'b': neither the major
// nor the minor may skip a step. It is the cheap check that decides whether a registry listing is
// needed at all.
func isUpdatingSequence(a, b *semver.Version) bool {
	if a.Major()+1 < b.Major() {
		return false
	}

	if a.Minor()+1 < b.Minor() {
		return false
	}

	return true
}

// admittedByFromTo reports whether any of the module's from-to constraints covers the version,
// returning the parse failures it hit so a malformed constraint is visible rather than silent.
func admittedByFromTo(version *semver.Version, constraints []moduletypes.ModuleUpdateVersion) error {
	var errs error

	for _, constraint := range constraints {
		from, err := semver.NewVersion(constraint.From)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("parse constraint from '%s': %w", constraint.From, err))
			continue
		}

		to, err := semver.NewVersion(constraint.To)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("parse constraint to '%s': %w", constraint.To, err))
			continue
		}

		// the version falls in [from, to)
		if version.Compare(from) >= 0 && version.Compare(to) < 0 {
			return nil
		}
	}

	if errs != nil {
		return fmt.Errorf("parse constraint: %w", errs)
	}

	return nil
}
