package synthetic

import (
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"math/rand"
	"net/http"
	"time"
	"upmeter/pkg/probers/util"
)

func ResolveSmokeMiniIps(addr string, resolveTimeout time.Duration) (ips []string, failed bool) {
	smokeIPs, err := util.LookupIPsWithTimeout(addr, resolveTimeout)
	if err != nil {
		log.Errorf("resolve '%s': %v", SmokeMiniAddr, err)
		//pr.ResultCh <- pr.ResultFail(accessProbeRef)
		return nil, true
	}
	if len(smokeIPs) == 0 {
		log.Errorf("resolve get 0 IPs for '%s'", SmokeMiniAddr)
		return nil, true
	}

	// randomize ips
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(smokeIPs), func(i, j int) { smokeIPs[i], smokeIPs[j] = smokeIPs[j], smokeIPs[i] })

	return smokeIPs, false
}

func RequestSmokeMiniUrl(ctx context.Context, ip string, path string) ([]byte, int, error) {
	if path == "" {
		path = "/"
	}
	smokeUrl := fmt.Sprintf("http://%s:8080%s", ip, path)

	req, err := http.NewRequest("GET", smokeUrl, nil)
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

	if resp != nil {
		defer resp.Body.Close()
		respData, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Debugf("cannot read body: %v", err)
			return nil, 0, nil
		}
		return respData, resp.StatusCode, nil
	}

	return nil, 0, nil
}
