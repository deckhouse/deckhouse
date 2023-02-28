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
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	kube "github.com/flant/kube-client/client"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"

	k8saccess "d8.io/upmeter/pkg/kubernetes"
)

func Test_smokeMiniAvailable(t *testing.T) {
	// Running tests with race detector can lead to fail if the timeout is too small (e.g. 10ms)

	var (
		logger  = newDummyLogger()
		timeout = 100 * time.Millisecond
		slow    = timeout / 11 * 10 // ~91%
		tooLate = 2 * timeout
	)

	noError := func(t *testing.T, err error) {
		assert.NoError(t, err)
	}
	hasError := func(t *testing.T, err error) {
		assert.Error(t, err)
	}

	tests := []struct {
		name    string
		servers []*httptest.Server
		cancel  bool // cause context cancelling by inexising endpoint
		assert  func(*testing.T, error)
	}{
		{
			name:    "single responding server leads to success",
			servers: []*httptest.Server{respondWith(200)},
			assert:  noError,
		},
		{
			name:    "single slowly responding server leads to success",
			servers: []*httptest.Server{respondSlowlyWith(slow, 200)},
			assert:  noError,
		},
		{
			name:    "single error-responding server leads to error",
			servers: []*httptest.Server{respondWith(500)},
			assert:  hasError,
		},
		{
			name:    "single hanging server leads to error",
			servers: []*httptest.Server{respondSlowlyWith(tooLate, 200)},
			assert:  hasError,
		},
		{
			name: "fast 200 among failing servers leads to success",
			servers: []*httptest.Server{
				respondSlowlyWith(tooLate, 200),
				respondWith(200),
				respondWith(500),
			},
			assert: noError,
		},
		{
			name: "slow 200 among failing servers leads to success",
			servers: []*httptest.Server{
				respondSlowlyWith(tooLate, 200),
				respondSlowlyWith(slow, 200),
				respondWith(500),
			},
			assert: noError,
		},
		{
			name:   "inexisintg endpoint leads to error",
			cancel: true,
			assert: hasError,
		},
		{
			name: "inexisintg endpoint with good server leads to success",
			servers: []*httptest.Server{
				respondSlowlyWith(slow, 200),
			},
			cancel: true,
			assert: noError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer goleak.VerifyNone(t,
				// klog
				goleak.IgnoreTopFunction("k8s.io/klog/v2.(*loggingT).flushDaemon"),
				// httputil.Server
				goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"),
			)

			defer func() {
				for _, s := range tt.servers {
					s.Close()
				}
			}()

			checker := &smokeMiniChecker{
				path:        "/",
				httpTimeout: timeout,
				access:      NewFake(kube.NewFake(nil)),
				lookuper: &dummyLookuper{
					servers:  tt.servers,
					addBogus: tt.cancel,
				},
				client: &http.Client{
					Transport: &http.Transport{
						MaxIdleConns:    5,
						MaxConnsPerHost: 1,
					},
				},
				logger: logger.WithField("test", tt.name),
			}

			tt.assert(t, checker.Check())
		})
	}
}

type dummyLookuper struct {
	servers  []*httptest.Server
	addBogus bool
}

func (l *dummyLookuper) Lookup() ([]string, error) {
	ips := make([]string, len(l.servers))
	for i, s := range l.servers {
		u, _ := url.Parse(s.URL)
		ips[i] = u.Host
	}
	if l.addBogus {
		ips = append(ips, "127.1.1.18:52876")
	}
	return ips, nil
}

func newDummyLogger() *logrus.Entry {
	logger := logrus.New()

	// logger.Level = logrus.DebugLevel
	logger.SetOutput(ioutil.Discard)

	return logrus.NewEntry(logger)
}

//nolint:unparam

func respondWith(status int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(status)
	}))
}

//nolint:unparam
func respondSlowlyWith(timeout time.Duration, status int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		time.Sleep(timeout)
		rw.WriteHeader(status)
	}))
}

func NewFake(client kube.Client) *FakeAccess {
	return &FakeAccess{client: client}
}

type FakeAccess struct {
	client kube.Client
}

func (a *FakeAccess) Kubernetes() kube.Client {
	return a.client
}

func (a *FakeAccess) ServiceAccountToken() string {
	return "pewpew"
}

func (a *FakeAccess) UserAgent() string {
	return "UpmeterTestClient/1.0"
}

func (a *FakeAccess) SchedulerProbeImage() *k8saccess.ProbeImage {
	return createTestProbeImage("test-image:latest", nil)
}

func (a *FakeAccess) SchedulerProbeNode() string {
	return ""
}

func (a *FakeAccess) CloudControllerManagerNamespace() string {
	return ""
}

func (a *FakeAccess) ClusterDomain() string {
	return ""
}
