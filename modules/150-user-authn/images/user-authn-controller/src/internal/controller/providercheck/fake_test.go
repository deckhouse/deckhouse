/*
Copyright 2026 Flant JSC

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

package providercheck

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net/http"
	"sync"
	"time"
)

type fakeHTTP struct {
	mu      sync.Mutex
	routes  map[string]fakeResponse
	calls   int
	lastURL []string
	delay   time.Duration
}

type fakeResponse struct {
	status int
	body   []byte
	err    error
}

func newFakeHTTP() *fakeHTTP {
	return &fakeHTTP{routes: map[string]fakeResponse{}}
}

func (f *fakeHTTP) set(rawURL string, status int, body string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.routes[rawURL] = fakeResponse{status: status, body: []byte(body)}
}

func (f *fakeHTTP) setErr(rawURL string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.routes[rawURL] = fakeResponse{err: err}
}

func (f *fakeHTTP) New(TLSOptions) (HTTPDoer, error) {
	return f, nil
}

func (f *fakeHTTP) Do(req *http.Request) (*http.Response, error) {
	f.mu.Lock()
	delay := f.delay
	f.mu.Unlock()
	if delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case <-timer.C:
		}
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.lastURL = append(f.lastURL, req.URL.String())
	resp, ok := f.routes[req.URL.String()]
	if !ok {
		resp, ok = f.routes[req.URL.Scheme+"://"+req.URL.Host+req.URL.Path]
	}
	if !ok {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(bytes.NewReader(nil)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	}
	if resp.err != nil {
		return nil, resp.err
	}
	status := resp.status
	if status == 0 {
		status = http.StatusOK
	}
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(bytes.NewReader(resp.body)),
		Header:     make(http.Header),
		Request:    req,
	}, nil
}

func (f *fakeHTTP) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

type fakeLDAP struct {
	dialErr  error
	bindErr  error
	hasTLS   bool
	notAfter time.Time
}

type fakeLDAPConn struct {
	bindErr  error
	hasTLS   bool
	notAfter time.Time
}

func (f *fakeLDAP) Dial(_ context.Context, _ *DexProviderLDAPForCheck) (LDAPConn, error) {
	if f.dialErr != nil {
		return nil, f.dialErr
	}
	return &fakeLDAPConn{bindErr: f.bindErr, hasTLS: f.hasTLS, notAfter: f.notAfter}, nil
}

func (c *fakeLDAPConn) Close() error { return nil }

func (c *fakeLDAPConn) Bind(_, _ string) error { return c.bindErr }

func (c *fakeLDAPConn) TLSConnectionState() (tls.ConnectionState, bool) {
	if !c.hasTLS {
		return tls.ConnectionState{}, false
	}
	notAfter := c.notAfter
	if notAfter.IsZero() {
		notAfter = time.Now().Add(365 * 24 * time.Hour)
	}
	return tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{{NotAfter: notAfter}},
	}, true
}
