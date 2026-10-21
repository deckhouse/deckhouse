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
	"bytes"
	"context"
	"io"
	"net"
	"strconv"
	"testing"
	"time"

	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/settings"
	sshconfig "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/ssh/config"
	dhlog "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-dhctl/pkg/logger"
	"github.com/stretchr/testify/require"
)

// Every step that reaches the master opens a channel of its own, a wait does it
// every attempt, and the provider frames each open as a "Get SSH client" block.
// The terminal must not show it, and the debug file must keep it.
func TestOpeningAChannelStaysOffTheTerminal(t *testing.T) {
	target := startEchoServer(t)
	bastion := startFakeBastion(t, testHostKey(t), target)

	var terminal bytes.Buffer
	_, stop := openChannelThroughFakeBastion(dhlog.ToContext(t.Context(), dhlog.NewStreamLogger(&terminal)), t, bastion, target)
	stop()
	require.NotContains(t, terminal.String(), "Get SSH client", "opening a channel reached the terminal")

	var debugFile bytes.Buffer
	_, stop = openChannelThroughFakeBastion(dhlog.ToContext(t.Context(), dhlog.NewBufferLogger(&debugFile)), t, bastion, target)
	stop()
	require.Contains(t, debugFile.String(), `level=DEBUG msg="Starting: Get SSH client"`,
		"opening a channel is how a bastion that refuses is diagnosed, the debug file must keep it")
}

// A dial that ends on a deadline must cost the one connection it was made for.
// gossh's own forward ends its accept loop on such a dial and leaves the local
// port bound with nobody accepting: a converge holds one forward for its whole
// run, and the master behind it stops answering while it is being replaced.
func TestTheForwardOutlivesADialThatTimedOut(t *testing.T) {
	target := startEchoServer(t)
	bastion := startFakeBastion(t, testHostKey(t), target)

	address, stop := openChannelThroughFakeBastion(t.Context(), t, bastion, target)
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

func openChannelThroughFakeBastion(ctx context.Context, t *testing.T, bastion *fakeBastion, target string) (string, func()) {
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
		ctx,
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
