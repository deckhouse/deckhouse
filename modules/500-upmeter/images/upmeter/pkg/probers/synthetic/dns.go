package synthetic

import (
	"context"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"

	"upmeter/pkg/probe/types"
	"upmeter/pkg/probers/util"
)

/*
Checks run 5 times per second

CHECK 1:
Smoke-mini Pod that responds with code 200 via "/dns"
- Resolve IPs from DNS
- Randomize IPs list
- Request endpoints until first success

Period:                0.2 seconds
Dns resolve timeout:   0.1 seconds
Http response timeout: 0.1 seconds


CHECK 2:
Resolve 'kubernetes' domain.

Period:              0.2 seconds
Dns resolve timeout: 0.1 seconds
*/

var dnsProbeRef = types.ProbeRef{
	Group: groupName,
	Probe: "dns",
}

const (
	// To reach the probe launch frequency of 5 times per second, we need to
	// stick to the interval of 200 ms. Unfortunately, if the period is set to
	// 200 ms, about 60% of DNS probes take extra tick of 100 ms. The reason for
	// this is the timer inaccuracy. The timer corrects itself, and thus
	// sometimes desired 100ms intervals are longer or shorter. By subtracting
	// -20% of the period, we compensate the timer inaccuracy that seem to be
	// within a fraction of a percent of the timer interval (10s of microseconds).
	//
	// See https://github.com/golang/go/issues/14410#issuecomment-277413169
	dnsPeriod = 180 * time.Millisecond

	dnsProbeDNSTimeout  = 100 * time.Millisecond
	dnsProbeHTTPTimeout = 100 * time.Millisecond
)

func NewDnsProberSmokeCheck() types.Prober {
	pr := &types.CommonProbe{
		ProbeRef: &dnsProbeRef,
		Period:   dnsPeriod,
	}

	pr.RunFn = func() {
		// Resolve
		smokeIPs, found := LookupAndShuffleIPs(SmokeMiniAddr, dnsProbeDNSTimeout)
		if !found {
			pr.ResultCh <- pr.CheckResult("smoke", types.ProbeFailed)
			return
		}

		// Check that at least one pod responds
		logger := pr.LogEntry()
		success := checkSmokeMiniPodsAlive(smokeIPs, dnsProbeHTTPTimeout, logger)
		pr.ResultCh <- pr.CheckResult("smoke", success)
	}

	return pr
}

func NewDnsProberInternalDomainCheck() types.Prober {
	pr := &types.CommonProbe{
		ProbeRef: &dnsProbeRef,
		Period:   dnsPeriod,
	}

	pr.RunFn = func() {
		_, found := LookupAndShuffleIPs("kubernetes.default", dnsProbeDNSTimeout)
		pr.ResultCh <- pr.CheckResult("internal", found)
	}

	return pr
}

// checkSmokeMiniPodsAlive returns true if at least one of smoke-mini pods respond, returns false otherwise.
func checkSmokeMiniPodsAlive(ips []string, period time.Duration, logger *log.Entry) bool {
	success := false

	util.SequentialDoWithTimer(
		context.Background(),
		period,
		ips,
		func(ctx context.Context, _ int, ip string) int {
			_, status, err := RequestSmokeMiniUrl(ctx, ip, "/dns")
			if err != nil {
				// Go to next IP
				logger.Debugf("Request smoke mini '%s': %v", ip, err)
				return 0
			}

			if status == http.StatusOK {
				success = true
				// Break the loop
				return 1
			}
			return 0
		},
		func(index int, item string) {
			// The last smoke IP timed out, the check failed
			if index == len(ips)-1 {
				success = false
			}
		})

	return success
}
