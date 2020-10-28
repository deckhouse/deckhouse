package entity

import (
	"upmeter/pkg/probe/types"
)

func FilterDisabledProbesFromGroupProbeList(probeRefs []types.ProbeRef) []types.ProbeRef {
	res := make([]types.ProbeRef, 0)

	for _, probeRef := range probeRefs {
		if types.IsProbeEnabled(probeRef.ProbeId()) {
			res = append(res, probeRef)
		}
	}

	return res
}
