// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/suite"

	v1 "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/api/handlers/root/v1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/api/server"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/envconfig"
	"github.com/deckhouse/deckhouse/pkg/log"
)

type SocketOnlySuite struct {
	suite.Suite

	socketPath string
	port       int
	client     *http.Client
	server     *server.Server
}

func TestSocketOnlySuite(t *testing.T) {
	suite.Run(t, new(SocketOnlySuite))
}

func (s *SocketOnlySuite) SetupTest() {
	// A short directory: a Unix socket path is limited to ~104 bytes on macOS.
	dir, err := os.MkdirTemp("/tmp", "pkgapi")
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = os.RemoveAll(dir) })

	s.socketPath = filepath.Join(dir, "debug.socket")

	// The TCP address the V2 server would bind: free now, and must stay free.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	s.Require().NoError(err)
	s.port = l.Addr().(*net.TCPAddr).Port
	s.Require().NoError(l.Close())
	s.T().Setenv(envconfig.EnvListenAddress, "127.0.0.1")
	s.T().Setenv(envconfig.EnvListenPort, strconv.Itoa(s.port))

	s.client = &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", s.socketPath)
		},
	}}

	provider := fakeProvider{}
	s.server = server.NewSocketOnly(s.socketPath, provider, v1.Deps{
		Packages:     provider,
		Queues:       provider,
		Scheduler:    provider,
		Requirements: func() map[string]any { return nil },
	}, log.NewNop())
	s.Require().NoError(s.server.Start())
	s.T().Cleanup(func() { s.Require().NoError(s.server.Stop()) })
}

func (s *SocketOnlySuite) TestServesThePackagesDumpOnTheSocket() {
	body, status := s.get("/api/v1/packages/dump")
	s.Equal(http.StatusOK, status)
	s.JSONEq(`{"apps": {}, "modules": {"known": {"report": {}, "observed": {}, "info": null}}}`, body)
}

func (s *SocketOnlySuite) TestBindsNoTCPPort() {
	l, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(s.port)))
	s.Require().NoError(err, "the socket-only server must not bind the TCP listen address")
	s.Require().NoError(l.Close())
}

func (s *SocketOnlySuite) TestMetricsIsNotFoundWithoutStorage() {
	_, status := s.get("/metrics")
	s.Equal(http.StatusNotFound, status)
}

func (s *SocketOnlySuite) TestReadyzAnswersTheEngineReadiness() {
	_, status := s.get("/readyz")
	s.Equal(http.StatusOK, status)
}

func (s *SocketOnlySuite) get(path string) (string, int) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://socket"+path, nil)
	s.Require().NoError(err)

	resp, err := s.client.Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)

	return string(body), resp.StatusCode
}

// fakeProvider is a ready engine with one known module.
type fakeProvider struct{}

func (fakeProvider) Ready() bool { return true }

func (fakeProvider) Dump() any {
	return map[string]any{
		"apps":    map[string]any{},
		"modules": map[string]any{"known": map[string]any{"report": map[string]any{}, "observed": map[string]any{}, "info": nil}},
	}
}

func (fakeProvider) DumpByName(string) any                          { return nil }
func (fakeProvider) DumpGlobal() any                                { return nil }
func (fakeProvider) Render(context.Context, string) (string, error) { return "", nil }
func (fakeProvider) Snapshots(string) (any, bool)                   { return nil, false }
func (fakeProvider) DumpQueues(string) any                          { return nil }
func (fakeProvider) DumpScheduler() any                             { return nil }
func (fakeProvider) DumpSchedulerByName(string) any                 { return nil }
