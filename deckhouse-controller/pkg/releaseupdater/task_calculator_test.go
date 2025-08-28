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
		log:            log.NewLogger(),
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
		log:            log.NewLogger(),
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
		log:            log.NewLogger(),
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
		log:            log.NewLogger(),
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
		log:            log.NewLogger(),
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
