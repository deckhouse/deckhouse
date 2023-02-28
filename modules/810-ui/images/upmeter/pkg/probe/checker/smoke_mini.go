/*
Copyright 2023 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package checker

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/kubernetes"
)

// SmokeMiniAvailable is a checker constructor and configurator
type SmokeMiniAvailable struct {
	Path        string
	DnsTimeout  time.Duration
	HttpTimeout time.Duration
	Logger      *log.Entry
	Access      kubernetes.Access
}

func (s SmokeMiniAvailable) Checker() check.Checker {
	lkp := &nameLookuper{
		name:    "smoke-mini",
		port:    "8080",
		timeout: s.DnsTimeout,
	}

	// timeouts are maintained in request context
	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:    5,
			MaxConnsPerHost: 1,
		},
	}

	return &smokeMiniChecker{
		// dns
		lookuper: lkp,

		// http
		path:        s.Path,
		httpTimeout: s.HttpTimeout,
		client:      client,
		access:      s.Access,

		logger: s.Logger,
	}
}

// smokeMiniChecker checks that at least one smoke-mini pod responds with status 200
type smokeMiniChecker struct {
	// dns
	lookuper lookuper

	// http
	path        string
	httpTimeout time.Duration
	client      *http.Client
	access      kubernetes.Access

	logger *log.Entry
}

func (c *smokeMiniChecker) Check() check.Error {
	ips, lookupErr := c.lookuper.Lookup()
	if lookupErr != nil {
		return check.ErrUnknown("failed to resolve smoke-mini IPs: %v", lookupErr)
	}

	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), c.httpTimeout)

	wg.Add(len(ips))
	go func(wg *sync.WaitGroup) {
		// Sync requesting goroutines

		logger := c.logger.WithField("role", "syncer")

		logger.Debugf("waiting")
		wg.Wait()

		logger.Debugf("cancelling parent context")
		cancel()
	}(&wg)

	results := make(chan error)
	for _, ip := range ips {
		go func(wg *sync.WaitGroup, ip string) {
			defer wg.Done()

			// Wee need context per request, because among concurrent requests there
			// should not be single shared cancelling.
			rctx, rcancel := context.WithCancel(ctx)
			defer rcancel()

			logger := c.logger.WithField("ip", ip).WithField("role", "requester")
			logger.Debugf("requesting")

			err := c.request(rctx, ip)

			select {
			case results <- err:
			case <-rctx.Done():
				logger.Debugf("cancelled: %s", rctx.Err())
			}
		}(&wg, ip)
	}

	logger := c.logger.WithField("role", "collector")
	errs := make([]error, 0)
loop:
	for {
		select {
		case err := <-results:
			if err != nil {
				errs = append(errs, err)
				continue
			}
			logger.Debugf("success, cancelling parent context")
			return nil

		case <-ctx.Done():
			logger.Debugf("parent context cancelled: %v", ctx.Err())
			// Either all requests finished, or the time is out
			if err := ctx.Err(); err != nil {
				logger.WithError(err).Debugf("parent context error")
				errs = append(errs, err)
			}
			break loop
		}
	}

	// Report failure reasons
	msgs := make([]string, len(errs))
	for i, e := range errs {
		msgs[i] = e.Error()
	}
	return check.ErrFail("failed requests to smoke-mini: %s", strings.Join(msgs, ", "))
}

func (c *smokeMiniChecker) request(ctx context.Context, ip string) error {
	u := url.URL{
		Scheme: "http",
		Host:   ip,
		Path:   c.path,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, u.String(), nil)
	req.Header.Set("User-Agent", c.access.UserAgent())
	if err != nil {
		return err
	}

	res, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%s reponded with %d", u.String(), res.StatusCode)
	}

	return nil
}

// DnsAvailable is a checker constructor and configurator
type DnsAvailable struct {
	Domain     string
	DnsTimeout time.Duration
	Logger     *log.Entry
}

func (d DnsAvailable) Checker() check.Checker {
	lkp := &nameLookuper{
		name:    d.Domain,
		timeout: d.DnsTimeout,
	}
	return &dnsChecker{
		lookuper: lkp,
		logger:   d.Logger.WithField("domain", d.Domain),
	}
}

type dnsChecker struct {
	lookuper lookuper
	logger   *log.Entry
}

func (c *dnsChecker) Check() check.Error {
	_, err := c.lookuper.Lookup()
	if err != nil {
		return check.ErrFail("resolve: %w", err)
	}
	return nil
}

type lookuper interface {
	Lookup() ([]string, error)
}

type nameLookuper struct {
	name    string
	port    string
	timeout time.Duration
}

func (l *nameLookuper) Lookup() ([]string, error) {
	ips, err := lookupAndShuffleIPs(l.name, l.timeout)
	if err != nil {
		return ips, err
	}

	// append port to ips
	if l.port != "" {
		for i := range ips {
			ips[i] += ":" + l.port
		}
	}

	return ips, nil
}

// lookupAndShuffleIPs resolves IPs with timeout. It either returns nil and error, or non-empty
// slice of IPs and nil error.
func lookupAndShuffleIPs(name string, resolveTimeout time.Duration) ([]string, error) {
	// lookup
	ips, err := lookupIPs(name, resolveTimeout)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve '%s': %v", name, err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("resolved no addresses for '%s'", name)
	}

	// shuffle
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(ips), func(i, j int) { ips[i], ips[j] = ips[j], ips[i] })

	return ips, nil
}

func lookupIPs(domain string, timeout time.Duration) (ips []string, err error) {
	// If hostname is ip return it as is
	if isIP(domain) {
		ips = []string{domain}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	resolver := net.Resolver{}
	addrs, err := resolver.LookupIPAddr(ctx, domain)
	if err != nil {
		return
	}

	for _, addr := range addrs {
		ips = append(ips, addr.IP.String())
	}
	return ips, nil
}

func isIP(hostname string) bool {
	input := net.ParseIP(hostname)
	if input == nil || (input.To4() == nil && input.To16() == nil) {
		return false
	}
	return true
}
