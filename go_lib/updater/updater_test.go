package updater

import (
	"testing"

	"github.com/deckhouse/deckhouse/go_lib/dependency"

	"github.com/stretchr/testify/require"
)

func TestDeployDelayReason(t *testing.T) {
	var (
		reason deployDelayReason
		now    = dependency.TestDC.GetClock().Now()
	)
	require.True(t, reason == noDelay)
	require.True(t, reason.contains(noDelay))

	reason = reason.add(outOfWindowReason)

	require.False(t, reason.contains(manualApprovalRequiredReason))
	require.False(t, reason.contains(noDelay))
	require.True(t, reason.contains(outOfWindowReason))
	require.Equal(t, "Release is waiting for the update window", reason.String())
	require.Equal(t, "Release is waiting for the update window until 17 Oct 19 15:33 UTC", reason.string(now))

	reason = reason.add(manualApprovalRequiredReason)
	require.True(t, reason.contains(manualApprovalRequiredReason))
	require.True(t, reason.contains(outOfWindowReason))
	require.Equal(t, "Release is waiting for the update window. Release is waiting for manual approval", reason.String())
	require.Equal(t, "Release is waiting for the update window. Release is waiting for manual approval. After approval the release will be delayed until 17 Oct 19 15:33 UTC", reason.string(now))
}
