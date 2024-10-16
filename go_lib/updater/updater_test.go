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

package updater

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

func TestDeployDelayReason(t *testing.T) {
	var (
		reason   deployDelayReason
		zeroTime time.Time
		now      = dependency.TestDC.GetClock().Now()
	)
	require.True(t, reason == noDelay)
	require.True(t, reason.contains(noDelay))
	require.Equal(t, "noDelay", reason.String())
	require.Equal(t, "noDelay", reason.Message(zeroTime))
	require.Equal(t, "noDelay", reason.GoString())

	reason = reason.add(outOfWindowReason)

	require.False(t, reason.contains(noDelay))
	require.False(t, reason.contains(manualApprovalRequiredReason))
	require.True(t, reason.contains(outOfWindowReason))
	require.Equal(t, "outOfWindowReason", reason.String())
	require.Equal(t, "Release is waiting for the update window", reason.Message(zeroTime))
	require.Equal(t, "Release is waiting for the update window until 17 Oct 19 15:33 UTC", reason.Message(now))
	require.Equal(t, "outOfWindowReason", reason.GoString())

	reason = reason.add(manualApprovalRequiredReason)
	require.True(t, reason.contains(manualApprovalRequiredReason))
	require.True(t, reason.contains(outOfWindowReason))
	require.Equal(t, "outOfWindowReason and manualApprovalRequiredReason", reason.String())
	require.Equal(t, "Release is waiting for the update window, waiting for the 'release.deckhouse.io/approved: \"true\"' annotation", reason.Message(zeroTime))
	require.Equal(t, "Release is waiting for the update window, waiting for the 'release.deckhouse.io/approved: \"true\"' annotation. After approval the release will be delayed until 17 Oct 19 15:33 UTC", reason.Message(now))
	require.Equal(t, "outOfWindowReason|manualApprovalRequiredReason", reason.GoString())
}
