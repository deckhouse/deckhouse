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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
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
			// a from-to window that does not cover the deployed version does NOT bridge
			// the gap: the release updater refuses the jump (deployed < from), so the
			// chain must be reported incomplete and the intermediate releases fetched
			name:   "from-to window does not cover deployed - not bridged",
			target: "v1.55.1",
			releases: []*v1alpha1.ModuleRelease{
				moduleRelease("1.52.0", v1alpha1.ModuleReleasePhaseDeployed),
				moduleReleaseFromTo("1.55.1", v1alpha1.ModuleReleasePhasePending, "1.53", "1.55"),
			},
			want: false,
		},
		{
			// the reported incident: a from-to whose "to" (2.0) overshoots the release's
			// own minor (1.55). The release updater ignores such a rule (release
			// major.minor != to), so the source controller must too - report the chain
			// incomplete and fetch the missing 1.53/1.54 instead of freezing
			name:   "from-to to overshoots release minor - not bridged",
			target: "v1.55.1",
			releases: []*v1alpha1.ModuleRelease{
				moduleRelease("1.52.0", v1alpha1.ModuleReleasePhaseDeployed),
				moduleReleaseFromTo("1.55.1", v1alpha1.ModuleReleasePhasePending, "1.40", "2.0"),
			},
			want: false,
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

// TestSyncRegistrySettings covers the fan-out that re-applies changed registry settings to the
// deployed releases of a source. The release controller drops the annotation as soon as it has
// handled it, so it writes the very objects this fan-out writes; the cases below pin down that a
// collision there cannot cost progress, because a lost fan-out replays a moduleRun task per release.
func TestSyncRegistrySettings(t *testing.T) {
	const sourceName = "deckhouse"

	const sourceUID = types.UID("11111111-1111-1111-1111-111111111111")

	scheme, err := project.Scheme()
	require.NoError(t, err)

	newSource := func() *v1alpha1.ModuleSource {
		return &v1alpha1.ModuleSource{
			ObjectMeta: metav1.ObjectMeta{
				Name: sourceName,
				UID:  sourceUID,
				// a stale checksum, so the fan-out is due
				Annotations: map[string]string{v1alpha1.ModuleSourceAnnotationRegistryChecksum: "stale"},
			},
			Spec: v1alpha1.ModuleSourceSpec{
				Registry: v1alpha1.ModuleSourceSpecRegistry{Repo: "registry.example.com/modules", Scheme: "HTTPS"},
			},
		}
	}

	newRelease := func(name, phase string, owned bool) *v1alpha1.ModuleRelease {
		release := &v1alpha1.ModuleRelease{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: map[string]string{v1alpha1.ModuleReleaseLabelSource: sourceName},
			},
			Spec:   v1alpha1.ModuleReleaseSpec{ModuleName: name},
			Status: v1alpha1.ModuleReleaseStatus{Phase: phase},
		}

		if owned {
			release.OwnerReferences = []metav1.OwnerReference{{
				Kind: v1alpha1.ModuleSourceGVK.Kind,
				Name: sourceName,
				UID:  sourceUID,
			}}
		}

		return release
	}

	annotationOf := func(t *testing.T, cl client.Client, name string) (string, bool) {
		t.Helper()

		release := new(v1alpha1.ModuleRelease)
		require.NoError(t, cl.Get(context.Background(), client.ObjectKey{Name: name}, release))
		value, set := release.GetAnnotations()[v1alpha1.ModuleReleaseAnnotationRegistrySpecChanged]

		return value, set
	}

	t.Run("annotates deployed releases of the source and commits the checksum", func(t *testing.T) {
		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
			newRelease("console", v1alpha1.ModuleReleasePhaseDeployed, true),
			newRelease("observability", v1alpha1.ModuleReleasePhaseDeployed, true),
			newRelease("prompp", v1alpha1.ModuleReleasePhasePending, true),
			newRelease("foreign", v1alpha1.ModuleReleasePhaseDeployed, false),
		).Build()

		r := &reconciler{client: cl, dc: dependency.NewDependencyContainer(), logger: log.NewNop()}

		source := newSource()
		require.NoError(t, r.syncRegistrySettings(context.Background(), source))

		for _, name := range []string{"console", "observability"} {
			_, set := annotationOf(t, cl, name)
			assert.True(t, set, "deployed release %q of the source must be annotated", name)
		}

		for _, name := range []string{"prompp", "foreign"} {
			_, set := annotationOf(t, cl, name)
			assert.False(t, set, "release %q is not a deployed release of the source", name)
		}

		assert.NotEqual(t, "stale", source.GetAnnotations()[v1alpha1.ModuleSourceAnnotationRegistryChecksum],
			"a complete fan-out must commit the new checksum, otherwise the next resync replays it")
	})

	t.Run("nothing to do when the checksum already matches", func(t *testing.T) {
		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
			newRelease("console", v1alpha1.ModuleReleasePhaseDeployed, true),
		).Build()

		r := &reconciler{client: cl, dc: dependency.NewDependencyContainer(), logger: log.NewNop()}

		// prime the checksum by running a full sync first
		source := newSource()
		require.NoError(t, r.syncRegistrySettings(context.Background(), source))

		require.ErrorIs(t, r.syncRegistrySettings(context.Background(), source), ErrSettingsNotChanged)
	})

	t.Run("annotates through a patch, never an update", func(t *testing.T) {
		// An update carries a resourceVersion and therefore races the release controller, which
		// writes the same releases to drop the annotation. Losing that race used to cost the whole
		// fan-out, so this path must not go anywhere near an update.
		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
			newRelease("console", v1alpha1.ModuleReleasePhaseDeployed, true),
			newRelease("observability", v1alpha1.ModuleReleasePhaseDeployed, true),
		).WithInterceptorFuncs(interceptor.Funcs{
			Update: func(_ context.Context, _ client.WithWatch, obj client.Object, _ ...client.UpdateOption) error {
				return apierrors.NewConflict(v1alpha1.ModuleReleaseGVR.GroupResource(), obj.GetName(),
					errors.New("the object has been modified"))
			},
		}).Build()

		r := &reconciler{client: cl, dc: dependency.NewDependencyContainer(), logger: log.NewNop()}

		source := newSource()
		require.NoError(t, r.syncRegistrySettings(context.Background(), source))

		for _, name := range []string{"console", "observability"} {
			_, set := annotationOf(t, cl, name)
			assert.True(t, set, "release %q must be annotated", name)
		}

		assert.NotEqual(t, "stale", source.GetAnnotations()[v1alpha1.ModuleSourceAnnotationRegistryChecksum])
	})

	t.Run("a release deleted meanwhile is not a failure", func(t *testing.T) {
		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
			newRelease("console", v1alpha1.ModuleReleasePhaseDeployed, true),
			newRelease("observability", v1alpha1.ModuleReleasePhaseDeployed, true),
		).WithInterceptorFuncs(interceptor.Funcs{
			Patch: func(ctx context.Context, cl client.WithWatch, obj client.Object, p client.Patch, opts ...client.PatchOption) error {
				if obj.GetName() == "console" {
					return apierrors.NewNotFound(v1alpha1.ModuleReleaseGVR.GroupResource(), obj.GetName())
				}

				return cl.Patch(ctx, obj, p, opts...)
			},
		}).Build()

		r := &reconciler{client: cl, dc: dependency.NewDependencyContainer(), logger: log.NewNop()}

		source := newSource()
		require.NoError(t, r.syncRegistrySettings(context.Background(), source),
			"a release that no longer exists has nothing to pick the settings up for")

		assert.NotEqual(t, "stale", source.GetAnnotations()[v1alpha1.ModuleSourceAnnotationRegistryChecksum])
	})

	t.Run("one unwritable release does not hold back the others", func(t *testing.T) {
		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
			newRelease("aaa-first", v1alpha1.ModuleReleasePhaseDeployed, true),
			newRelease("bbb-broken", v1alpha1.ModuleReleasePhaseDeployed, true),
			newRelease("ccc-last", v1alpha1.ModuleReleasePhaseDeployed, true),
		).WithInterceptorFuncs(interceptor.Funcs{
			Patch: func(ctx context.Context, cl client.WithWatch, obj client.Object, p client.Patch, opts ...client.PatchOption) error {
				if obj.GetName() == "bbb-broken" {
					return errors.New("boom")
				}

				return cl.Patch(ctx, obj, p, opts...)
			},
		}).Build()

		r := &reconciler{client: cl, dc: dependency.NewDependencyContainer(), logger: log.NewNop()}

		source := newSource()
		err := r.syncRegistrySettings(context.Background(), source)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bbb-broken")

		// the release listed after the broken one must still have been reached
		for _, name := range []string{"aaa-first", "ccc-last"} {
			_, set := annotationOf(t, cl, name)
			assert.True(t, set, "release %q must be annotated despite a failure on another release", name)
		}

		assert.Equal(t, "stale", source.GetAnnotations()[v1alpha1.ModuleSourceAnnotationRegistryChecksum],
			"an incomplete fan-out must not commit the checksum, or the failed release keeps stale settings")
	})

	t.Run("an annotation left over from an earlier change is refreshed", func(t *testing.T) {
		const earlier = "2006-01-02T15:04:05Z"

		release := newRelease("console", v1alpha1.ModuleReleasePhaseDeployed, true)
		release.Annotations = map[string]string{v1alpha1.ModuleReleaseAnnotationRegistrySpecChanged: earlier}

		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(release).Build()

		r := &reconciler{client: cl, dc: dependency.NewDependencyContainer(), logger: log.NewNop()}

		require.NoError(t, r.syncRegistrySettings(context.Background(), newSource()))

		value, set := annotationOf(t, cl, "console")
		require.True(t, set)
		assert.NotEqual(t, earlier, value, "the stamp must move, so the release is not left on an old change")
	})
}
