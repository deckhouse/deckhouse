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
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

func TestDeployDelayReason(t *testing.T) {
	var (
		reason           DeployDelayReason
		deckhouseRelease *v1alpha1.DeckhouseRelease
		moduleRelease    *v1alpha1.ModuleRelease
		zeroTime         time.Time
		now              = dependency.TestDC.GetClock().Now()
	)

	require.True(t, reason == noDelay)
	require.True(t, reason.contains(noDelay))
	require.Equal(t, "noDelay", reason.String())
	require.Equal(t, "noDelay", reason.GoString())

	reason = reason.add(outOfWindowReason)
	require.False(t, reason.contains(noDelay))
	require.False(t, reason.contains(manualApprovalRequiredReason))
	require.True(t, reason.contains(outOfWindowReason))
	require.Equal(t, "outOfWindowReason", reason.String())
	require.Equal(t, "Release is waiting for the update window until 17 Oct 19 15:33 UTC", reason.Message(deckhouseRelease, now))
	require.Equal(t, "outOfWindowReason", reason.GoString())

	reason = reason.add(manualApprovalRequiredReason)
	require.True(t, reason.contains(manualApprovalRequiredReason))
	require.True(t, reason.contains(outOfWindowReason))
	require.Equal(t, "manualApprovalRequiredReason, outOfWindowReason", reason.String())
	require.Panics(t, func() { reason.Message(nil, zeroTime) })
	require.Equal(t, "Release is waiting for the update window, waiting for the 'release.deckhouse.io/approved: \"true\"' annotation", reason.Message(deckhouseRelease, zeroTime))
	require.Equal(t, "Release is waiting for the update window, waiting for the 'modules.deckhouse.io/approved: \"true\"' annotation", reason.Message(moduleRelease, zeroTime))
	require.Equal(t, "Release is waiting for the update window, waiting for the 'release.deckhouse.io/approved: \"true\"' annotation. After approval the release will be delayed until 17 Oct 19 15:33 UTC", reason.Message(deckhouseRelease, now))
	require.Equal(t, "Release is waiting for the update window, waiting for the 'modules.deckhouse.io/approved: \"true\"' annotation. After approval the release will be delayed until 17 Oct 19 15:33 UTC", reason.Message(moduleRelease, now))
	require.Equal(t, "manualApprovalRequiredReason|outOfWindowReason", reason.GoString())
}
