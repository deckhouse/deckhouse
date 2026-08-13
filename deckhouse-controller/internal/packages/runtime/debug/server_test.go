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

package debug_test

import (
	"context"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/debug"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// stopTimeout bounds the graceful shutdown of the server under test.
const stopTimeout = 5 * time.Second

// ServerSuite exercises the split between the socket and HTTP routers.
type ServerSuite struct {
	suite.Suite

	server     *debug.Server
	socketPath string
	httpAddr   string
}

// TestServerSuite runs the suite.
func TestServerSuite(t *testing.T) {
	suite.Run(t, new(ServerSuite))
}

// SetupTest starts a server with one socket-only and one HTTP-exposed route.
func (s *ServerSuite) SetupTest() {
	// Not t.TempDir(): its per-test name blows past the ~104 byte sun_path limit.
	dir, err := os.MkdirTemp("", "d8dbg")
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = os.RemoveAll(dir) })

	s.socketPath = filepath.Join(dir, "s.sock")

	s.server = debug.NewServer(debug.Config{
		SocketPath: s.socketPath,
		Address:    "127.0.0.1",
		Port:       "0", // let the kernel pick a free port
	}, log.NewNop())

	s.Require().NoError(s.server.Register(http.MethodGet, "/secret", handlerFor("secret"), debug.SocketOnly))
	s.Require().NoError(s.server.Register(http.MethodGet, "/public", handlerFor("public"), debug.AddHTTP))
	s.Require().NoError(s.server.Start())

	addrs := s.server.Addrs()
	s.Require().Len(addrs, 2)
	s.httpAddr = addrs[1]
}

// TearDownTest shuts the server down.
func (s *ServerSuite) TearDownTest() {
	ctx, cancel := context.WithTimeout(context.Background(), stopTimeout)
	defer cancel()

	s.Require().NoError(s.server.Stop(ctx))
}

// TestSocketServesEveryRoute checks that the socket reaches both exposures.
func (s *ServerSuite) TestSocketServesEveryRoute() {
	client, err := debug.NewClient(s.socketPath)
	s.Require().NoError(err)

	defer client.Close()

	secret, err := client.Get(context.Background(), "secret")
	s.Require().NoError(err)
	s.Equal("secret", string(secret))

	public, err := client.Get(context.Background(), "public")
	s.Require().NoError(err)
	s.Equal("public", string(public))
}

// TestHTTPServesOnlyAddHTTPRoutes checks that a socket-only route 404s over TCP.
func (s *ServerSuite) TestHTTPServesOnlyAddHTTPRoutes() {
	status, body := s.httpGet("/public")
	s.Equal(http.StatusOK, status)
	s.Equal("public", body)

	status, _ = s.httpGet("/secret")
	s.Equal(http.StatusNotFound, status)
}

// TestHTTPDiscoveryHidesSocketOnlyRoutes checks that discovery does not leak paths.
func (s *ServerSuite) TestHTTPDiscoveryHidesSocketOnlyRoutes() {
	status, body := s.httpGet("/discovery")
	s.Equal(http.StatusOK, status)
	s.Contains(body, "GET /public")
	s.NotContains(body, "/secret")

	client, err := debug.NewClient(s.socketPath)
	s.Require().NoError(err)

	defer client.Close()

	listing, err := client.Get(context.Background(), "discovery")
	s.Require().NoError(err)
	s.Contains(string(listing), "GET /secret")
}

// TestRegisterAfterStartIsRejected checks that the routing tree is frozen once serving.
func (s *ServerSuite) TestRegisterAfterStartIsRejected() {
	err := s.server.Register(http.MethodGet, "/late", handlerFor("late"), debug.AddHTTP)
	s.Require().ErrorIs(err, debug.ErrAlreadyStarted)

	status, _ := s.httpGet("/late")
	s.Equal(http.StatusNotFound, status)
}

// TestSocketIsOwnerOnly checks the socket file permissions.
func (s *ServerSuite) TestSocketIsOwnerOnly() {
	info, err := os.Stat(s.socketPath)
	s.Require().NoError(err)
	s.Equal(fs.FileMode(0o600), info.Mode().Perm())
}

// httpGet performs a GET against the TCP listener and returns status and body.
func (s *ServerSuite) httpGet(path string) (int, string) {
	s.T().Helper()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+s.httpAddr+path, nil)
	s.Require().NoError(err)

	resp, err := http.DefaultClient.Do(req)
	s.Require().NoError(err)

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)

	return resp.StatusCode, string(body)
}

// handlerFor builds a handler that writes the given body.
func handlerFor(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}
}
