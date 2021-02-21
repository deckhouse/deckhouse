package control_plane

import (
	"time"

	"upmeter/pkg/checks"
	"upmeter/pkg/probes/util"
)

/*
NewAccessProbe

CHECK:
API server should be accessible.

Fetch /version endpoint from API server.

Period: 5 seconds.
HTTP request timeout: 5 seconds.
*/
func NewAccessProbe() *checks.Probe {
	var accessProbeRef = checks.ProbeRef{
		Group: groupName,
		Probe: "access",
	}
	const accessPeriod = 5 * time.Second
	const accessTimeout = 5 * time.Second

	pr := &checks.Probe{
		Ref:    &accessProbeRef,
		Period: accessPeriod,
	}

	pr.RunFn = func() {
		log := pr.LogEntry()
		util.DoWithTimer(accessTimeout, func() {
			_, err := pr.KubernetesClient.Discovery().ServerVersion()
			if err != nil {
				log.Errorf("Get cluster version: %v type=%T", err, err)
				pr.ResultCh <- pr.Result(checks.StatusFail)
			} else {
				pr.ResultCh <- pr.Result(checks.StatusSuccess)
			}
		}, func() {
			log.Infof("Exceeds timeout '%s' when fetch /version", accessTimeout.String())
			pr.ResultCh <- pr.Result(checks.StatusUnknown)
		})
	}

	return pr
}
