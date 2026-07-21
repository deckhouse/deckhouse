// Copyright 2025 Flant JSC
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

package source

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/project"
	"github.com/deckhouse/deckhouse/pkg/log"
)

func TestResolveEmbeddedTargetSource(t *testing.T) {
	const embedded = v1alpha1.ModuleSourceEmbedded

	tests := []struct {
		name             string
		chosenSource     string
		availableSources []string
		wantTarget       string
		wantConflict     bool
	}{
		{
			name:             "explicitly chosen source that is offered wins",
			chosenSource:     "deckhouse-upstream-ee",
			availableSources: []string{"deckhouse", "deckhouse-upstream-ee"},
			wantTarget:       "deckhouse-upstream-ee",
			wantConflict:     false,
		},
		{
			name:             "chosen source that is no longer offered is a conflict",
			chosenSource:     "gone",
			availableSources: []string{"deckhouse", "deckhouse-upstream-ee"},
			wantTarget:       "",
			wantConflict:     true,
		},
		{
			name:             "single real source is used",
			availableSources: []string{"deckhouse-upstream-ee"},
			wantTarget:       "deckhouse-upstream-ee",
			wantConflict:     false,
		},
		{
			// the case that produced the false-positive ModuleAtConflict alert
			name:             "deckhouse plus a mirror resolves to deckhouse, not a conflict",
			availableSources: []string{"deckhouse", "deckhouse-upstream-ee"},
			wantTarget:       "deckhouse",
			wantConflict:     false,
		},
		{
			name:             "source order does not matter, deckhouse still wins",
			availableSources: []string{"deckhouse-upstream-ee", "deckhouse"},
			wantTarget:       "deckhouse",
			wantConflict:     false,
		},
		{
			name:             "Embedded sentinel plus one real source is not a conflict",
			availableSources: []string{embedded, "deckhouse-upstream-ee"},
			wantTarget:       "deckhouse-upstream-ee",
			wantConflict:     false,
		},
		{
			name:             "Embedded plus deckhouse plus a mirror resolves to deckhouse",
			availableSources: []string{embedded, "deckhouse", "deckhouse-upstream-ee"},
			wantTarget:       "deckhouse",
			wantConflict:     false,
		},
		{
			name:             "only the Embedded sentinel is available - nothing to pre-stage, not a conflict",
			availableSources: []string{embedded},
			wantTarget:       "",
			wantConflict:     false,
		},
		{
			name:             "several non-default real sources with no selection is a genuine conflict",
			availableSources: []string{"vendor-a", "vendor-b"},
			wantTarget:       "",
			wantConflict:     true,
		},
		{
			name:             "no sources at all is not a conflict",
			availableSources: nil,
			wantTarget:       "",
			wantConflict:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, conflict := resolveEmbeddedTargetSource(tt.chosenSource, tt.availableSources)
			assert.Equal(t, tt.wantTarget, target, "target source")
			assert.Equal(t, tt.wantConflict, conflict, "conflict")
		})
	}
}

func TestReleaseChainToTargetComplete(t *testing.T) {
	const moduleName = "console"

	moduleRelease := func(version, phase string) *v1alpha1.ModuleRelease {
		return &v1alpha1.ModuleRelease{
			ObjectMeta: metav1.ObjectMeta{
				Name:   moduleName + "-v" + version,
				Labels: map[string]string{v1alpha1.ModuleReleaseLabelModule: moduleName},
			},
			Spec:   v1alpha1.ModuleReleaseSpec{ModuleName: moduleName, Version: version},
			Status: v1alpha1.ModuleReleaseStatus{Phase: phase},
		}
	}

	// moduleReleaseFromTo builds a release that declares a from-to transition rule on
	// itself (the constrained "to" release), allowing a direct jump from `from`.
	moduleReleaseFromTo := func(version, phase, from, to string) *v1alpha1.ModuleRelease {
		release := moduleRelease(version, phase)
		release.Spec.UpdateSpec = &v1alpha1.UpdateSpec{
			Versions: []v1alpha1.UpdateConstraint{{From: from, To: to}},
		}
		return release
	}

	tests := []struct {
		name     string
		target   string
		releases []*v1alpha1.ModuleRelease
		want     bool
		wantErr  bool
	}{
		{
			// the console case: deployed 1.52.0, target 1.55.1, intermediates missing
			name:   "gap between deployed and target",
			target: "v1.55.1",
			releases: []*v1alpha1.ModuleRelease{
				moduleRelease("1.52.0", v1alpha1.ModuleReleasePhaseDeployed),
				moduleRelease("1.55.1", v1alpha1.ModuleReleasePhasePending),
			},
			want: false,
		},
		{
			name:   "full continuous chain",
			target: "v1.55.1",
			releases: []*v1alpha1.ModuleRelease{
				moduleRelease("1.52.0", v1alpha1.ModuleReleasePhaseDeployed),
				moduleRelease("1.53.2", v1alpha1.ModuleReleasePhasePending),
				moduleRelease("1.54.1", v1alpha1.ModuleReleasePhasePending),
				moduleRelease("1.55.1", v1alpha1.ModuleReleasePhasePending),
			},
			want: true,
		},
		{
			// a from-to rule on the target legitimizes the minor jump: the chain is
			// complete and the fetch must NOT reopen on every reconcile
			name:   "gap bridged by target from-to rule",
			target: "v1.55.1",
			releases: []*v1alpha1.ModuleRelease{
				moduleRelease("1.52.0", v1alpha1.ModuleReleasePhaseDeployed),
				moduleReleaseFromTo("1.55.1", v1alpha1.ModuleReleasePhasePending, "1.52", "1.55"),
			},
			want: true,
		},
		{
			// NB: mirrors the fetcher's from-to leniency - a parseable from-to rule on
			// the higher release bridges the gap even when its [from,to) window does not
			// actually cover the deployed version (isUpdatingSequenceWithFromTo returns
			// nil on no-match). Kept consistent on purpose so the guard closes exactly
			// when the fetch would no-op; see the note in the review.
			name:   "from-to rule present bridges regardless of window",
			target: "v1.55.1",
			releases: []*v1alpha1.ModuleRelease{
				moduleRelease("1.52.0", v1alpha1.ModuleReleasePhaseDeployed),
				moduleReleaseFromTo("1.55.1", v1alpha1.ModuleReleasePhasePending, "1.53", "1.55"),
			},
			want: true,
		},
		{
			name:   "one intermediate minor missing",
			target: "v1.55.1",
			releases: []*v1alpha1.ModuleRelease{
				moduleRelease("1.52.0", v1alpha1.ModuleReleasePhaseDeployed),
				moduleRelease("1.53.2", v1alpha1.ModuleReleasePhasePending),
				moduleRelease("1.55.1", v1alpha1.ModuleReleasePhasePending),
			},
			want: false,
		},
		{
			name:   "target release itself missing",
			target: "v1.55.1",
			releases: []*v1alpha1.ModuleRelease{
				moduleRelease("1.52.0", v1alpha1.ModuleReleasePhaseDeployed),
				moduleRelease("1.53.2", v1alpha1.ModuleReleasePhasePending),
				moduleRelease("1.54.1", v1alpha1.ModuleReleasePhasePending),
			},
			want: false,
		},
		{
			name:     "no deployed release - first install, nothing to bridge",
			target:   "v1.55.1",
			releases: []*v1alpha1.ModuleRelease{moduleRelease("1.55.1", v1alpha1.ModuleReleasePhasePending)},
			want:     true,
		},
		{
			name:   "target not ahead of deployed",
			target: "v1.52.0",
			releases: []*v1alpha1.ModuleRelease{
				moduleRelease("1.52.0", v1alpha1.ModuleReleasePhaseDeployed),
			},
			want: true,
		},
		{
			name:   "sequential minor step is complete",
			target: "v1.53.2",
			releases: []*v1alpha1.ModuleRelease{
				moduleRelease("1.52.0", v1alpha1.ModuleReleasePhaseDeployed),
				moduleRelease("1.53.2", v1alpha1.ModuleReleasePhasePending),
			},
			want: true,
		},
		{
			// a corrupt Spec.Version must surface as a handled error, not panic through
			// GetVersion's semver.MustParse on this steady-state path
			name:   "malformed release version returns error",
			target: "v1.55.1",
			releases: []*v1alpha1.ModuleRelease{
				moduleRelease("1.52.0", v1alpha1.ModuleReleasePhaseDeployed),
				moduleRelease("not-a-semver", v1alpha1.ModuleReleasePhasePending),
			},
			wantErr: true,
		},
	}

	scheme, err := project.Scheme()
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := make([]client.Object, 0, len(tt.releases))
			for _, rel := range tt.releases {
				objects = append(objects, rel)
			}

			r := &reconciler{
				client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build(),
				logger: log.NewNop(),
			}

			got, err := r.releaseChainToTargetComplete(context.Background(), moduleName, tt.target)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
