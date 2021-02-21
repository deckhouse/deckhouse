package synthetic

import (
	"context"
	"net/http"
	"time"

	"upmeter/pkg/checks"
	"upmeter/pkg/probes/util"
)

/*
CHECK:
It should be at least one smoke-mini Pod that response
with code 200 via "/" endpoint.

Get IPs from DNS, randomize list and sequentially
request endpoint until first success.

Period: 5 seconds.
Dns resolve timeout: 2 seconds.
Http response timeout: 2 seconds.
*/

func NewAccessProbe() *checks.Probe {
	var accessProbeRef = checks.ProbeRef{
		Group: groupName,
		Probe: "access",
	}
	const accessPeriod = 5 * time.Second
	const accessDnsTimeout = 2 * time.Second
	const accessTimeout = 2 * time.Second

	pr := &checks.Probe{
		Ref:    &accessProbeRef,
		Period: accessPeriod,
	}

	pr.RunFn = func() {
		log := pr.LogEntry()

		smokeIPs, found := LookupAndShuffleIPs(SmokeMiniAddr, accessDnsTimeout)
		if !found {
			pr.ResultCh <- pr.Result(checks.StatusUnknown)
			return
		}

		success := false

		util.SequentialDoWithTimer(
			context.Background(),
			accessTimeout,
			smokeIPs,
			func(ctx context.Context, idx int, item string) int {
				_, status, err := RequestSmokeMiniUrl(ctx, item, "/")
				if err != nil {
					log.Infof("Request smoke mini '%s': %v", item, err)
					return 0
				}

				if status == http.StatusOK {
					success = true
					// Stop the loop
					return 1
				}
				return 0
			}, func(idx int, item string) {
				// The last smokeIp is timed out, send fail result.
				if idx == len(smokeIPs)-1 {
					pr.ResultCh <- pr.Result(checks.StatusFail)
				}
			})

		pr.ResultCh <- pr.Result(success)
	}

	return pr
}
