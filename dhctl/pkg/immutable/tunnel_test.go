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

package immutable

import (
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/lib-connection/pkg/settings"
	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"
)

// A wait rebuilds this channel on every attempt, and the provider prints three
// lines per open. Taking the logger from the context is what lets the caller
// quiet the plumbing behind its own progress line.
func TestTheTunnelNarratesWhereItsContextNarrates(t *testing.T) {
	src, err := os.ReadFile("tunnel.go")
	if err != nil {
		t.Fatalf("read tunnel.go: %v", err)
	}

	body := string(src)
	if !strings.Contains(body, "channelSettings{Settings: sett, logger: dhlog.FromContext(ctx)}") {
		t.Error("the SSH provider must take its logger from the context, or a wait cannot quiet the plumbing behind it")
	}
	if strings.Contains(body, "provider.NewDefaultSSHProvider(\n\t\tsett,") {
		t.Error("the bare settings narrate onto the screen whatever the caller asked for")
	}
}

// A dial that ends on a deadline must cost the one connection it was made for.
// gossh's own forward ends its accept loop on such a dial and leaves the local
// port bound with nobody accepting: a converge holds one forward for its whole
// run, and the master behind it stops answering while it is being replaced.
func TestTheForwardOutlivesADialThatTimedOut(t *testing.T) {
	target := startEchoServer(t)
	bastion := startFakeBastion(t, testHostKey(t), target)

	address, stop := openChannelThroughFakeBastion(t, bastion, target)
	defer stop()

	echoed, err := echoThrough(address, "ping")
	require.NoError(t, err)
	require.Equal(t, "ping", echoed, "the forward must carry traffic before anything breaks it")

	bastion.blackhole.Store(true)
	waitForTheDialToTimeOut(t, address)
	bastion.blackhole.Store(false)

	require.Eventually(t, func() bool {
		echoed, err := echoThrough(address, "pong")
		return err == nil && echoed == "pong"
	}, 10*time.Second, 200*time.Millisecond,
		"the forward is dead and nothing rebuilds it: the local port stays bound with nobody accepting")
}

// waitForTheDialToTimeOut drives one connection into the blackholed forward and
// waits until the dial made for it gives up and its local end is closed.
func waitForTheDialToTimeOut(t *testing.T, address string) {
	t.Helper()

	conn, err := net.DialTimeout("tcp", address, time.Second)
	require.NoError(t, err)
	defer conn.Close()

	require.NoError(t, conn.SetDeadline(time.Now().Add(time.Minute)))
	_, err = io.Copy(io.Discard, conn)
	require.NoError(t, err)
}

func openChannelThroughFakeBastion(t *testing.T, bastion *fakeBastion, target string) (string, func()) {
	t.Helper()

	bastionHost, bastionPort := splitHostPort(t, bastion.address)
	targetHost, targetPort := splitHostPort(t, target)

	config := &sshconfig.Config{
		BastionHost:     bastionHost,
		BastionPort:     &bastionPort,
		BastionUser:     "ubuntu",
		BastionPassword: "password",
	}
	address, stop, err := OpenBastionChannel(
		t.Context(),
		&sshconfig.ConnectionConfig{Config: config.FillDefaults()},
		settings.NewBaseProviders(settings.ProviderParams{}),
		targetHost,
		targetPort,
		"test",
	)
	require.NoError(t, err)

	return address, stop
}

func startEchoServer(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() })

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				io.Copy(conn, conn)
			}()
		}
	}()

	return listener.Addr().String()
}

func echoThrough(address, message string) (string, error) {
	conn, err := net.DialTimeout("tcp", address, time.Second)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		return "", err
	}
	if _, err := conn.Write([]byte(message)); err != nil {
		return "", err
	}

	echoed := make([]byte, len(message))
	if _, err := io.ReadFull(conn, echoed); err != nil {
		return "", err
	}
	return string(echoed), nil
}

func splitHostPort(t *testing.T, address string) (string, int) {
	t.Helper()

	host, port, err := net.SplitHostPort(address)
	require.NoError(t, err)
	number, err := strconv.Atoi(port)
	require.NoError(t, err)

	return host, number
}
