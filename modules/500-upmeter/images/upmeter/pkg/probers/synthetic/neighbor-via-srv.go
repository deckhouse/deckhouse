package synthetic

import (
	"context"
	"net/http"
	"time"

	"upmeter/pkg/probe/types"
	"upmeter/pkg/probers/util"
)

/*
CHECK:
It should be at least one smoke-mini Pod that response
with code 200 via "/neighbor-via-service" endpoint.

Get IPs from DNS, randomize list and sequentially
request endpoint until first success.

Period: 5 seconds.
Dns resolve timeout: 2 seconds.
Http response timeout: 4 seconds.

*/

func NewNeighborViaServiceProber() types.Prober {
	var nghsrvProbeRef = types.ProbeRef{
		Group: groupName,
		Probe: "neighbor-via-service",
	}
	const nghsrvPeriod = 5
	const nghsrvDnsTimeout = 2 * time.Second
	const nghsrvTimeout = 4 * time.Second

	pr := &types.CommonProbe{
		ProbeRef: &nghsrvProbeRef,
		Period:   nghsrvPeriod,
	}

	pr.RunFn = func(start int64) {
		log := pr.LogEntry()

		smokeIPs, failed := ResolveSmokeMiniIps(SmokeMiniAddr, nghsrvDnsTimeout)
		if failed {
			pr.ResultCh <- pr.Result(types.ProbeFailed)
			return
		}

		success := false

		util.SequentialDoWithTimer(
			context.Background(),
			nghsrvTimeout,
			smokeIPs,
			func(ctx context.Context, idx int, item string) int {
				_, status, err := RequestSmokeMiniUrl(ctx, item, "/neighbor-via-service")
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
					pr.ResultCh <- pr.Result(types.ProbeFailed)
				}
			})

		pr.ResultCh <- pr.Result(success)
	}

	return pr
}
