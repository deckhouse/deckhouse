package synthetic

import (
	"upmeter/pkg/checks"
)

const groupName = "synthetic"

var SmokeMiniAddr = "smoke-mini"

func LoadGroup() []*checks.Probe {
	return []*checks.Probe{
		NewAccessProbe(),
		NewDnsProbeSmokeCheck(),
		NewDnsProbeInternalDomainCheck(),
		NewNeighborProbe(),
		NewNeighborViaServiceProbe(),
	}
}
