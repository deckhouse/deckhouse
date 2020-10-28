package control_plane

import (
	"time"

	"upmeter/pkg/probe/types"
	"upmeter/pkg/probers/util"
)

/*
NewAccessProber

CHECK:
API server should be accessible.

Fetch /version endpoint from API server.

Period: 5 seconds.
HTTP request timeout: 5 seconds.
*/
func NewAccessProber() types.Prober {
	var accessProbeRef = types.ProbeRef{
		Group: groupName,
		Probe: "access",
	}
	const accessPeriod = 5
	const accessTimeout = 5 * time.Second

	pr := &types.CommonProbe{
		ProbeRef: &accessProbeRef,
		Period:   accessPeriod,
	}

	pr.RunFn = func(start int64) {
		log := pr.LogEntry()
		util.DoWithTimer(accessTimeout, func() {
			_, err := pr.KubernetesClient.Discovery().ServerVersion()
			if err != nil {
				log.Errorf("Get cluster version: %v type=%T", err, err)
				pr.ResultCh <- pr.Result(types.ProbeFailed)
			} else {
				pr.ResultCh <- pr.Result(types.ProbeSuccess)
			}
		}, func() {
			log.Infof("Exceeds timeout '%s' when fetch /version", accessTimeout.String())
			pr.ResultCh <- pr.Result(types.ProbeUnknown)
		})
	}

	return pr
}
