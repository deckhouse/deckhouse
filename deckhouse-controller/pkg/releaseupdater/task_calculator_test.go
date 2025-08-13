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

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
)

func TestConstraintsSkipIntermediates(t *testing.T) {
	rDeployed := &v1alpha1.ModuleRelease{Spec: v1alpha1.ModuleReleaseSpec{ModuleName: "demo", Version: "1.67.5"}, Status: v1alpha1.ModuleReleaseStatus{Phase: v1alpha1.ModuleReleasePhaseDeployed}}
	r168 := &v1alpha1.ModuleRelease{Spec: v1alpha1.ModuleReleaseSpec{ModuleName: "demo", Version: "1.68.4"}, Status: v1alpha1.ModuleReleaseStatus{Phase: v1alpha1.ModuleReleasePhasePending}}
	r169 := &v1alpha1.ModuleRelease{Spec: v1alpha1.ModuleReleaseSpec{ModuleName: "demo", Version: "1.69.10"}, Status: v1alpha1.ModuleReleaseStatus{Phase: v1alpha1.ModuleReleasePhasePending}}
	r170 := &v1alpha1.ModuleRelease{Spec: v1alpha1.ModuleReleaseSpec{ModuleName: "demo", Version: "1.70.11"}, Status: v1alpha1.ModuleReleaseStatus{Phase: v1alpha1.ModuleReleasePhasePending}}
	r175 := &v1alpha1.ModuleRelease{Spec: v1alpha1.ModuleReleaseSpec{ModuleName: "demo", Version: "1.75.2", UpdateConstraints: &v1alpha1.ModuleUpdateConstraints{Versions: []v1alpha1.ModuleUpdateConstraint{{From: "1.67", To: "1.75"}}}}, Status: v1alpha1.ModuleReleaseStatus{Phase: v1alpha1.ModuleReleasePhasePending}}

	releases := []v1alpha1.Release{rDeployed, r168, r169, r170, r175}

	tc := &TaskCalculator{
		listFunc: func(_ context.Context, _ client.Client, _ string) ([]v1alpha1.Release, error) {
			return releases, nil
		},
		log: log.NewNop(),
	}

	// 1.68.4 should be skipped
	task, err := tc.CalculatePendingReleaseTask(context.Background(), r168)
	require.NoError(t, err)
	require.Equal(t, Skip, task.TaskType)

	// 1.75.2 should be processed as endpoint (minor)
	task, err = tc.CalculatePendingReleaseTask(context.Background(), r175)
	require.NoError(t, err)
	require.Equal(t, Process, task.TaskType)
	require.False(t, task.IsPatch)
}
