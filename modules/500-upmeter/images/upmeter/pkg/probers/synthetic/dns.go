package synthetic

import (
	"context"
	"net/http"
	"time"

	"upmeter/pkg/probe/types"
	"upmeter/pkg/probers/util"
)

/*
CHECK 1:
It should be at least one smoke-mini Pod that response
with code 200 via "/dns" endpoint.

Get IPs from DNS, randomize list and sequentially
request endpoint until first success.

Period: 5 seconds.
Dns resolve timeout: 2 seconds.
Http response timeout: 4 seconds.


CHECK 2:
'kubernetes' domain should resolve.

Period: 5 sec
Dns resolve timeout: 2 sec
*/

var dnsProbeRef = types.ProbeRef{
	Group: groupName,
	Probe: "dns",
}

const dnsPeriod = 5

func NewDnsProberSmokeCheck() types.Prober {
	const dnsSmokeDnsTimeout = 2 * time.Second
	const dnsSmokeTimeout = 2 * time.Second

	pr := &types.CommonProbe{
		ProbeRef: &dnsProbeRef,
		Period:   dnsPeriod,
	}

	pr.RunFn = func(start int64) {
		log := pr.LogEntry()

		smokeIPs, failed := ResolveSmokeMiniIps(SmokeMiniAddr, dnsSmokeDnsTimeout)
		if failed {
			pr.ResultCh <- pr.CheckResult("smoke", types.ProbeFailed)
			return
		}

		success := false

		util.SequentialDoWithTimer(
			context.Background(),
			dnsSmokeTimeout,
			smokeIPs,
			func(ctx context.Context, idx int, item string) int {
				_, status, err := RequestSmokeMiniUrl(ctx, item, "/dns")
				if err != nil {
					log.Debugf("Request smoke mini '%s': %v", item, err)
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
					pr.ResultCh <- pr.CheckResult("smoke", types.ProbeFailed)
				}
			})

		pr.ResultCh <- pr.CheckResult("smoke", success)
	}

	return pr
}

func NewDnsProberInternalDomainCheck() types.Prober {
	const dnsKubeDnsTimeout = 2 * time.Second

	pr := &types.CommonProbe{
		ProbeRef: &dnsProbeRef,
		Period:   dnsPeriod,
	}

	pr.RunFn = func(start int64) {
		_, failed := ResolveSmokeMiniIps("kubernetes", dnsKubeDnsTimeout)
		pr.ResultCh <- pr.CheckResult("internal", !failed)
	}

	return pr
}
