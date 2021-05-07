package entity

import (
	"d8.io/upmeter/pkg/check"
)

func FilterDisabledProbesFromGroupProbeList(probeRefs []check.ProbeRef) []check.ProbeRef {
	res := make([]check.ProbeRef, 0)

	for _, probeRef := range probeRefs {
		if check.IsProbeEnabled(probeRef.Id()) {
			res = append(res, probeRef)
		}
	}

	return res
}
