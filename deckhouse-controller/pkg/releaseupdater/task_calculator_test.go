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
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
)

func mkDR(name, ver, phase string) *v1alpha1.DeckhouseRelease {
	return &v1alpha1.DeckhouseRelease{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       v1alpha1.DeckhouseReleaseSpec{Version: ver},
		Status:     v1alpha1.DeckhouseReleaseStatus{Phase: phase},
	}
}

func TestReleaseQueueDepthDelta_MajorVersionChange(t *testing.T) {
	ctx := context.Background()

	// 0.66.0 (Deployed)
	// 1.0.0 (Pending) <- current (major version change 0->1, which is allowed)
	// Should have Major=1, Minor=0, Patch=0 and GetReleaseQueueDepth() should return 0
	releases := []v1alpha1.Release{
		mkDR("v0.66.0", "v0.66.0", v1alpha1.DeckhouseReleasePhaseDeployed),
		mkDR("v1.0.0", "v1.0.0", v1alpha1.DeckhouseReleasePhasePending),
	}

	current := releases[1]

	calc := &TaskCalculator{
		listFunc: func(_ context.Context, _ client.Client, _ string) ([]v1alpha1.Release, error) {
			return releases, nil
		},
		log:            log.NewNop(),
		releaseChannel: "",
	}

	task, err := calc.CalculatePendingReleaseTask(ctx, current)
	if err != nil {
		t.Fatalf("calculate task error: %v", err)
	}
	if task.TaskType != Process {
		t.Fatalf("unexpected task type: %v", task.TaskType)
	}
	if task.QueueDepth.Major != 1 {
		t.Fatalf("unexpected major delta: got %d, want %d", task.QueueDepth.Major, 1)
	}
	if task.QueueDepth.Minor != 0 {
		t.Fatalf("unexpected minor delta: got %d, want %d", task.QueueDepth.Minor, 0)
	}
	if task.QueueDepth.Patch != 0 {
		t.Fatalf("unexpected patch delta: got %d, want %d", task.QueueDepth.Patch, 0)
	}
	if task.QueueDepth.GetReleaseQueueDepth() != 0 {
		t.Fatalf("unexpected queue depth: got %d, want %d", task.QueueDepth.GetReleaseQueueDepth(), 0)
	}
}

func TestReleaseQueueDepthDelta_PatchVersionChange(t *testing.T) {
	ctx := context.Background()

	// 1.66.0 (Deployed)
	// 1.66.1 (Pending) <- current (patch version change)
	// Should have Major=0, Minor=0, Patch=1 and GetReleaseQueueDepth() should return 1
	releases := []v1alpha1.Release{
		mkDR("v1.66.0", "v1.66.0", v1alpha1.DeckhouseReleasePhaseDeployed),
		mkDR("v1.66.1", "v1.66.1", v1alpha1.DeckhouseReleasePhasePending),
	}

	current := releases[1]

	calc := &TaskCalculator{
		listFunc: func(_ context.Context, _ client.Client, _ string) ([]v1alpha1.Release, error) {
			return releases, nil
		},
		log:            log.NewNop(),
		releaseChannel: "",
	}

	task, err := calc.CalculatePendingReleaseTask(ctx, current)
	if err != nil {
		t.Fatalf("calculate task error: %v", err)
	}
	if task.TaskType != Process {
		t.Fatalf("unexpected task type: %v", task.TaskType)
	}
	if task.QueueDepth.Major != 0 {
		t.Fatalf("unexpected major delta: got %d, want %d", task.QueueDepth.Major, 0)
	}
	if task.QueueDepth.Minor != 0 {
		t.Fatalf("unexpected minor delta: got %d, want %d", task.QueueDepth.Minor, 0)
	}
	if task.QueueDepth.Patch != 1 {
		t.Fatalf("unexpected patch delta: got %d, want %d", task.QueueDepth.Patch, 1)
	}
	if task.QueueDepth.GetReleaseQueueDepth() != 1 {
		t.Fatalf("unexpected queue depth: got %d, want %d", task.QueueDepth.GetReleaseQueueDepth(), 1)
	}
}

func TestReleaseQueueDepthDelta_MinorVersionWithPatches(t *testing.T) {
	ctx := context.Background()

	// 1.66.5 (Deployed)
	// 1.67.2 (Pending) <- current (minor version change, but latest has patches)
	// Should have Major=0, Minor=1, Patch=0 (patches ignored when minor changes) and GetReleaseQueueDepth() should return 1
	releases := []v1alpha1.Release{
		mkDR("v1.66.5", "v1.66.5", v1alpha1.DeckhouseReleasePhaseDeployed),
		mkDR("v1.67.2", "v1.67.2", v1alpha1.DeckhouseReleasePhasePending),
	}

	current := releases[1]

	calc := &TaskCalculator{
		listFunc: func(_ context.Context, _ client.Client, _ string) ([]v1alpha1.Release, error) {
			return releases, nil
		},
		log:            log.NewNop(),
		releaseChannel: "",
	}

	task, err := calc.CalculatePendingReleaseTask(ctx, current)
	if err != nil {
		t.Fatalf("calculate task error: %v", err)
	}
	if task.TaskType != Process {
		t.Fatalf("unexpected task type: %v", task.TaskType)
	}
	if task.QueueDepth.Major != 0 {
		t.Fatalf("unexpected major delta: got %d, want %d", task.QueueDepth.Major, 0)
	}
	if task.QueueDepth.Minor != 1 {
		t.Fatalf("unexpected minor delta: got %d, want %d", task.QueueDepth.Minor, 1)
	}
	if task.QueueDepth.Patch != 0 {
		t.Fatalf("unexpected patch delta: got %d, want %d", task.QueueDepth.Patch, 0)
	}
	if task.QueueDepth.GetReleaseQueueDepth() != 1 {
		t.Fatalf("unexpected queue depth: got %d, want %d", task.QueueDepth.GetReleaseQueueDepth(), 1)
	}
}

func TestReleaseQueueDepthDelta_MajorVersionAwaiting(t *testing.T) {
	ctx := context.Background()

	// 1.66.0 (Deployed)
	// 2.0.0 (Pending) <- current (major version change 1->2, which should await)
	releases := []v1alpha1.Release{
		mkDR("v1.66.0", "v1.66.0", v1alpha1.DeckhouseReleasePhaseDeployed),
		mkDR("v2.0.0", "v2.0.0", v1alpha1.DeckhouseReleasePhasePending),
	}

	current := releases[1]

	calc := &TaskCalculator{
		listFunc: func(_ context.Context, _ client.Client, _ string) ([]v1alpha1.Release, error) {
			return releases, nil
		},
		log:            log.NewNop(),
		releaseChannel: "",
	}

	task, err := calc.CalculatePendingReleaseTask(ctx, current)
	if err != nil {
		t.Fatalf("calculate task error: %v", err)
	}
	if task.QueueDepth.Major != 1 {
		t.Fatalf("unexpected major delta: got %d, want %d", task.QueueDepth.Major, 1)
	}
	if task.QueueDepth.Minor != 0 {
		t.Fatalf("unexpected minor delta: got %d, want %d", task.QueueDepth.Minor, 0)
	}
	if task.QueueDepth.Patch != 0 {
		t.Fatalf("unexpected patch delta: got %d, want %d", task.QueueDepth.Patch, 0)
	}
	if task.QueueDepth.GetReleaseQueueDepth() != 0 {
		t.Fatalf("unexpected queue depth: got %d, want %d", task.QueueDepth.GetReleaseQueueDepth(), 0)
	}
}

func TestReleaseQueueDepthDelta_MajorVersionWithMinorUpdates(t *testing.T) {
	ctx := context.Background()

	// Should calculate Minor=5 (from 1.0 to 1.5) and Major=1 (from 1.x to 2.x)
	releases := []v1alpha1.Release{
		mkDR("v1.0.0", "v1.0.0", v1alpha1.DeckhouseReleasePhaseDeployed),
		mkDR("v1.1.0", "v1.1.0", v1alpha1.DeckhouseReleasePhasePending),
		mkDR("v1.2.0", "v1.2.0", v1alpha1.DeckhouseReleasePhasePending),
		mkDR("v1.3.5", "v1.3.5", v1alpha1.DeckhouseReleasePhasePending),
		mkDR("v1.3.6", "v1.3.6", v1alpha1.DeckhouseReleasePhasePending),
		mkDR("v1.3.7", "v1.3.7", v1alpha1.DeckhouseReleasePhasePending),
		mkDR("v1.4.8", "v1.4.8", v1alpha1.DeckhouseReleasePhasePending),
		mkDR("v1.5.10", "v1.5.10", v1alpha1.DeckhouseReleasePhasePending),
		mkDR("v2.5.0", "v2.5.0", v1alpha1.DeckhouseReleasePhasePending),
	}

	current := releases[1] // testing with 1.1.0 as current

	calc := &TaskCalculator{
		listFunc: func(_ context.Context, _ client.Client, _ string) ([]v1alpha1.Release, error) {
			return releases, nil
		},
		log:            log.NewNop(),
		releaseChannel: "",
	}

	task, err := calc.CalculatePendingReleaseTask(ctx, current)
	if err != nil {
		t.Fatalf("calculate task error: %v", err)
	}
	if task.TaskType != Process {
		t.Fatalf("unexpected task type: %v", task.TaskType)
	}
	if task.QueueDepth.Major != 1 {
		t.Fatalf("unexpected major delta: got %d, want %d", task.QueueDepth.Major, 1)
	}
	if task.QueueDepth.Minor != 5 {
		t.Fatalf("unexpected minor delta: got %d, want %d", task.QueueDepth.Minor, 5)
	}
	if task.QueueDepth.Patch != 0 {
		t.Fatalf("unexpected patch delta: got %d, want %d", task.QueueDepth.Patch, 0)
	}
	if task.QueueDepth.GetReleaseQueueDepth() != 5 {
		t.Fatalf("unexpected queue depth: got %d, want %d", task.QueueDepth.GetReleaseQueueDepth(), 5)
	}
}

func TestTaskCalculator_CalculatePendingReleaseTask(t *testing.T) {
	logger := log.NewNop()

	tests := []struct {
		name           string
		releaseChannel string
		releases       []v1alpha1.Release
		pendingRelease v1alpha1.Release
		expectedTask   *Task
		expectedError  error
	}{
		{
			name:           "release phase is not pending",
			releaseChannel: "stable",
			releases: []v1alpha1.Release{
				&mockRelease{name: "v1.67.0", version: "1.67.0", phase: v1alpha1.DeckhouseReleasePhaseDeployed},
			},
			pendingRelease: &mockRelease{name: "v1.67.0", version: "1.67.0", phase: v1alpha1.DeckhouseReleasePhaseDeployed},
			expectedError:  ErrReleasePhaseIsNotPending,
		},
		{
			name:           "single release",
			releaseChannel: "stable",
			releases: []v1alpha1.Release{
				&mockRelease{name: "v1.67.0", version: "1.67.0", phase: v1alpha1.DeckhouseReleasePhasePending},
			},
			pendingRelease: &mockRelease{name: "v1.67.0", version: "1.67.0", phase: v1alpha1.DeckhouseReleasePhasePending},
			expectedTask: &Task{
				TaskType:   Process,
				IsSingle:   true,
				IsLatest:   true,
				QueueDepth: &ReleaseQueueDepthDelta{},
			},
		},
		{
			name:           "forced release greater than pending",
			releaseChannel: "stable",
			releases: []v1alpha1.Release{
				&mockRelease{name: "v1.67.0", version: "1.67.0", phase: v1alpha1.DeckhouseReleasePhasePending},
				&mockRelease{name: "v1.68.0", version: "1.68.0", phase: v1alpha1.DeckhouseReleasePhasePending, force: true},
			},
			pendingRelease: &mockRelease{name: "v1.67.0", version: "1.67.0", phase: v1alpha1.DeckhouseReleasePhasePending},
			expectedTask: &Task{
				TaskType: Skip,
			},
		},
		{
			name:           "forced release less than pending - should process",
			releaseChannel: "stable",
			releases: []v1alpha1.Release{
				&mockRelease{name: "v1.67.0", version: "1.67.0", phase: v1alpha1.DeckhouseReleasePhasePending, force: true},
				&mockRelease{name: "v1.68.0", version: "1.68.0", phase: v1alpha1.DeckhouseReleasePhasePending},
			},
			pendingRelease: &mockRelease{name: "v1.68.0", version: "1.68.0", phase: v1alpha1.DeckhouseReleasePhasePending},
			expectedTask: &Task{
				TaskType:   Process,
				IsLatest:   true,
				QueueDepth: &ReleaseQueueDepthDelta{},
			},
		},
		{
			name:           "deployed release greater than pending",
			releaseChannel: "stable",
			releases: []v1alpha1.Release{
				&mockRelease{name: "v1.67.0", version: "1.67.0", phase: v1alpha1.DeckhouseReleasePhasePending},
				&mockRelease{name: "v1.68.0", version: "1.68.0", phase: v1alpha1.DeckhouseReleasePhaseDeployed},
			},
			pendingRelease: &mockRelease{name: "v1.67.0", version: "1.67.0", phase: v1alpha1.DeckhouseReleasePhasePending},
			expectedTask: &Task{
				TaskType: Skip,
			},
		},
		{
			name:           "deployed release equals pending",
			releaseChannel: "stable",
			releases: []v1alpha1.Release{
				&mockRelease{name: "v1.67.0", version: "1.67.0", phase: v1alpha1.DeckhouseReleasePhaseDeployed},
				&mockRelease{name: "v1.67.0", version: "1.67.0", phase: v1alpha1.DeckhouseReleasePhasePending},
			},
			pendingRelease: &mockRelease{name: "v1.67.0", version: "1.67.0", phase: v1alpha1.DeckhouseReleasePhasePending},
			expectedError:  ErrReleaseIsAlreadyDeployed,
		},
		{
			name:           "constraint endpoint - process as from-to",
			releaseChannel: "stable",
			releases: []v1alpha1.Release{
				&mockRelease{name: "v1.67.0", version: "1.67.0", phase: v1alpha1.DeckhouseReleasePhaseDeployed},
				&mockRelease{name: "v1.68.0", version: "1.68.0", phase: v1alpha1.DeckhouseReleasePhasePending},
				&mockRelease{name: "v1.69.0", version: "1.69.0", phase: v1alpha1.DeckhouseReleasePhasePending},
				&mockRelease{name: "v1.70.0", version: "1.70.0", phase: v1alpha1.DeckhouseReleasePhasePending, constraints: []v1alpha1.UpdateConstraint{{From: "1.67", To: "1.70"}}},
			},
			pendingRelease: &mockRelease{name: "v1.70.0", version: "1.70.0", phase: v1alpha1.DeckhouseReleasePhasePending, constraints: []v1alpha1.UpdateConstraint{{From: "1.67", To: "1.70"}}},
			expectedTask: &Task{
				TaskType: Process,
				IsFromTo: true,
				IsLatest: true,
				DeployedReleaseInfo: &ReleaseInfo{
					Name:    "v1.67.0",
					Version: semver.MustParse("1.67.0"),
				},
				QueueDepth: &ReleaseQueueDepthDelta{Major: 0, Minor: 3, Patch: 0},
			},
		},
		{
			name:           "await previous pending release",
			releaseChannel: "stable",
			releases: []v1alpha1.Release{
				&mockRelease{name: "v1.67.0", version: "1.67.0", phase: v1alpha1.DeckhouseReleasePhasePending},
				&mockRelease{name: "v1.68.0", version: "1.68.0", phase: v1alpha1.DeckhouseReleasePhasePending},
			},
			pendingRelease: &mockRelease{name: "v1.68.0", version: "1.68.0", phase: v1alpha1.DeckhouseReleasePhasePending},
			expectedTask: &Task{
				TaskType:   Await,
				Message:    "awaiting for v1.67.0 release to be deployed",
				QueueDepth: &ReleaseQueueDepthDelta{},
			},
		},
		{
			name:           "await major version jump (not 0->1)",
			releaseChannel: "stable",
			releases: []v1alpha1.Release{
				&mockRelease{name: "v1.67.0", version: "1.67.0", phase: v1alpha1.DeckhouseReleasePhaseDeployed},
				&mockRelease{name: "v2.0.0", version: "2.0.0", phase: v1alpha1.DeckhouseReleasePhasePending},
			},
			pendingRelease: &mockRelease{name: "v2.0.0", version: "2.0.0", phase: v1alpha1.DeckhouseReleasePhasePending},
			expectedTask: &Task{
				TaskType: Await,
				Message:  "major version is greater than deployed 1.67.0",
				IsMajor:  true,
				DeployedReleaseInfo: &ReleaseInfo{
					Name:    "v1.67.0",
					Version: semver.MustParse("1.67.0"),
				},
				QueueDepth: &ReleaseQueueDepthDelta{Major: 1, Minor: 0, Patch: 0},
			},
		},
		{
			name:           "allow major version jump 0->1",
			releaseChannel: "stable",
			releases: []v1alpha1.Release{
				&mockRelease{name: "v0.67.0", version: "0.67.0", phase: v1alpha1.DeckhouseReleasePhaseDeployed},
				&mockRelease{name: "v1.0.0", version: "1.0.0", phase: v1alpha1.DeckhouseReleasePhasePending},
			},
			pendingRelease: &mockRelease{name: "v1.0.0", version: "1.0.0", phase: v1alpha1.DeckhouseReleasePhasePending},
			expectedTask: &Task{
				TaskType: Process,
				IsLatest: true,
				DeployedReleaseInfo: &ReleaseInfo{
					Name:    "v0.67.0",
					Version: semver.MustParse("0.67.0"),
				},
				QueueDepth: &ReleaseQueueDepthDelta{Major: 1, Minor: 0, Patch: 0},
			},
		},
		{
			name:           "await minor version jump in regular channel",
			releaseChannel: "stable",
			releases: []v1alpha1.Release{
				&mockRelease{name: "v1.65.0", version: "1.65.0", phase: v1alpha1.DeckhouseReleasePhaseDeployed},
				&mockRelease{name: "v1.67.0", version: "1.67.0", phase: v1alpha1.DeckhouseReleasePhasePending},
			},
			pendingRelease: &mockRelease{name: "v1.67.0", version: "1.67.0", phase: v1alpha1.DeckhouseReleasePhasePending},
			expectedTask: &Task{
				TaskType: Await,
				Message:  "minor version is greater than deployed 1.65.0 by one",
				DeployedReleaseInfo: &ReleaseInfo{
					Name:    "v1.65.0",
					Version: semver.MustParse("1.65.0"),
				},
				QueueDepth: &ReleaseQueueDepthDelta{Major: 0, Minor: 2, Patch: 0},
			},
		},
		{
			name:           "allow minor version jump in LTS channel",
			releaseChannel: "lts",
			releases: []v1alpha1.Release{
				&mockRelease{name: "v1.65.0", version: "1.65.0", phase: v1alpha1.DeckhouseReleasePhaseDeployed},
				&mockRelease{name: "v1.67.0", version: "1.67.0", phase: v1alpha1.DeckhouseReleasePhasePending},
			},
			pendingRelease: &mockRelease{name: "v1.67.0", version: "1.67.0", phase: v1alpha1.DeckhouseReleasePhasePending},
			expectedTask: &Task{
				TaskType: Process,
				IsLatest: true,
				DeployedReleaseInfo: &ReleaseInfo{
					Name:    "v1.65.0",
					Version: semver.MustParse("1.65.0"),
				},
				QueueDepth: &ReleaseQueueDepthDelta{Major: 0, Minor: 2, Patch: 0},
			},
		},
		{
			name:           "await excessive minor version jump in LTS channel",
			releaseChannel: "lts",
			releases: []v1alpha1.Release{
				&mockRelease{name: "v1.65.0", version: "1.65.0", phase: v1alpha1.DeckhouseReleasePhaseDeployed},
				&mockRelease{name: "v1.76.0", version: "1.76.0", phase: v1alpha1.DeckhouseReleasePhasePending},
			},
			pendingRelease: &mockRelease{name: "v1.76.0", version: "1.76.0", phase: v1alpha1.DeckhouseReleasePhasePending},
			expectedTask: &Task{
				TaskType: Await,
				Message:  "minor version is greater than deployed 1.65.0 by 11, it's more than acceptable channel limitation",
				DeployedReleaseInfo: &ReleaseInfo{
					Name:    "v1.65.0",
					Version: semver.MustParse("1.65.0"),
				},
				QueueDepth: &ReleaseQueueDepthDelta{Major: 0, Minor: 11, Patch: 0},
			},
		},
		{
			name:           "skip patch release when higher patch exists",
			releaseChannel: "stable",
			releases: []v1alpha1.Release{
				&mockRelease{name: "v1.67.0", version: "1.67.0", phase: v1alpha1.DeckhouseReleasePhaseDeployed},
				&mockRelease{name: "v1.67.3", version: "1.67.3", phase: v1alpha1.DeckhouseReleasePhasePending},
				&mockRelease{name: "v1.67.5", version: "1.67.5", phase: v1alpha1.DeckhouseReleasePhasePending},
			},
			pendingRelease: &mockRelease{name: "v1.67.3", version: "1.67.3", phase: v1alpha1.DeckhouseReleasePhasePending},
			expectedTask: &Task{
				TaskType: Skip,
				IsPatch:  true,
			},
		},
		{
			name:           "process patch release when it's the latest",
			releaseChannel: "stable",
			releases: []v1alpha1.Release{
				&mockRelease{name: "v1.67.0", version: "1.67.0", phase: v1alpha1.DeckhouseReleasePhaseDeployed},
				&mockRelease{name: "v1.67.5", version: "1.67.5", phase: v1alpha1.DeckhouseReleasePhasePending},
			},
			pendingRelease: &mockRelease{name: "v1.67.5", version: "1.67.5", phase: v1alpha1.DeckhouseReleasePhasePending},
			expectedTask: &Task{
				TaskType: Process,
				IsPatch:  true,
				IsLatest: true,
				DeployedReleaseInfo: &ReleaseInfo{
					Name:    "v1.67.0",
					Version: semver.MustParse("1.67.0"),
				},
				QueueDepth: &ReleaseQueueDepthDelta{Major: 0, Minor: 0, Patch: 5},
			},
		},
		{
			name:           "process minor release when next is major",
			releaseChannel: "stable",
			releases: []v1alpha1.Release{
				&mockRelease{name: "v1.67.0", version: "1.67.0", phase: v1alpha1.DeckhouseReleasePhaseDeployed},
				&mockRelease{name: "v1.68.0", version: "1.68.0", phase: v1alpha1.DeckhouseReleasePhasePending},
				&mockRelease{name: "v2.0.0", version: "2.0.0", phase: v1alpha1.DeckhouseReleasePhasePending},
			},
			pendingRelease: &mockRelease{name: "v1.68.0", version: "1.68.0", phase: v1alpha1.DeckhouseReleasePhasePending},
			expectedTask: &Task{
				TaskType: Process,
				IsPatch:  false,
				IsLatest: false,
				DeployedReleaseInfo: &ReleaseInfo{
					Name:    "v1.67.0",
					Version: semver.MustParse("1.67.0"),
				},
				QueueDepth: &ReleaseQueueDepthDelta{Major: 1, Minor: 1, Patch: 0},
			},
		},
		{
			name:           "process sequential minor release",
			releaseChannel: "stable",
			releases: []v1alpha1.Release{
				&mockRelease{name: "v1.67.0", version: "1.67.0", phase: v1alpha1.DeckhouseReleasePhaseDeployed},
				&mockRelease{name: "v1.68.0", version: "1.68.0", phase: v1alpha1.DeckhouseReleasePhasePending},
			},
			pendingRelease: &mockRelease{name: "v1.68.0", version: "1.68.0", phase: v1alpha1.DeckhouseReleasePhasePending},
			expectedTask: &Task{
				TaskType: Process,
				IsLatest: true,
				IsPatch:  false,
				DeployedReleaseInfo: &ReleaseInfo{
					Name:    "v1.67.0",
					Version: semver.MustParse("1.67.0"),
				},
				QueueDepth: &ReleaseQueueDepthDelta{Major: 0, Minor: 1, Patch: 0},
			},
		},
		{
			name:           "constraint not applicable - deployed version too low",
			releaseChannel: "stable",
			releases: []v1alpha1.Release{
				&mockRelease{name: "v1.65.0", version: "1.65.0", phase: v1alpha1.DeckhouseReleasePhaseDeployed},
				&mockRelease{name: "v1.70.0", version: "1.70.0", phase: v1alpha1.DeckhouseReleasePhasePending, constraints: []v1alpha1.UpdateConstraint{{From: "1.67", To: "1.70"}}},
			},
			pendingRelease: &mockRelease{name: "v1.70.0", version: "1.70.0", phase: v1alpha1.DeckhouseReleasePhasePending, constraints: []v1alpha1.UpdateConstraint{{From: "1.67", To: "1.70"}}},
			expectedTask: &Task{
				TaskType: Await,
				Message:  "minor version is greater than deployed 1.65.0 by one",
				DeployedReleaseInfo: &ReleaseInfo{
					Name:    "v1.65.0",
					Version: semver.MustParse("1.65.0"),
				},
				QueueDepth: &ReleaseQueueDepthDelta{Major: 0, Minor: 5, Patch: 0},
			},
		},
		{
			name:           "constraint not applicable - deployed version too high",
			releaseChannel: "stable",
			releases: []v1alpha1.Release{
				&mockRelease{name: "v1.72.0", version: "1.72.0", phase: v1alpha1.DeckhouseReleasePhaseDeployed},
				&mockRelease{name: "v1.70.0", version: "1.70.0", phase: v1alpha1.DeckhouseReleasePhasePending, constraints: []v1alpha1.UpdateConstraint{{From: "1.67", To: "1.70"}}},
			},
			pendingRelease: &mockRelease{name: "v1.70.0", version: "1.70.0", phase: v1alpha1.DeckhouseReleasePhasePending, constraints: []v1alpha1.UpdateConstraint{{From: "1.67", To: "1.70"}}},
			expectedTask: &Task{
				TaskType: Skip,
			},
		},
		{
			name:           "suspended releases are ignored",
			releaseChannel: "stable",
			releases: []v1alpha1.Release{
				&mockRelease{name: "v1.70.17", version: "1.70.17", phase: v1alpha1.DeckhouseReleasePhaseDeployed},
				&mockRelease{name: "v1.71.5", version: "1.71.5", phase: "Suspended"},
				&mockRelease{name: "v1.71.7", version: "1.71.7", phase: v1alpha1.DeckhouseReleasePhasePending},
				&mockRelease{name: "v1.72.5", version: "1.72.5", phase: "Suspended"},
				&mockRelease{name: "v1.72.10", version: "1.72.10", phase: v1alpha1.DeckhouseReleasePhasePending},
			},
			pendingRelease: &mockRelease{name: "v1.72.10", version: "1.72.10", phase: v1alpha1.DeckhouseReleasePhasePending},
			expectedTask: &Task{
				TaskType: Await,
				Message:  "awaiting for v1.71.7 release to be deployed",
				DeployedReleaseInfo: &ReleaseInfo{
					Name:    "v1.70.17",
					Version: semver.MustParse("1.70.17"),
				},
				QueueDepth: &ReleaseQueueDepthDelta{Major: 0, Minor: 2, Patch: 0},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := NewDeckhouseReleaseTaskCalculator(nil, logger, tt.releaseChannel)

			// Mock the listFunc to return our test releases
			tc.listFunc = func(_ context.Context, _ client.Client, _ string) ([]v1alpha1.Release, error) {
				return tt.releases, nil
			}

			task, err := tc.CalculatePendingReleaseTask(context.Background(), tt.pendingRelease)

			if tt.expectedError != nil {
				require.Error(t, err)
				assert.Equal(t, tt.expectedError, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedTask.TaskType, task.TaskType)
			assert.Equal(t, tt.expectedTask.Message, task.Message)
			assert.Equal(t, tt.expectedTask.IsMajor, task.IsMajor)
			assert.Equal(t, tt.expectedTask.IsFromTo, task.IsFromTo)
			assert.Equal(t, tt.expectedTask.IsPatch, task.IsPatch)
			assert.Equal(t, tt.expectedTask.IsSingle, task.IsSingle)
			assert.Equal(t, tt.expectedTask.IsLatest, task.IsLatest)

			if tt.expectedTask.DeployedReleaseInfo != nil {
				require.NotNil(t, task.DeployedReleaseInfo)
				assert.Equal(t, tt.expectedTask.DeployedReleaseInfo.Name, task.DeployedReleaseInfo.Name)
				assert.True(t, tt.expectedTask.DeployedReleaseInfo.Version.Equal(task.DeployedReleaseInfo.Version))
			} else {
				assert.Nil(t, task.DeployedReleaseInfo)
			}

			if tt.expectedTask.QueueDepth != nil {
				require.NotNil(t, task.QueueDepth)
				assert.Equal(t, tt.expectedTask.QueueDepth.Major, task.QueueDepth.Major)
				assert.Equal(t, tt.expectedTask.QueueDepth.Minor, task.QueueDepth.Minor)
				assert.Equal(t, tt.expectedTask.QueueDepth.Patch, task.QueueDepth.Patch)
			} else {
				assert.Nil(t, task.QueueDepth)
			}
		})
	}
}

// mockRelease implements the v1alpha1.Release interface for testing
type mockRelease struct {
	name        string
	version     string
	phase       string
	force       bool
	constraints []v1alpha1.UpdateConstraint
}

func (m *mockRelease) GetName() string {
	return m.name
}

func (m *mockRelease) GetVersion() *semver.Version {
	v, _ := semver.NewVersion(m.version)
	return v
}

func (m *mockRelease) GetPhase() string {
	return m.phase
}

func (m *mockRelease) GetForce() bool {
	return m.force
}

func (m *mockRelease) GetModuleName() string {
	return ""
}

func (m *mockRelease) GetApplyAfter() *time.Time {
	return nil
}

func (m *mockRelease) GetRequirements() map[string]string {
	return nil
}

func (m *mockRelease) GetChangelogLink() string {
	return ""
}

func (m *mockRelease) GetDisruptions() []string {
	return nil
}

func (m *mockRelease) GetDisruptionApproved() bool {
	return false
}

func (m *mockRelease) GetReinstall() bool {
	return false
}

func (m *mockRelease) GetApplyNow() bool {
	return false
}

func (m *mockRelease) GetApprovedStatus() bool {
	return false
}

func (m *mockRelease) SetApprovedStatus(_ bool) {
	// no-op for mock
}

func (m *mockRelease) GetSuspend() bool {
	return false
}

func (m *mockRelease) GetManuallyApproved() bool {
	return false
}

func (m *mockRelease) GetMessage() string {
	return ""
}

func (m *mockRelease) GetNotified() bool {
	return false
}

func (m *mockRelease) GetUpdateSpec() *v1alpha1.UpdateSpec {
	if len(m.constraints) == 0 {
		return nil
	}
	return &v1alpha1.UpdateSpec{
		Versions: m.constraints,
	}
}
