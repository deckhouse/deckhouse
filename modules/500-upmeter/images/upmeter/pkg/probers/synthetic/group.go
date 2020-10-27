package synthetic

import "upmeter/pkg/probe/types"

const groupName = "synthetic"

var SmokeMiniAddr = "smoke-mini"

func LoadGroup() []types.Prober {
	return []types.Prober{
		NewAccessProber(),
		NewDnsProberSmokeCheck(),
		NewDnsProberInternalDomainCheck(),
		NewNeighborProber(),
		NewNeighborViaServiceProber(),
	}
}
