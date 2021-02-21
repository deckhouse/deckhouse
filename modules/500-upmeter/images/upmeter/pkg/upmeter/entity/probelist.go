package entity

import (
	"upmeter/pkg/checks"
)

func FilterDisabledProbesFromGroupProbeList(probeRefs []checks.ProbeRef) []checks.ProbeRef {
	res := make([]checks.ProbeRef, 0)

	for _, probeRef := range probeRefs {
		if checks.IsProbeEnabled(probeRef.Id()) {
			res = append(res, probeRef)
		}
	}

	return res
}
