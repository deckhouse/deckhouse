package control_plane

import (
	"upmeter/pkg/checks"
)

const groupName = "control-plane"

func LoadGroup() []*checks.Probe {
	return []*checks.Probe{
		NewAccessProbe(),
		NewBasicProbe(),
		NewNamespaceProbe(),
		NewControlPlaneManagerProbe(),
		NewSchedulerProbe(),
	}
}
