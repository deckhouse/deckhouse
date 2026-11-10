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

package checks

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/settings"
	sshconfig "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/ssh/config"
	sshutils "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/ssh/utils"
	dhlog "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-dhctl/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

// TestBastionAvailabilityWithNoBastionIsNotApplicable: the check used to describe its own
// absence ("no bastion configured, skipping…") and pass, so the reader saw a ✓ over a sentence
// saying nothing had been checked — and the runner cached it. The absence is now an outcome of
// its own, which is neither a pass nor remembered.
func TestBastionAvailabilityWithNoBastionIsNotApplicable(t *testing.T) {
	const desc = "ssh connection to the bastion host is possible"

	tests := []struct {
		name          string
		cfg           *sshconfig.ConnectionConfig
		notApplicable bool
	}{
		{"bastion set", &sshconfig.ConnectionConfig{Config: &sshconfig.Config{BastionHost: "10.0.0.1"}}, false},
		{"no bastion", &sshconfig.ConnectionConfig{Config: &sshconfig.Config{}}, true},
		{"nil inner config", &sshconfig.ConnectionConfig{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			init := providerinitializer.NewSSHProviderInitializer(nil, tt.cfg)
			check := BastionAvailabilityCheck{SSHProviderInitializer: init}

			if got := check.Description(); got != desc {
				t.Errorf("Description() = %q, want the assertion that holds on a pass, %q", got, desc)
			}

			if !tt.notApplicable {
				return
			}
			_, err := check.Run(t.Context())
			if !errors.Is(err, preflight.ErrNotApplicable) {
				t.Errorf("Run() = %v, want a not-applicable outcome", err)
			}
			if !strings.Contains(err.Error(), "--ssh-bastion-host") {
				t.Errorf("the reason must name the flag that would configure one, got %q", err)
			}
		})
	}
}

func TestBastionConfigured(t *testing.T) {
	tests := []struct {
		name string
		cfg  *sshconfig.Config
		want bool
	}{
		{"nil config", nil, false},
		{"empty bastion host", &sshconfig.Config{}, false},
		{"bastion host set", &sshconfig.Config{BastionHost: "10.0.0.1"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := bastionConfigured(tt.cfg); got != tt.want {
				t.Errorf("bastionConfigured() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBastionPort(t *testing.T) {
	port := 2222
	tests := []struct {
		name string
		cfg  *sshconfig.Config
		want string
	}{
		{"nil config", nil, "22"},
		{"nil port defaults to 22", &sshconfig.Config{BastionHost: "h"}, "22"},
		{"port set", &sshconfig.Config{BastionHost: "h", BastionPort: &port}, "2222"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := bastionPort(tt.cfg); got != tt.want {
				t.Errorf("bastionPort() = %v, want %v", got, tt.want)
			}
		})
	}
}

// testPrivateKey returns a freshly generated OpenSSH private key, optionally encrypted.
func testPrivateKey(t *testing.T, passphrase string) string {
	t.Helper()

	_, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	var block *pem.Block
	if passphrase == "" {
		block, err = ssh.MarshalPrivateKey(key, "")
	} else {
		block, err = ssh.MarshalPrivateKeyWithPassphrase(key, "", []byte(passphrase))
	}
	require.NoError(t, err)

	return string(pem.EncodeToMemory(block))
}

// writeKeyFile writes a key file, as --ssh-agent-private-keys gives it. A connection config gives
// its keys inline.
func writeKeyFile(t *testing.T, pemKey string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "id_ed25519")
	require.NoError(t, os.WriteFile(path, []byte(pemKey), 0o600))
	return path
}

// TestBastionAuthMethods covers how the bastion check decides what to authenticate with. It never
// had a test, and the three sources it draws on — configured keys, a running ssh-agent and the
// bastion password — are exactly what an operator gets wrong when the handshake is refused.
func TestBastionAuthMethods(t *testing.T) {
	t.Run("a key given as a path", func(t *testing.T) {
		cfg := &sshconfig.Config{PrivateKeys: []sshconfig.AgentPrivateKey{
			{Key: writeKeyFile(t, testPrivateKey(t, "")), IsPath: true},
		}}

		auth, err := (BastionAvailabilityCheck{}).bastionAuthMethods(t.Context(), cfg, "")
		require.NoError(t, err)
		defer auth.cleanup()
		assert.Len(t, auth.methods, 1)
	})

	t.Run("a key given inline", func(t *testing.T) {
		cfg := &sshconfig.Config{PrivateKeys: []sshconfig.AgentPrivateKey{
			{Key: testPrivateKey(t, "")},
		}}
		var logged bytes.Buffer
		ctx := dhlog.ToContext(t.Context(), slog.New(slog.NewTextHandler(&logged, nil)))

		auth, err := (BastionAvailabilityCheck{}).bastionAuthMethods(ctx, cfg, "")
		require.NoError(t, err)
		defer auth.cleanup()
		assert.Len(t, auth.methods, 1)
		// Text is not a path: no <key>-cert.pub is looked up, so the key never reaches a warning.
		assert.Empty(t, logged.String())
	})

	t.Run("an encrypted key with its passphrase", func(t *testing.T) {
		cfg := &sshconfig.Config{PrivateKeys: []sshconfig.AgentPrivateKey{
			{Key: writeKeyFile(t, testPrivateKey(t, "secret")), IsPath: true, Passphrase: "secret"},
		}}

		auth, err := (BastionAvailabilityCheck{}).bastionAuthMethods(t.Context(), cfg, "")
		require.NoError(t, err)
		defer auth.cleanup()
		assert.Len(t, auth.methods, 1)
	})

	t.Run("an encrypted key with the wrong passphrase", func(t *testing.T) {
		cfg := &sshconfig.Config{PrivateKeys: []sshconfig.AgentPrivateKey{
			{Key: writeKeyFile(t, testPrivateKey(t, "secret")), IsPath: true, Passphrase: "not-it"},
		}}

		_, err := (BastionAvailabilityCheck{}).bastionAuthMethods(t.Context(), cfg, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "could not parse the private key")
	})

	t.Run("a key file that is not there", func(t *testing.T) {
		cfg := &sshconfig.Config{PrivateKeys: []sshconfig.AgentPrivateKey{
			{Key: filepath.Join(t.TempDir(), "absent"), IsPath: true},
		}}

		_, err := (BastionAvailabilityCheck{}).bastionAuthMethods(t.Context(), cfg, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "could not read the private key")
	})

	t.Run("the bastion password on its own", func(t *testing.T) {
		// --ssh-bastion-pass with no key at all is a configuration that works, so it must not
		// be reported as having nothing to authenticate with.
		cfg := &sshconfig.Config{BastionPassword: "hunter2"}

		auth, err := (BastionAvailabilityCheck{}).bastionAuthMethods(t.Context(), cfg, "")
		require.NoError(t, err)
		defer auth.cleanup()
		assert.Len(t, auth.methods, 1)
	})

	t.Run("a key and a password are both offered", func(t *testing.T) {
		cfg := &sshconfig.Config{
			PrivateKeys:     []sshconfig.AgentPrivateKey{{Key: testPrivateKey(t, "")}},
			BastionPassword: "hunter2",
		}

		auth, err := (BastionAvailabilityCheck{}).bastionAuthMethods(t.Context(), cfg, "")
		require.NoError(t, err)
		defer auth.cleanup()
		assert.Len(t, auth.methods, 2)
	})

	t.Run("nothing to authenticate with", func(t *testing.T) {
		// The answer the check has to give before it dials: refusing here names the cause,
		// while dialing gets "attempted methods [none]" from the far end.

		_, err := (BastionAvailabilityCheck{}).bastionAuthMethods(t.Context(), &sshconfig.Config{}, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no private key, no ssh-agent identity and no password")
	})

	t.Run("a running ssh-agent counts as a method", func(t *testing.T) {
		// The agent is the usual way a key reaches the bastion.
		socket, _ := testAgentSocket(t)

		auth, err := (BastionAvailabilityCheck{}).bastionAuthMethods(t.Context(), &sshconfig.Config{}, socket)
		require.NoError(t, err)
		defer auth.cleanup()
		assert.Len(t, auth.methods, 1)
	})

	t.Run("an agent socket pointing at nothing", func(t *testing.T) {
		_, err := (BastionAvailabilityCheck{}).bastionAuthMethods(t.Context(), &sshconfig.Config{}, filepath.Join(t.TempDir(), "not-a-socket"))
		require.ErrorIs(t, err, sshutils.ErrSSHCredentials)
		assert.ErrorContains(t, err, "open ssh-agent socket")
	})

	t.Run("a cancelled attempt is not a broken agent", func(t *testing.T) {
		ctx, cancel := context.WithCancelCause(t.Context())
		cause := errors.New("preflight cancelled before connecting to the agent")
		cancel(cause)

		_, err := (BastionAvailabilityCheck{}).bastionAuthMethods(ctx, &sshconfig.Config{}, filepath.Join(t.TempDir(), "not-a-socket"))

		require.ErrorIs(t, err, cause)
		assert.NotErrorIs(t, err, sshutils.ErrSSHCredentials)
	})
}

// testCA is a fresh SSH certificate authority.
func testCA(t *testing.T) ssh.Signer {
	t.Helper()

	_, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	ca, err := ssh.NewSignerFromKey(key)
	require.NoError(t, err)
	return ca
}

// signedCertificate is a user certificate for key, signed by ca and valid from after to before.
func signedCertificate(t *testing.T, ca ssh.Signer, key ssh.PublicKey, after, before uint64) *ssh.Certificate {
	t.Helper()

	cert := &ssh.Certificate{Key: key, CertType: ssh.UserCert, ValidAfter: after, ValidBefore: before}
	require.NoError(t, cert.SignCert(rand.Reader, ca))
	return cert
}

// testUserCertificate signs a user certificate for key with a fresh CA.
func testUserCertificate(t *testing.T, key ssh.PublicKey) *ssh.Certificate {
	t.Helper()
	return signedCertificate(t, testCA(t), key, 0, ssh.CertTimeInfinity)
}

// testCertificateValidFor signs a user certificate for key valid from after to before.
func testCertificateValidFor(t *testing.T, key ssh.PublicKey, after, before time.Time) *ssh.Certificate {
	t.Helper()
	return signedCertificate(t, testCA(t), key, uint64(after.Unix()), uint64(before.Unix()))
}

// TestBastionWithCertificates: a bastion that trusts a CA accepts only the certificate in
// <key>-cert.pub next to the key.
func TestBastionWithCertificates(t *testing.T) {
	pemKey := testPrivateKey(t, "")
	signer, err := ssh.ParsePrivateKey([]byte(pemKey))
	require.NoError(t, err)
	other, err := ssh.ParsePrivateKey([]byte(testPrivateKey(t, "")))
	require.NoError(t, err)
	hostCert := testUserCertificate(t, signer.PublicKey())
	hostCert.CertType = ssh.HostCert
	now := time.Now()

	for name, tc := range map[string]struct {
		content   []byte
		offered   bool
		warning   string
		notWarned string
	}{
		"valid":         {content: ssh.MarshalAuthorizedKey(testUserCertificate(t, signer.PublicKey())), offered: true},
		"expired":       {content: ssh.MarshalAuthorizedKey(testCertificateValidFor(t, signer.PublicKey(), now.Add(-2*time.Hour), now.Add(-time.Hour))), offered: true, warning: "expired at"},
		"not yet valid": {content: ssh.MarshalAuthorizedKey(testCertificateValidFor(t, signer.PublicKey(), now.Add(time.Hour), now.Add(2*time.Hour))), offered: true, warning: "is not valid until"},
		"malformed":     {content: []byte("not an SSH certificate"), warning: "parse SSH certificate"},
		"plain key":     {content: ssh.MarshalAuthorizedKey(signer.PublicKey()), warning: "must contain an SSH user certificate"},
		"different key": {content: ssh.MarshalAuthorizedKey(testUserCertificate(t, other.PublicKey())), warning: "does not match its private key"},
		"host cert":     {content: ssh.MarshalAuthorizedKey(hostCert), warning: "must contain an SSH user certificate"},
		// A certificate that is not offered gets no validity warning.
		"expired, different key": {content: ssh.MarshalAuthorizedKey(testCertificateValidFor(t, other.PublicKey(), now.Add(-2*time.Hour), now.Add(-time.Hour))), warning: "does not match its private key", notWarned: "expired at"},
	} {
		t.Run(name, func(t *testing.T) {
			path := writeKeyFile(t, pemKey)
			require.NoError(t, os.WriteFile(path+"-cert.pub", tc.content, 0o600))
			var logged strings.Builder
			ctx := dhlog.ToContext(t.Context(), slog.New(slog.NewTextHandler(&logged, nil)))

			keys := []sshconfig.AgentPrivateKey{{Key: path, IsPath: true}}
			signers, err := bastionSigners(keys)
			require.NoError(t, err)
			signers = (BastionAvailabilityCheck{}).withCertificates(ctx, keys, signers)

			if tc.warning == "" {
				assert.Empty(t, logged.String())
			} else {
				assert.Contains(t, logged.String(), tc.warning)
				assert.Contains(t, logged.String(), path+"-cert.pub")
			}
			if tc.notWarned != "" {
				assert.NotContains(t, logged.String(), tc.notWarned)
			}
			// A broken certificate is only a warning: the plain key may still be authorized. An
			// expired one is still offered: the local clock may be off.
			if !tc.offered {
				require.Len(t, signers, 1)
				return
			}
			require.Len(t, signers, 2)
			assert.Equal(t, signer.PublicKey().Marshal(), signers[0].PublicKey().Marshal())
			assert.Equal(t, tc.content, ssh.MarshalAuthorizedKey(signers[1].PublicKey()))
		})
	}
}

// refusingSigner is an agent identity whose agent refuses to sign.
type refusingSigner struct{ pub ssh.PublicKey }

func (s refusingSigner) PublicKey() ssh.PublicKey { return s.pub }

func (refusingSigner) Sign(io.Reader, []byte) (*ssh.Signature, error) {
	return nil, errors.New("agent refused to sign")
}

func TestOrderBastionSigners(t *testing.T) {
	newSigner := func() ssh.Signer {
		s, err := ssh.ParsePrivateKey([]byte(testPrivateKey(t, "")))
		require.NoError(t, err)
		return s
	}
	keyA, keyB, keyC, keyD := newSigner(), newSigner(), newSigner(), newSigner()
	certB, err := ssh.NewCertSigner(testUserCertificate(t, keyB.PublicKey()), keyB)
	require.NoError(t, err)
	agentCertA := refusingSigner{testUserCertificate(t, keyA.PublicKey())}
	agentCertB := refusingSigner{testUserCertificate(t, keyB.PublicKey())}
	agentCertD := refusingSigner{testUserCertificate(t, keyD.PublicKey())}

	got := orderBastionSigners(
		[]ssh.Signer{keyA, keyB, certB},
		[]ssh.Signer{keyC, agentCertB, agentCertD, keyA, agentCertA},
	)

	// Each passed key before its certificates, the <key>-cert.pub first, then the rest of the agent
	// in agent order.
	want := []ssh.PublicKey{keyA.PublicKey(), agentCertA.pub, keyB.PublicKey(), certB.PublicKey(), agentCertB.pub, keyC.PublicKey(), agentCertD.pub}
	require.Len(t, got, len(want))
	for i := range want {
		assert.Equal(t, want[i].Marshal(), got[i].PublicKey().Marshal(), "position %d", i)
	}

	// The agent certificate for a passed key is signed with that key, not by the agent.
	_, err = got[1].Sign(rand.Reader, []byte("data"))
	require.NoError(t, err)
}

// startTestBastion serves SSH on loopback and accepts only user certificates signed by ca. It
// rejects every forward with ConnectionFailed, which the check accepts: only "administratively
// prohibited" fails it.
func startTestBastion(t *testing.T, ca ssh.PublicKey) int {
	t.Helper()
	return startLimitedTestBastion(t, ca, 0)
}

// startLimitedTestBastion is startTestBastion that disconnects after maxAuthTries rejected keys.
func startLimitedTestBastion(t *testing.T, ca ssh.PublicKey, maxAuthTries int) int {
	t.Helper()

	checker := &ssh.CertChecker{IsUserAuthority: func(auth ssh.PublicKey) bool {
		return bytes.Equal(auth.Marshal(), ca.Marshal())
	}}
	return serveTestBastion(t, &ssh.ServerConfig{PublicKeyCallback: checker.Authenticate, MaxAuthTries: maxAuthTries})
}

// serveTestBastion serves cfg on loopback with a fresh host key and rejects every forward with
// ConnectionFailed.
func serveTestBastion(t *testing.T, cfg *ssh.ServerConfig) int {
	t.Helper()

	_, hostKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	hostSigner, err := ssh.NewSignerFromKey(hostKey)
	require.NoError(t, err)
	cfg.AddHostKey(hostSigner)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_, chans, reqs, err := ssh.NewServerConn(conn, cfg)
				if err != nil {
					return
				}
				go ssh.DiscardRequests(reqs)
				for ch := range chans {
					_ = ch.Reject(ssh.ConnectionFailed, "the test bastion forwards nowhere")
				}
			}()
		}
	}()

	return listener.Addr().(*net.TCPAddr).Port
}

// testAgentSocket listens on a fresh unix socket for a test ssh-agent.
func testAgentSocket(t *testing.T) (string, net.Listener) {
	t.Helper()

	// Not t.TempDir(): a unix socket path is capped near 100 bytes.
	dir, err := os.MkdirTemp("/tmp", "d8ag")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	socket := filepath.Join(dir, "a.sock")
	listener, err := net.Listen("unix", socket)
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	return socket, listener
}

// serveTestAgent serves keyring on a unix socket. A nil keyring accepts and drops every
// connection, an agent that cannot list its identities.
func serveTestAgent(t *testing.T, keyring agent.Agent) string {
	t.Helper()

	socket, listener := testAgentSocket(t)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			if keyring == nil {
				_ = conn.Close()
				continue
			}
			go func() {
				defer conn.Close()
				_ = agent.ServeAgent(keyring, conn)
			}()
		}
	}()

	return socket
}

// blockingSignAgent lists its keys and does not answer a sign request until done is closed. With
// done already closed it refuses every sign at once.
type blockingSignAgent struct {
	agent.Agent
	done    <-chan struct{}
	started chan<- struct{}
}

func (a blockingSignAgent) Sign(ssh.PublicKey, []byte) (*ssh.Signature, error) {
	select {
	case a.started <- struct{}{}:
	default:
	}
	<-a.done
	return nil, errors.New("agent stopped")
}

// A hung agent ends the attempt with an error instead of holding the preflight forever.
func TestBastionRunWithHungAgent(t *testing.T) {
	ca := testCA(t)
	port := startTestBastion(t, ca.PublicKey())

	_, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(key)
	require.NoError(t, err)
	cert := signedCertificate(t, ca, signer.PublicKey(), 0, ssh.CertTimeInfinity)
	keyring := agent.NewKeyring()
	require.NoError(t, keyring.Add(agent.AddedKey{PrivateKey: key, Certificate: cert}))
	done := make(chan struct{})
	t.Cleanup(func() { close(done) })

	started := make(chan struct{}, 1)

	cfg := &sshconfig.ConnectionConfig{Config: &sshconfig.Config{
		BastionHost: "127.0.0.1",
		BastionPort: &port,
		BastionUser: "user",
	}}
	init := providerinitializer.NewSSHProviderInitializer(nil, cfg)
	init.Reinitialize(t.Context(), settings.NewBaseProviders(settings.ProviderParams{AuthSock: serveTestAgent(t, blockingSignAgent{Agent: keyring, done: done, started: started})}), cfg)

	ctx, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)
	finished := make(chan error, 1)
	go func() {
		_, err := BastionAvailabilityCheck{SSHProviderInitializer: init}.Run(ctx)
		finished <- err
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("the agent was never asked to sign")
	}

	cause := errors.New("preflight cancelled during signing")
	cancel(cause)
	select {
	case err := <-finished:
		require.ErrorIs(t, err, cause)
		var failure *preflight.Failure
		assert.NotErrorAs(t, err, &failure)
	case <-time.After(5 * time.Second):
		t.Fatal("the check is still waiting for the agent")
	}
}

// slowSignAgent lists its keys at once and answers a sign request after delay, as an agent
// waiting for a hardware key touch does.
type slowSignAgent struct {
	agent.Agent
	delay time.Duration
}

func (a slowSignAgent) Sign(key ssh.PublicKey, data []byte) (*ssh.Signature, error) {
	time.Sleep(a.delay)
	return a.Agent.Sign(key, data)
}

// The listing deadline does not carry over to signing: a signature slower than
// the listing budget still logs in.
func TestBastionRunWithSlowSigningAgent(t *testing.T) {
	ca := testCA(t)
	port := startTestBastion(t, ca.PublicKey())

	_, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(key)
	require.NoError(t, err)
	keyring := agent.NewKeyring()
	require.NoError(t, keyring.Add(agent.AddedKey{
		PrivateKey:  key,
		Certificate: signedCertificate(t, ca, signer.PublicKey(), 0, ssh.CertTimeInfinity),
	}))

	listTimeout := time.Second

	cfg := &sshconfig.ConnectionConfig{Config: &sshconfig.Config{
		BastionHost: "127.0.0.1",
		BastionPort: &port,
		BastionUser: "user",
	}}
	init := providerinitializer.NewSSHProviderInitializer(nil, cfg)
	agentSocket := serveTestAgent(t, slowSignAgent{Agent: keyring, delay: listTimeout + 100*time.Millisecond})
	init.Reinitialize(t.Context(), settings.NewBaseProviders(settings.ProviderParams{AuthSock: agentSocket}), cfg)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	_, err = BastionAvailabilityCheck{SSHProviderInitializer: init, agentListTimeout: listTimeout}.Run(ctx)

	require.NoError(t, err)
}

// serveHungTestAgent accepts connections and never answers, an agent waiting on something else.
func serveHungTestAgent(t *testing.T) string {
	t.Helper()

	socket, listener := testAgentSocket(t)
	var mu sync.Mutex
	var conns []net.Conn
	t.Cleanup(func() {
		mu.Lock()
		defer mu.Unlock()
		for _, conn := range conns {
			_ = conn.Close()
		}
	})

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			conns = append(conns, conn)
			mu.Unlock()
		}
	}()

	return socket
}

// TestBastionRunWithCertificates logs in to a bastion that trusts only a CA, the setup the
// certificate support exists for, and checks which identities the check offers.
func TestBastionRunWithCertificates(t *testing.T) {
	ca := testCA(t)
	port := startTestBastion(t, ca.PublicKey())

	signCert := func(key ssh.PublicKey) *ssh.Certificate {
		return signedCertificate(t, ca, key, 0, ssh.CertTimeInfinity)
	}

	// The key the bastion accepts through the user's agent only.
	_, agentKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	agentSigner, err := ssh.NewSignerFromKey(agentKey)
	require.NoError(t, err)
	keyring := agent.NewKeyring()
	require.NoError(t, keyring.Add(agent.AddedKey{PrivateKey: agentKey, Certificate: signCert(agentSigner.PublicKey())}))

	tests := []struct {
		name     string
		withCert bool
		noKey    bool
		agent    string
		legacy   bool
		ok       bool
		hung     bool
	}{
		{name: "a key with its certificate", withCert: true, ok: true},
		{name: "a key without a certificate", ok: false},
		{name: "a hung agent does not cost the key", withCert: true, agent: serveHungTestAgent(t), ok: true, hung: true},
		// The default backend offers the agent alongside the passed keys, and so does the check.
		{name: "the agent next to a key", agent: serveTestAgent(t, keyring), ok: true},
		// The legacy backend loads the passed keys into an ssh-agent of its own. A pass on the
		// user's agent would be a pass on a key the connection never offers.
		{name: "the agent next to a key in legacy mode", agent: serveTestAgent(t, keyring), legacy: true, ok: false},
		// Without passed keys the legacy backend uses the user's agent, and so does the check.
		{name: "the agent alone in legacy mode", noKey: true, agent: serveTestAgent(t, keyring), legacy: true, ok: true},
		{name: "legacy with file keys ignores a missing user agent", withCert: true, agent: filepath.Join(t.TempDir(), "gone.sock"), legacy: true, ok: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pemKey := testPrivateKey(t, "")
			path := writeKeyFile(t, pemKey)
			if tt.withCert {
				signer, err := ssh.ParsePrivateKey([]byte(pemKey))
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(path+"-cert.pub", ssh.MarshalAuthorizedKey(signCert(signer.PublicKey())), 0o600))
			}

			cfg := &sshconfig.ConnectionConfig{Config: &sshconfig.Config{
				BastionHost: "127.0.0.1",
				BastionPort: &port,
				BastionUser: "user",
				PrivateKeys: []sshconfig.AgentPrivateKey{{Key: path, IsPath: true}},
			}}
			if tt.noKey {
				cfg.Config.PrivateKeys = nil
			}
			cfg.Config.ForceLegacy = tt.legacy
			init := providerinitializer.NewSSHProviderInitializer(nil, cfg)
			init.Reinitialize(t.Context(), settings.NewBaseProviders(settings.ProviderParams{AuthSock: tt.agent}), cfg)

			// A hung agent must cost the listing deadline, not the whole attempt.
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			check := BastionAvailabilityCheck{SSHProviderInitializer: init}
			if tt.hung {
				check.agentListTimeout = 200 * time.Millisecond
			}
			_, err := check.Run(ctx)

			if tt.ok {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, "the bastion rejected every authentication method offered")
		})
	}
}

// With no key passed, the ssh-agent is the only source of keys: an agent that cannot list them is
// what went wrong, not the bastion.
func TestBastionRunWithFailingAgentOnly(t *testing.T) {
	ca := testCA(t)
	port := startTestBastion(t, ca.PublicKey())
	for name, tc := range map[string]struct {
		authSock  string
		permanent bool
	}{
		"an agent that closes the connection": {authSock: serveTestAgent(t, nil), permanent: true},
		// A slow agent may answer on the next attempt.
		"a hung agent": {authSock: serveHungTestAgent(t), permanent: false},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := &sshconfig.ConnectionConfig{Config: &sshconfig.Config{
				BastionHost: "127.0.0.1",
				BastionPort: &port,
				BastionUser: "user",
			}}
			init := providerinitializer.NewSSHProviderInitializer(nil, cfg)
			init.Reinitialize(t.Context(), settings.NewBaseProviders(settings.ProviderParams{AuthSock: tc.authSock}), cfg)

			var logged bytes.Buffer
			ctx := dhlog.ToContext(t.Context(), slog.New(slog.NewTextHandler(&logged, nil)))

			check := BastionAvailabilityCheck{SSHProviderInitializer: init}
			if !tc.permanent {
				check.agentListTimeout = 200 * time.Millisecond
			}
			_, err := check.Run(ctx)

			var failure *preflight.Failure
			require.ErrorAs(t, err, &failure)
			assert.Contains(t, failure.Observed, "could not use its SSH credentials")
			assert.Contains(t, failure.Fix, "ssh-add -l")
			assert.Equal(t, tc.permanent, isPermanent(err))
			assert.NotNil(t, failure.Err)
			assert.Empty(t, logged.String(), "the failure already explains the agent error")
		})
	}
}

func TestBastionRunWithUnavailableAgentSocket(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "d8ag")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	cfg := &sshconfig.ConnectionConfig{Config: &sshconfig.Config{
		BastionHost: "127.0.0.1",
		PrivateKeys: []sshconfig.AgentPrivateKey{{Key: testPrivateKey(t, "")}},
	}}
	init := providerinitializer.NewSSHProviderInitializer(nil, cfg)
	init.Reinitialize(t.Context(), settings.NewBaseProviders(settings.ProviderParams{AuthSock: filepath.Join(dir, "absent.sock")}), cfg)
	check := BastionAvailabilityCheck{SSHProviderInitializer: init}

	_, err = check.Run(t.Context())

	require.ErrorIs(t, err, sshutils.ErrSSHCredentials)
	var failure *preflight.Failure
	require.ErrorAs(t, err, &failure)
	assert.Contains(t, failure.Observed, "could not open the configured ssh-agent socket")
	assert.Contains(t, failure.Fix, "unset SSH_AUTH_SOCK")
	assert.True(t, isPermanent(err))
}

func TestBastionRunWithFailingAgentAndPassword(t *testing.T) {
	for _, tc := range []struct {
		name            string
		passwordWorks   bool
		listingTimesOut bool
	}{
		{name: "the password still works", passwordWorks: true},
		{name: "a rejected password does not hide a broken agent"},
		{name: "a rejected password does not make a slow agent permanent", listingTimesOut: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			port := serveTestBastion(t, &ssh.ServerConfig{
				PublicKeyCallback: func(ssh.ConnMetadata, ssh.PublicKey) (*ssh.Permissions, error) {
					return nil, errors.New("unexpected key")
				},
				PasswordCallback: func(ssh.ConnMetadata, []byte) (*ssh.Permissions, error) {
					if tc.passwordWorks {
						return nil, nil
					}
					return nil, errors.New("wrong password")
				},
			})
			cfg := &sshconfig.ConnectionConfig{Config: &sshconfig.Config{
				BastionHost:     "127.0.0.1",
				BastionPort:     &port,
				BastionUser:     "user",
				BastionPassword: "dh-test-secret-0123456789abcdef",
			}}

			check := BastionAvailabilityCheck{SSHProviderInitializer: providerinitializer.NewSSHProviderInitializer(nil, cfg)}
			var socket string
			if tc.listingTimesOut {
				socket = serveHungTestAgent(t)
				check.agentListTimeout = 200 * time.Millisecond
			} else {
				socket = serveTestAgent(t, nil)
			}
			check.SSHProviderInitializer.Reinitialize(t.Context(), settings.NewBaseProviders(settings.ProviderParams{AuthSock: socket}), cfg)

			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			var logged bytes.Buffer
			ctx = dhlog.ToContext(ctx, slog.New(slog.NewTextHandler(&logged, nil)))

			_, err := check.Run(ctx)

			if tc.passwordWorks {
				require.NoError(t, err)
				assert.Contains(t, logged.String(), "list ssh-agent identities")
				return
			}

			var failure *preflight.Failure
			require.ErrorAs(t, err, &failure)
			assert.Contains(t, failure.Observed, "could not list the ssh-agent identities")
			assert.Contains(t, failure.Observed, "bastion rejected the password")
			assert.Contains(t, failure.Err.Error(), "unable to authenticate")
			assert.Equal(t, !tc.listingTimesOut, isPermanent(err))
			if tc.listingTimesOut {
				assert.ErrorIs(t, err, errBastionAgentTimeout)
			} else {
				assert.ErrorIs(t, err, sshutils.ErrSSHCredentials)
			}
			assert.Empty(t, logged.String(), "the failure already explains the agent error")
		})
	}
}

// An agent that fails next to a passed key costs nothing, but the user is told about it.
func TestBastionRunWarnsAboutTheAgent(t *testing.T) {
	ca := testCA(t)
	port := startTestBastion(t, ca.PublicKey())

	for name, tc := range map[string]struct {
		authSock string
		warning  string
	}{
		"an agent that cannot list": {authSock: serveTestAgent(t, nil), warning: "cannot list ssh-agent identities"},
	} {
		t.Run(name, func(t *testing.T) {
			pemKey := testPrivateKey(t, "")
			signer, err := ssh.ParsePrivateKey([]byte(pemKey))
			require.NoError(t, err)
			path := writeKeyFile(t, pemKey)
			cert := signedCertificate(t, ca, signer.PublicKey(), 0, ssh.CertTimeInfinity)
			require.NoError(t, os.WriteFile(path+"-cert.pub", ssh.MarshalAuthorizedKey(cert), 0o600))

			cfg := &sshconfig.ConnectionConfig{Config: &sshconfig.Config{
				BastionHost: "127.0.0.1",
				BastionPort: &port,
				BastionUser: "user",
				PrivateKeys: []sshconfig.AgentPrivateKey{{Key: path, IsPath: true}},
			}}
			init := providerinitializer.NewSSHProviderInitializer(nil, cfg)
			init.Reinitialize(t.Context(), settings.NewBaseProviders(settings.ProviderParams{AuthSock: tc.authSock}), cfg)
			var logged bytes.Buffer
			ctx := dhlog.ToContext(t.Context(), slog.New(slog.NewTextHandler(&logged, nil)))

			_, err = BastionAvailabilityCheck{SSHProviderInitializer: init}.Run(ctx)

			require.NoError(t, err)
			assert.Contains(t, logged.String(), tc.warning)
		})
	}
}

// Run builds the credentials again on every retry; a warning about them is logged once per check.
func TestBastionRunWarnsOncePerCheck(t *testing.T) {
	ca := testCA(t)
	port := startTestBastion(t, ca.PublicKey())

	pemKey := testPrivateKey(t, "")
	signer, err := ssh.ParsePrivateKey([]byte(pemKey))
	require.NoError(t, err)
	path := writeKeyFile(t, pemKey)
	now := time.Now()
	expired := testCertificateValidFor(t, signer.PublicKey(), now.Add(-2*time.Hour), now.Add(-time.Hour))
	require.NoError(t, os.WriteFile(path+"-cert.pub", ssh.MarshalAuthorizedKey(expired), 0o600))

	cfg := &sshconfig.ConnectionConfig{Config: &sshconfig.Config{
		BastionHost: "127.0.0.1",
		BastionPort: &port,
		BastionUser: "user",
		PrivateKeys: []sshconfig.AgentPrivateKey{{Key: path, IsPath: true}},
	}}
	init := providerinitializer.NewSSHProviderInitializer(nil, cfg)
	init.Reinitialize(t.Context(), settings.NewBaseProviders(settings.ProviderParams{}), cfg)

	var logged bytes.Buffer
	ctx := dhlog.ToContext(t.Context(), slog.New(slog.NewTextHandler(&logged, nil)))
	check := BastionAvailability(init)
	for range 3 {
		_, err := check.Run(ctx)
		require.Error(t, err)
	}
	assert.Equal(t, 1, strings.Count(logged.String(), "expired at"), logged.String())
}

// A bastion whose MaxAuthTries runs out before an authorized identity is reached says so.
func TestBastionRunMaxAuthTries(t *testing.T) {
	ca := testCA(t)
	port := startLimitedTestBastion(t, ca.PublicKey(), 1)

	keys := make([]sshconfig.AgentPrivateKey, 0, 3)
	for range 3 {
		keys = append(keys, sshconfig.AgentPrivateKey{Key: writeKeyFile(t, testPrivateKey(t, "")), IsPath: true})
	}
	cfg := &sshconfig.ConnectionConfig{Config: &sshconfig.Config{
		BastionHost: "127.0.0.1",
		BastionPort: &port,
		BastionUser: "user",
		PrivateKeys: keys,
	}}
	init := providerinitializer.NewSSHProviderInitializer(nil, cfg)
	init.Reinitialize(t.Context(), settings.NewBaseProviders(settings.ProviderParams{}), cfg)

	_, err := BastionAvailabilityCheck{SSHProviderInitializer: init}.Run(t.Context())

	var failure *preflight.Failure
	require.ErrorAs(t, err, &failure)
	assert.Contains(t, failure.Observed, "MaxAuthTries")
	assert.Contains(t, failure.Fix, maxAuthTriesNote)
	assert.True(t, isPermanent(err))
}

// A timed-out listing drops the agent and closes the stream, so any late
// response cannot be mistaken for the response to a later request.
func TestBastionPublicKeysAfterATimedOutListing(t *testing.T) {
	client, peer := net.Pipe()
	t.Cleanup(func() { _ = client.Close() })
	t.Cleanup(func() { _ = peer.Close() })
	require.NoError(t, peer.SetDeadline(time.Now().Add(5*time.Second)))

	closed := make(chan error, 1)
	go func() {
		defer peer.Close()
		var request [5]byte // The agent list request is a four-byte length and a one-byte type.
		if _, err := io.ReadFull(peer, request[:]); err != nil {
			closed <- err
			return
		}
		// Withhold the reply and wait for the client to close the socket.
		_, err := peer.Read(request[:])
		closed <- err
	}()

	conn := client
	fileKey, err := ssh.ParsePrivateKey([]byte(testPrivateKey(t, "")))
	require.NoError(t, err)
	var logged bytes.Buffer
	ctx := dhlog.ToContext(t.Context(), slog.New(slog.NewTextHandler(&logged, nil)))
	check := BastionAvailabilityCheck{agentListTimeout: 200 * time.Millisecond}

	first, err := check.bastionPublicKeys(ctx, []ssh.Signer{fileKey}, &conn)
	require.NoError(t, err)
	require.Len(t, first, 1)
	require.Nil(t, conn)
	require.ErrorIs(t, <-closed, io.EOF)
	require.Equal(t, 1, strings.Count(logged.String(), "cannot list ssh-agent identities"))

	second, err := check.bastionPublicKeys(ctx, []ssh.Signer{fileKey}, &conn)
	require.NoError(t, err)
	require.Len(t, second, 1, "only the passed key: the agent was dropped")
	assert.Equal(t, 1, strings.Count(logged.String(), "cannot list ssh-agent identities"))
}

// A key given as text is not a path, so no <key>-cert.pub is looked up for it, even when a file of
// that name exists.
func TestWithCertificatesSkipsInlineKeys(t *testing.T) {
	pemKey := testPrivateKey(t, "")
	signer, err := ssh.ParsePrivateKey([]byte(pemKey))
	require.NoError(t, err)
	path := writeKeyFile(t, pemKey)
	require.NoError(t, os.WriteFile(path+"-cert.pub", ssh.MarshalAuthorizedKey(testUserCertificate(t, signer.PublicKey())), 0o600))

	got := (BastionAvailabilityCheck{}).withCertificates(t.Context(), []sshconfig.AgentPrivateKey{{Key: path}}, []ssh.Signer{signer})

	require.Len(t, got, 1)
}

// A certificate the agent holds for a passed key arrives as an agent identity, not as an
// *ssh.Certificate. It still goes right after the key and is signed with the key file.
func TestOrderBastionSignersWithARealAgent(t *testing.T) {
	_, privA, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	keyA, err := ssh.NewSignerFromKey(privA)
	require.NoError(t, err)
	_, privOther, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	other, err := ssh.NewSignerFromKey(privOther)
	require.NoError(t, err)
	certA := testUserCertificate(t, keyA.PublicKey())

	keyring := agent.NewKeyring()
	require.NoError(t, keyring.Add(agent.AddedKey{PrivateKey: privOther}))
	require.NoError(t, keyring.Add(agent.AddedKey{PrivateKey: privA, Certificate: certA}))
	refused := make(chan struct{})
	close(refused)
	conn, err := net.Dial("unix", serveTestAgent(t, blockingSignAgent{Agent: keyring, done: refused}))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	agentSigners, err := agent.NewClient(struct{ io.ReadWriter }{conn}).Signers()
	require.NoError(t, err)

	got := orderBastionSigners([]ssh.Signer{keyA}, agentSigners)

	require.GreaterOrEqual(t, len(got), 3)
	assert.Equal(t, keyA.PublicKey().Marshal(), got[0].PublicKey().Marshal())
	assert.Equal(t, certA.Marshal(), got[1].PublicKey().Marshal())
	_, err = got[1].Sign(rand.Reader, []byte("data"))
	require.NoError(t, err, "the certificate is signed with the key file, not by the agent")
	assert.Equal(t, other.PublicKey().Marshal(), got[len(got)-1].PublicKey().Marshal())
}

// Each key file gets its own certificate, matched by position.
func TestWithCertificatesSeveralKeys(t *testing.T) {
	pemA, pemB := testPrivateKey(t, ""), testPrivateKey(t, "")
	pathA, pathB := writeKeyFile(t, pemA), writeKeyFile(t, pemB)
	keys := []sshconfig.AgentPrivateKey{{Key: pathA, IsPath: true}, {Key: pathB, IsPath: true}}
	signers, err := bastionSigners(keys)
	require.NoError(t, err)
	certB := ssh.MarshalAuthorizedKey(testUserCertificate(t, signers[1].PublicKey()))
	require.NoError(t, os.WriteFile(pathB+"-cert.pub", certB, 0o600))
	var logged bytes.Buffer
	ctx := dhlog.ToContext(t.Context(), slog.New(slog.NewTextHandler(&logged, nil)))

	got := (BastionAvailabilityCheck{}).withCertificates(ctx, keys, signers)

	require.Len(t, got, 3)
	assert.Equal(t, signers[0].PublicKey().Marshal(), got[0].PublicKey().Marshal())
	assert.Equal(t, signers[1].PublicKey().Marshal(), got[1].PublicKey().Marshal())
	assert.Equal(t, certB, ssh.MarshalAuthorizedKey(got[2].PublicKey()))
	assert.Empty(t, logged.String())
}

// A bastion that wants two keys in a row asks the agent twice in one handshake.
func TestBastionRunWithPartialSuccess(t *testing.T) {
	_, privFirst, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	first, err := ssh.NewSignerFromKey(privFirst)
	require.NoError(t, err)
	_, privSecond, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	second, err := ssh.NewSignerFromKey(privSecond)
	require.NoError(t, err)
	accept := func(want ssh.PublicKey) func(ssh.ConnMetadata, ssh.PublicKey) (*ssh.Permissions, error) {
		return func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if !bytes.Equal(key.Marshal(), want.Marshal()) {
				return nil, errors.New("not this key")
			}
			return nil, nil
		}
	}
	cfg := &ssh.ServerConfig{PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
		if _, err := accept(first.PublicKey())(conn, key); err != nil {
			return nil, err
		}
		return nil, &ssh.PartialSuccessError{Next: ssh.ServerAuthCallbacks{PublicKeyCallback: accept(second.PublicKey())}}
	}}
	port := serveTestBastion(t, cfg)

	keyring := agent.NewKeyring()
	require.NoError(t, keyring.Add(agent.AddedKey{PrivateKey: privFirst}))
	require.NoError(t, keyring.Add(agent.AddedKey{PrivateKey: privSecond}))
	conf := &sshconfig.ConnectionConfig{Config: &sshconfig.Config{
		BastionHost: "127.0.0.1",
		BastionPort: &port,
		BastionUser: "user",
	}}
	init := providerinitializer.NewSSHProviderInitializer(nil, conf)
	init.Reinitialize(t.Context(), settings.NewBaseProviders(settings.ProviderParams{AuthSock: serveTestAgent(t, keyring)}), conf)
	var logged bytes.Buffer
	ctx := dhlog.ToContext(t.Context(), slog.New(slog.NewTextHandler(&logged, nil)))

	_, err = BastionAvailabilityCheck{SSHProviderInitializer: init}.Run(ctx)

	require.NoError(t, err)
	assert.NotContains(t, logged.String(), "cannot list ssh-agent identities")
}

// A cancelled attempt closes the agent socket; the check reports the cancellation, not a broken
// agent.
func TestBastionRunCancelledWhileListing(t *testing.T) {
	ca := testCA(t)
	port := startTestBastion(t, ca.PublicKey())

	cfg := &sshconfig.ConnectionConfig{Config: &sshconfig.Config{
		BastionHost: "127.0.0.1",
		BastionPort: &port,
		BastionUser: "user",
	}}
	init := providerinitializer.NewSSHProviderInitializer(nil, cfg)
	init.Reinitialize(t.Context(), settings.NewBaseProviders(settings.ProviderParams{AuthSock: serveHungTestAgent(t)}), cfg)
	var logged bytes.Buffer
	ctx, cancel := context.WithTimeout(dhlog.ToContext(t.Context(), slog.New(slog.NewTextHandler(&logged, nil))), 300*time.Millisecond)
	defer cancel()

	_, err := BastionAvailabilityCheck{SSHProviderInitializer: init}.Run(ctx)

	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.NotContains(t, logged.String(), "cannot list ssh-agent identities", "the agent is not to blame")
	var failure *preflight.Failure
	assert.False(t, errors.As(err, &failure), "not a verdict on the credentials: %v", err)
}
