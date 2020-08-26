package control_plane

import "upmeter/pkg/probe/types"

const groupName = "control-plane"

func LoadGroup() []types.Prober {
	return []types.Prober{
		NewAccessProber(),
		NewBasicProber(),
		NewNamespaceProber(),
		NewControlPlaneManagerProber(),
		NewSchedulerProber(),
	}
}
