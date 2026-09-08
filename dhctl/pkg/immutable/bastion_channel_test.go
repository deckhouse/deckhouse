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
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"k8s.io/client-go/tools/clientcmd"
)

// fakeBastion accepts one SSH connection and serves direct-tcpip channels by
// dialing the address it is asked for, counting every channel it opened.
type fakeBastion struct {
	address  string
	channels atomic.Int64

	// blackhole leaves a channel request unanswered, which is what a dial to a
	// destroyed master looks like: it ends on a deadline rather than a refusal.
	blackhole atomic.Bool
}

func startFakeBastion(t *testing.T, hostKey ssh.Signer, target string) *fakeBastion {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() })

	bastion := &fakeBastion{address: listener.Addr().String()}
	serverConfig := &ssh.ServerConfig{NoClientAuth: true}
	serverConfig.AddHostKey(hostKey)

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go bastion.serve(conn, serverConfig, target)
		}
	}()

	return bastion
}

func (b *fakeBastion) serve(conn net.Conn, serverConfig *ssh.ServerConfig, target string) {
	sshConn, newChannels, requests, err := ssh.NewServerConn(conn, serverConfig)
	if err != nil {
		return
	}
	defer sshConn.Close()
	go ssh.DiscardRequests(requests)

	for newChannel := range newChannels {
		// Anything but a forward is the SSH client keeping itself alive, and a
		// refusal there makes it drop the connection and reconnect.
		if newChannel.ChannelType() != "direct-tcpip" {
			channel, channelRequests, err := newChannel.Accept()
			if err != nil {
				continue
			}
			go ssh.DiscardRequests(channelRequests)
			go io.Copy(io.Discard, channel)
			continue
		}
		if b.blackhole.Load() {
			continue
		}
		channel, channelRequests, err := newChannel.Accept()
		if err != nil {
			continue
		}
		b.channels.Add(1)
		go ssh.DiscardRequests(channelRequests)
		go b.pipe(channel, target)
	}
}

func (b *fakeBastion) pipe(channel ssh.Channel, target string) {
	defer channel.Close()
	upstream, err := net.Dial("tcp", target)
	if err != nil {
		return
	}
	defer upstream.Close()
	go io.Copy(upstream, channel)
	io.Copy(channel, upstream)
}

// apiServerFor serves TLS under one name only: the name the kubeconfig gives.
func apiServerFor(t *testing.T, serverName string) (*httptest.Server, *x509.CertPool) {
	t.Helper()

	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: serverName},
		DNSNames:              []string{serverName},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	require.NoError(t, err)

	keyDER, err := x509.MarshalPKCS8PrivateKey(private)
	require.NoError(t, err)
	pair, err := tls.X509KeyPair(
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}),
	)
	require.NoError(t, err)

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok"))
	}))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{pair}}
	server.StartTLS()
	t.Cleanup(server.Close)

	pool := x509.NewCertPool()
	certificate, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	pool.AddCert(certificate)

	return server, pool
}

func testHostKey(t *testing.T) ssh.Signer {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(private)
	require.NoError(t, err)
	return signer
}

// dialThroughBastion returns an http.Client whose every connection is a
// direct-tcpip channel on the fake bastion, verified against serverName.
func dialThroughBastion(t *testing.T, bastion *fakeBastion, target, serverName string, pool *x509.CertPool) *http.Client {
	t.Helper()

	client, err := ssh.Dial("tcp", bastion.address, &ssh.ClientConfig{
		User:            "ubuntu",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	})
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })

	return &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return client.Dial(network, target)
		},
		TLSClientConfig: &tls.Config{RootCAs: pool, ServerName: serverName},
	}}
}

// The kubeconfig names an address only the cluster network resolves. Retargeted,
// its server points at the local end of the forward while the name TLS is
// verified against stays the one the certificate was issued for — anything else
// and the operator's kubeconfig cannot be used through a bastion at all.
func TestARetargetedKubeconfigReachesTheAPIThroughTheBastion(t *testing.T) {
	const serverName = "master-0.example"

	apiServer, pool := apiServerFor(t, serverName)
	_, port, err := net.SplitHostPort(apiServer.Listener.Addr().String())
	require.NoError(t, err)
	bastion := startFakeBastion(t, testHostKey(t), apiServer.Listener.Addr().String())

	original := kubeconfigFor("https://" + serverName + ":" + port)
	host, remotePort, err := kubeconfigServer(original, "")
	require.NoError(t, err)
	require.Equal(t, serverName, host, "the forward must reach the address the kubeconfig names")

	// What OpenKubeconfigChannel builds its configuration from once the tunnel is up.
	retargeted, err := RetargetKubeconfig(t.Context(), original, "https://127.0.0.1:1", host)
	require.NoError(t, err)
	parsed, err := clientcmd.Load(retargeted)
	require.NoError(t, err)
	cluster := parsed.Clusters["cluster"]
	require.Equal(t, serverName, cluster.TLSServerName,
		"the name TLS is verified against is the one the kubeconfig named, not a node name")

	target := net.JoinHostPort(host, strconv.Itoa(remotePort))
	response, err := dialThroughBastion(t, bastion, target, cluster.TLSServerName, pool).Get("https://127.0.0.1/")
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Equal(t, int64(1), bastion.channels.Load(), "the request must have travelled through the bastion")
}

// The bootstrap retargets to the node's name because it collected the kubeconfig
// from that node. Converge has no node name, and using one here would fail every
// handshake: this is the guard on that difference.
func TestANodeNameWouldFailTheHandshake(t *testing.T) {
	const serverName = "master-0.example"

	apiServer, pool := apiServerFor(t, serverName)
	bastion := startFakeBastion(t, testHostKey(t), apiServer.Listener.Addr().String())

	client := dialThroughBastion(t, bastion, apiServer.Listener.Addr().String(), "immd-h-master-0", pool)
	_, err := client.Get("https://127.0.0.1/")
	require.ErrorContains(t, err, "certificate is valid for "+serverName)
}
