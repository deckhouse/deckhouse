package checker

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/probe/util"
)

// SmokeMiniAvailable is a checker constructor and configurator
type SmokeMiniAvailable struct {
	Path        string
	DnsTimeout  time.Duration
	HttpTimeout time.Duration
}

func (s SmokeMiniAvailable) Checker() check.Checker {
	return &smokeMiniChecker{
		path:        s.Path,
		dnsTimeout:  s.DnsTimeout,
		httpTimeout: s.HttpTimeout,
	}
}

type smokeMiniChecker struct {
	path        string
	dnsTimeout  time.Duration
	httpTimeout time.Duration
}

func (c *smokeMiniChecker) BusyWith() string {
	return "requesting smoke-mini " + c.path
}

func (c *smokeMiniChecker) Check() check.Error {
	const hostname = "smoke-mini"

	smokeIPs, found := lookupAndShuffleIPs(hostname, c.dnsTimeout)
	if !found {
		return check.ErrUnknown("no smoke IPs found")
	}

	var err check.Error

	util.SequentialDoWithTimer(
		context.Background(),
		c.httpTimeout,
		smokeIPs,
		func(ctx context.Context, idx int, item string) int {
			_, status, reqerr := requestSmokeMiniEndpoint(ctx, item, c.path)
			if reqerr != nil {
				err = check.ErrFail("requesting smoke-mini '%s': %v", item, reqerr)
				return 0
			}

			if status == http.StatusOK {
				// Stop the loop
				return 1
			}

			return 0
		}, func(idx int, item string) {
			// The last smokeIp is timed out, send fail result.
			if idx == len(smokeIPs)-1 {
				err = check.ErrFail("requesting smoke-mini %s timed out", c.path)
			}
		})

	return err
}

// DnsAvailable is a checker constructor and configurator
type DnsAvailable struct {
	Domain     string
	DnsTimeout time.Duration
}

func (d DnsAvailable) Checker() check.Checker {
	return &dnsChecker{
		domain:     d.Domain,
		dnsTimeout: d.DnsTimeout,
	}
}

type dnsChecker struct {
	domain     string
	dnsTimeout time.Duration
}

func (c dnsChecker) BusyWith() string {
	return "resolving " + c.domain
}

func (c dnsChecker) Check() check.Error {
	_, found := lookupAndShuffleIPs(c.domain, c.dnsTimeout)
	if !found {
		return check.ErrFail("cannot resolve %s", c.domain)
	}
	return nil
}

// lookupAndShuffleIPs resolves IPs with timeout. The resulting `ips` slice has at least on e IP or equal nil if none found.
// At the same time `found` is true if at least one IP resolved, otherwise it is false.
func lookupAndShuffleIPs(addr string, resolveTimeout time.Duration) (ips []string, found bool) {
	ips, err := util.LookupIPsWithTimeout(addr, resolveTimeout)
	if err != nil {
		log.Errorf("resolve '%s': %v", addr, err)
		return nil, false
	}

	if len(ips) == 0 {
		log.Errorf("resolve get 0 IPs for '%s'", addr)
		return nil, false
	}

	// randomize ips
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(ips), func(i, j int) { ips[i], ips[j] = ips[j], ips[i] })

	return ips, true
}

func requestSmokeMiniEndpoint(ctx context.Context, ip, path string) ([]byte, int, error) {
	if path == "" {
		path = "/"
	}
	smokeUrl := fmt.Sprintf("http://%s:8080%s", ip, path)

	req, err := http.NewRequest(http.MethodGet, smokeUrl, nil)
	if err != nil {
		log.Errorf("Create GET request: %v", err)
		return nil, 0, err
	}
	req = req.WithContext(ctx)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Debugf("Do GET request: %v", err)
		return nil, 0, err
	}

	if resp == nil {
		return nil, 0, nil
	}

	defer resp.Body.Close()

	respData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Debugf("cannot read body: %v", err)
		return nil, 0, nil
	}
	return respData, resp.StatusCode, nil
}
