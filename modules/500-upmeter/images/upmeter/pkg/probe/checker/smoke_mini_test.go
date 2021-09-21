/*
Copyright 2021 Flant JSC

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

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func Test_smokeMiniAvailable(t *testing.T) {
	// Running tests with race detector can lead to fail if the timeout is too small (e.g. 10ms)
	timeout := 25 * time.Millisecond

	s200 := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) { rw.WriteHeader(200) }))
	s500 := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) { rw.WriteHeader(500) }))
	slow200 := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		time.Sleep(timeout / 11 * 10) // ~91% of timeout
		rw.WriteHeader(200)
	}))
	tooSlow200 := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * timeout)
		rw.WriteHeader(200)
	}))

	logger := newDummyLogger()

	noError := func(t *testing.T, err error) {
		assert.NoError(t, err)
	}
	hasError := func(t *testing.T, err error) {
		assert.Error(t, err)
	}

	tests := []struct {
		name    string
		servers []*httptest.Server
		cancel  bool
		assert  func(*testing.T, error)
	}{
		{
			name:    "single responding server leads to success",
			servers: []*httptest.Server{s200},
			assert:  noError,
		},
		{
			name:    "single slowly responding server leads to success",
			servers: []*httptest.Server{slow200},
			assert:  noError,
		},
		{
			name:    "single error-responding server leads to error",
			servers: []*httptest.Server{s500},
			assert:  hasError,
		},
		{
			name:    "single hanging server leads to error",
			servers: []*httptest.Server{tooSlow200},
			assert:  hasError,
		},
		{
			name:    "fast 200 among failing servers leads to success",
			servers: []*httptest.Server{tooSlow200, s200, s500},
			assert:  noError,
		},
		{
			name:    "slow 200 among failing servers leads to success",
			servers: []*httptest.Server{tooSlow200, slow200, s500},
			assert:  noError,
		},
		{
			name:   "inexisintg endpoint leads to error",
			cancel: true,
			assert: hasError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := &smokeMiniChecker{
				path:        "/",
				httpTimeout: timeout,
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
