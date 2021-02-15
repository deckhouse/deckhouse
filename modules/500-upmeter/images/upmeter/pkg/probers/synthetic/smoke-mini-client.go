package synthetic

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"

	"upmeter/pkg/probers/util"
)

// LookupAndShuffleIPs resolves IPs with timeout. The resulting `ips` slice has at least on e IP or equal nil if none found.
// At the same time `found` is true if at least one IP resolved, otherwise it is false.
func LookupAndShuffleIPs(addr string, resolveTimeout time.Duration) (ips []string, found bool) {
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
