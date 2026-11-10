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
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	sshconfig "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/ssh/config"
	sshutils "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/ssh/utils"
	dhlog "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-dhctl/pkg/logger"
	"github.com/cenkalti/backoff/v4"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

const BastionAvailabilityCheckName preflight.CheckName = "bastion-availability"

const (
	defaultBastionPort      = "22"
	bastionDialTimeout      = 10 * time.Second
	bastionAgentDialTimeout = 5 * time.Second
	bastionAgentListTimeout = 5 * time.Second
)

// errBastionAgentTimeout marks an ssh-agent that is the only source of keys and did not list them
// in time. It may be slow rather than broken, so the check is retried.
var errBastionAgentTimeout = errors.New("the ssh-agent did not list its identities in time")

type BastionAvailabilityCheck struct {
	SSHProviderInitializer *providerinitializer.SSHProviderInitializer
	warned                 *sync.Map
	agentListTimeout       time.Duration
}

// warnOnce logs each warning about the bastion credentials once per check: Run builds them again on
// every retry.
func (c BastionAvailabilityCheck) warnOnce(ctx context.Context, message string) {
	if c.warned != nil {
		if _, seen := c.warned.LoadOrStore(message, true); seen {
			return
		}
	}

	dhlog.FromContext(ctx).WarnContext(ctx, message)
}

// Description is the assertion that holds when the check passes. It used to describe its own
// absence ("no bastion configured, skipping…") when there was no bastion, which printed a ✓ over
// a sentence saying nothing had been checked — and cached it. The absence is now reported by the
// body as a not-applicable outcome, which is neither a pass nor remembered.
func (BastionAvailabilityCheck) Description() string {
	return "ssh connection to the bastion host is possible"
}

func (BastionAvailabilityCheck) Phase() preflight.Phase {
	return preflight.PhasePreInfra
}

func (BastionAvailabilityCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.RetryPolicy{
		Attempts: 15,
		Options: []backoff.ExponentialBackOffOpts{
			backoff.WithInitialInterval(time.Second),
			backoff.WithMultiplier(2),
			backoff.WithMaxInterval(5 * time.Second),
			backoff.WithMaxElapsedTime(0),
		},
	}
}

func bastionConfigured(cfg *sshconfig.Config) bool {
	return cfg != nil && cfg.BastionHost != ""
}

func bastionPort(cfg *sshconfig.Config) string {
	if cfg == nil || cfg.BastionPort == nil {
		return defaultBastionPort
	}
	return cfg.BastionPortString()
}

func (c BastionAvailabilityCheck) Run(ctx context.Context) (string, error) {
	connCfg := c.SSHProviderInitializer.GetConfig()
	if connCfg == nil || !bastionConfigured(connCfg.Config) {
		return "", preflight.NotApplicable("no --ssh-bastion-host was given")
	}

	sshCfg := connCfg.Config
	addr := net.JoinHostPort(sshCfg.BastionHost, bastionPort(sshCfg))
	user := sshCfg.BastionUser

	// The agent the connection will talk to. The legacy backend loads passed keys into an ssh-agent
	// of its own and never asks the user's one, so offering it here would pass on keys the
	// connection never offers.
	authSock := authSockOf(c.SSHProviderInitializer)
	if c.SSHProviderInitializer.IsLegacyMode() && len(sshCfg.PrivateKeys) > 0 {
		authSock = ""
	}

	auth, err := c.bastionAuthMethods(ctx, sshCfg, authSock)
	if err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("bastion %s: %w", addr, context.Cause(ctx))
		}

		if errors.Is(err, sshutils.ErrSSHCredentials) {
			return "", preflight.Permanent(&preflight.Failure{
				Checked:  fmt.Sprintf("the ssh-agent for bastion %s", addr),
				Observed: "dhctl could not open the configured ssh-agent socket",
				Expected: "an accessible ssh-agent at SSH_AUTH_SOCK",
				Fix:      "check SSH_AUTH_SOCK and run ssh-add -l, or unset SSH_AUTH_SOCK if the agent is not needed",
				Err:      err,
			})
		}

		// No key, no agent and no password: nothing about that changes on a second attempt.
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  fmt.Sprintf("the credentials for bastion %s", addr),
			Observed: err.Error(),
			Expected: "a private key, an ssh-agent identity or a password for the bastion",
			Fix:      "pass --ssh-agent-private-keys, run an ssh-agent, or pass --ask-bastion-pass",
			Err:      err,
		})
	}
	defer auth.cleanup()

	clientCfg := &ssh.ClientConfig{
		User:            user,
		Auth:            auth.methods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         bastionDialTimeout,
	}

	// 1. TCP reachability check, honouring context cancellation.
	conn, err := (&net.Dialer{Timeout: bastionDialTimeout}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return "", &preflight.Failure{
			Checked:  fmt.Sprintf("tcp connection to bastion %s", addr),
			Observed: classifyNetworkError(err),
			Expected: "an answer from the bastion",
			Fix:      "check --ssh-bastion-host and --ssh-bastion-port, and that the port is open from this host",
			Err:      err,
		}
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(bastionDialTimeout))

	// 2. SSH transport + user authentication ONLY.
	//
	// We deliberately do NOT build a full lib-connection SSH client here: its
	// Start() spawns a keepalive goroutine that opens an SSH *session* on the
	// target. dhctl uses the bastion only as a ProxyJump (direct-tcpip
	// forwarding) and never runs commands on it, so a hardened bastion may
	// permit forwarding while denying shell/exec (ForceCommand, no-pty,
	// command="..." in authorized_keys, a restricted/kill shell). Such a
	// bastion is still valid for our use, but the keepalive session would be
	// rejected and trigger an endless reconnect loop. A bare transport+auth
	// handshake validates exactly what we rely on, nothing more.
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, clientCfg)
	// Cancellation can close the agent socket, and x/crypto may try a password
	// after that. Report the cancellation regardless of the handshake result.
	if ctx.Err() != nil {
		return "", fmt.Errorf("bastion %s: %w", addr, context.Cause(ctx))
	}

	if err != nil {
		if errors.Is(err, errBastionAgentTimeout) {
			return "", localCredentialProblem(fmt.Sprintf("bastion %s", addr), err)
		}

		if errors.Is(err, sshutils.ErrSSHCredentials) {
			return "", localCredentialFailure(fmt.Sprintf("bastion %s", addr), err)
		}

		failure := &preflight.Failure{
			Checked:  fmt.Sprintf("ssh login to bastion %s as %q", addr, user),
			Observed: "the bastion rejected every authentication method offered",
			Expected: "an authorized bastion user",
			Fix: "check --ssh-bastion-user. Authorize one of --ssh-agent-private-keys for that user " +
				"on the bastion, or pass --ask-bastion-pass. If the bastion trusts a CA, put the signed " +
				"certificate next to the key as <key>-cert.pub",
			Err: err,
		}
		if isSSHAuthError(err) {
			if auth.agentListErr != nil && auth.passwordTried {
				failure.Observed = "dhctl could not list the ssh-agent identities, and the bastion rejected the password"
				failure.Fix = "check SSH_AUTH_SOCK and ssh-add -l, and verify the password given with --ask-bastion-pass"
				failure.Err = errors.Join(err, auth.agentListErr)
				if isMaxAuthTries(err) {
					failure.Observed += " (MaxAuthTries)"
				}
				if errors.Is(auth.agentListErr, errBastionAgentTimeout) {
					return "", failure
				}
				return "", preflight.Permanent(failure)
			}

			if isMaxAuthTries(err) {
				failure.Observed = maxAuthTriesObserved
				failure.Fix += ". " + maxAuthTriesNote
			}

			return "", preflight.Permanent(failure)
		}
		failure.Observed = classifyNetworkError(err)
		return "", failure
	}

	if auth.agentListErr != nil {
		c.warnOnce(ctx, auth.agentListErr.Error())
	}

	client := ssh.NewClient(sshConn, chans, reqs)
	defer client.Close()

	// 3. The forward itself, which is the only thing dhctl actually asks of a bastion. A bastion
	// with AllowTcpForwarding no authenticates perfectly and is still useless, and that used to
	// surface minutes later as a failure to reach the master.
	if forwarded, err := client.Dial("tcp", net.JoinHostPort("127.0.0.1", "22")); err != nil {
		if strings.Contains(err.Error(), "administratively prohibited") {
			return "", preflight.Permanent(&preflight.Failure{
				Checked:  fmt.Sprintf("a direct-tcpip forward through bastion %s", addr),
				Observed: "the bastion refused to forward (administratively prohibited)",
				Expected: "a bastion that forwards TCP connections",
				Fix:      "set AllowTcpForwarding yes in sshd_config on the bastion",
				Err:      err,
			})
		}
		// Anything else here is about the address behind the bastion, which this check does not
		// own; the handshake it does own has already succeeded.
	} else {
		_ = forwarded.Close()
	}

	return fmt.Sprintf("ssh handshake with bastion %s as %q succeeded", addr, user), nil
}

// bastionAuth retains the publickey error across methods: x/crypto discards it
// when it tries the password next.
type bastionAuth struct {
	methods       []ssh.AuthMethod
	cleanup       func()
	agentListErr  error
	passwordTried bool
}

func (c BastionAvailabilityCheck) bastionAuthMethods(ctx context.Context, cfg *sshconfig.Config, authSock string) (*bastionAuth, error) {
	signers, err := bastionSigners(cfg.PrivateKeys)
	if err != nil {
		return nil, err
	}

	signers = c.withCertificates(ctx, cfg.PrivateKeys, signers)

	agentConn, cleanup, err := c.dialBastionAgent(ctx, authSock)
	if err != nil {
		return nil, err
	}

	auth := &bastionAuth{cleanup: cleanup}
	// One publickey method for the keys and the agent: x/crypto tries each method name once, so a
	// second publickey method would never be offered.
	if len(signers) > 0 || agentConn != nil {
		auth.methods = append(auth.methods, ssh.PublicKeysCallback(func() ([]ssh.Signer, error) {
			keys, err := c.bastionPublicKeys(ctx, signers, &agentConn)
			auth.agentListErr = err
			return keys, err
		}))
	}

	if cfg.BastionPassword != "" {
		auth.methods = append(auth.methods, ssh.PasswordCallback(func() (string, error) {
			auth.passwordTried = true
			return cfg.BastionPassword, nil
		}))
	}

	if len(auth.methods) == 0 {
		cleanup()
		return nil, fmt.Errorf("dhctl has no private key, no ssh-agent identity and no password for the bastion")
	}

	return auth, nil
}

// bastionPublicKeys orders signers with the identities of the ssh-agent for one handshake.
// x/crypto drops publickey entirely when this returns an error, so a failing agent is only a
// warning while passed keys remain. A failed listing drops the agent for the rest of the handshake.
func (c BastionAvailabilityCheck) bastionPublicKeys(ctx context.Context, signers []ssh.Signer, agentConn *net.Conn) ([]ssh.Signer, error) {
	conn := *agentConn
	if conn == nil {
		return orderBastionSigners(signers, nil), nil
	}

	timeout := c.agentListTimeout
	if timeout <= 0 {
		timeout = bastionAgentListTimeout
	}
	listCtx, cancel := context.WithTimeoutCause(ctx, timeout, errBastionAgentTimeout)
	stop := context.AfterFunc(listCtx, func() { _ = conn.Close() })
	// Keep requests serial: a later publickey callback can reuse this socket, so no background
	// reader may remain to consume its response.
	agentSigners, err := agent.NewClient(struct{ io.ReadWriter }{conn}).Signers()
	// Stop the listing timer before cancelling its context, so a successful listing leaves the
	// connection open for signing. Read the cause before cancel replaces it with context.Canceled.
	stop()
	cause := context.Cause(listCtx)
	cancel()
	if cause != nil {
		err = cause
	}

	if err != nil {
		// A late reply may still be on the socket, and the next call would read it as its own.
		_ = conn.Close()
		*agentConn = nil
		// A cancelled attempt closed the socket itself; that is not the agent's fault.
		if ctx.Err() != nil {
			return nil, context.Cause(ctx)
		}
		if len(signers) > 0 {
			c.warnOnce(ctx, fmt.Sprintf("cannot list ssh-agent identities for the bastion: %v", err))
			return orderBastionSigners(signers, nil), nil
		}
		if errors.Is(err, errBastionAgentTimeout) {
			return nil, err
		}

		return nil, fmt.Errorf("%w: list ssh-agent identities for the bastion: %w", sshutils.ErrSSHCredentials, err)
	}

	return orderBastionSigners(signers, agentSigners), nil
}

func (c BastionAvailabilityCheck) dialBastionAgent(ctx context.Context, authSock string) (net.Conn, func(), error) {
	if authSock == "" {
		return nil, func() {}, nil
	}

	conn, err := (&net.Dialer{Timeout: bastionAgentDialTimeout}).DialContext(ctx, "unix", authSock)
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil, context.Cause(ctx)
		}

		// gossh fails Start when a configured socket cannot be opened. The legacy
		// client leaves authentication to the ssh binary, which can still try a password.
		if c.SSHProviderInitializer.IsLegacyMode() {
			c.warnOnce(ctx, fmt.Sprintf("cannot connect to the ssh-agent at %s for the bastion: %v", authSock, err))
			return nil, func() {}, nil
		}

		return nil, nil, fmt.Errorf("%w: open ssh-agent socket %q: %w", sshutils.ErrSSHCredentials, authSock, err)
	}

	// Signing is outside the listing timeout. A hardware key touch still has to
	// fit within the SSH connection deadline; cancellation closes the agent socket.
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })

	return conn, func() {
		stop()
		_ = conn.Close()
	}, nil
}

// bastionSigners parses the connection config private keys into ssh signers: files of
// --ssh-agent-private-keys (IsPath) and the keys of a connection config as text.
func bastionSigners(keys []sshconfig.AgentPrivateKey) ([]ssh.Signer, error) {
	signers := make([]ssh.Signer, 0, len(keys))
	for _, k := range keys {
		pemBytes := []byte(k.Key)
		if k.IsPath {
			b, err := os.ReadFile(k.Key)
			if err != nil {
				return nil, fmt.Errorf("dhctl could not read the private key %s: %w", k.Key, err)
			}
			pemBytes = b
		}

		var (
			signer ssh.Signer
			err    error
		)
		if k.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(pemBytes, []byte(k.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(pemBytes)
		}
		if err != nil {
			return nil, fmt.Errorf("dhctl could not parse the private key %s: %w", k.Key, err)
		}
		signers = append(signers, signer)
	}

	return signers, nil
}

// withCertificates adds the user certificate in <key>-cert.pub after each key file. signers are
// bastionSigners(keys), in the same order. A broken certificate is only reported, and one outside
// its validity period is still offered: the local clock may be off, the bastion decides.
func (c BastionAvailabilityCheck) withCertificates(ctx context.Context, keys []sshconfig.AgentPrivateKey, signers []ssh.Signer) []ssh.Signer {
	result := make([]ssh.Signer, 0, len(signers))
	for i, k := range keys {
		result = append(result, signers[i])
		if !k.IsPath {
			continue
		}

		certSigner, err := c.bastionCertSigner(ctx, k.Key, signers[i])
		if err != nil {
			c.warnOnce(ctx, fmt.Sprintf("certificate ignored, offering the plain key: %v", err))
			continue
		}

		if certSigner != nil {
			result = append(result, certSigner)
		}
	}

	return result
}

// bastionCertSigner returns a signer for the user certificate in <keyPath>-cert.pub, or nil when
// there is no such file. lib-connection reads the certificate with its own SSH types.
func (c BastionAvailabilityCheck) bastionCertSigner(ctx context.Context, keyPath string, signer ssh.Signer) (ssh.Signer, error) {
	libCert, err := sshutils.ReadUserCertificate(keyPath)
	if err != nil || libCert == nil {
		return nil, err
	}

	certPath := keyPath + "-cert.pub"
	pub, err := ssh.ParsePublicKey(libCert.Marshal())
	if err != nil {
		return nil, fmt.Errorf("parse SSH certificate %q: %w", certPath, err)
	}
	// lib-connection has already checked that the file holds a user certificate.
	certSigner, err := ssh.NewCertSigner(pub.(*ssh.Certificate), signer)
	if err != nil {
		return nil, fmt.Errorf("%w: SSH certificate %q does not match its private key: %w", sshutils.ErrSSHCredentials, certPath, err)
	}

	if warning := sshutils.CertificateValidityWarning(libCert, certPath, time.Now()); warning != "" {
		c.warnOnce(ctx, warning)
	}

	return certSigner, nil
}

// orderBastionSigners offers each passed key, then its certificates, then the rest of the agent, each
// identity once. fileSigners list every key before its own certificate. It mirrors orderIdentities
// in lib-connection pkg/ssh/gossh/authentication.go for x/crypto types; keep the two in step.
func orderBastionSigners(fileSigners, agentSigners []ssh.Signer) []ssh.Signer {
	// Only plain key blobs are ever looked up here, so a certificate in the map is never matched.
	fileKeys := make(map[string]ssh.Signer, len(fileSigners))
	for _, s := range fileSigners {
		fileKeys[string(s.PublicKey().Marshal())] = s
	}

	agentFileCerts := make(map[string][]ssh.Signer, len(fileKeys))
	agentRest := make([]ssh.Signer, 0, len(agentSigners))
	for _, s := range agentSigners {
		keyBlob, certSigner, ok := agentCertForFileKey(s, fileKeys)
		if !ok {
			agentRest = append(agentRest, s)
			continue
		}

		agentFileCerts[keyBlob] = append(agentFileCerts[keyBlob], certSigner)
	}

	ordered := make([]ssh.Signer, 0, len(fileSigners)+len(agentSigners))
	seen := make(map[string]bool, cap(ordered))
	add := func(signers ...ssh.Signer) {
		for _, s := range signers {
			if key := string(s.PublicKey().Marshal()); !seen[key] {
				seen[key] = true
				ordered = append(ordered, s)
			}
		}
	}
	// Agent certificates of a key go after its file certificate.
	var pending []ssh.Signer
	for _, s := range fileSigners {
		if _, ok := s.PublicKey().(*ssh.Certificate); !ok {
			add(pending...)
			pending = agentFileCerts[string(s.PublicKey().Marshal())]
		}

		add(s)
	}

	add(pending...)
	add(agentRest...)

	return ordered
}

// agentCertForFileKey returns an agent certificate of a passed key as a signer of that key, keyed
// by the key's blob: an agent that refuses to sign would end publickey for every later candidate.
func agentCertForFileKey(s ssh.Signer, fileKeys map[string]ssh.Signer) (string, ssh.Signer, bool) {
	// Agent keys are *agent.Key, so a certificate is recognized by parsing its blob.
	pub, err := ssh.ParsePublicKey(s.PublicKey().Marshal())
	if err != nil {
		return "", nil, false
	}

	cert, ok := pub.(*ssh.Certificate)
	if !ok {
		return "", nil, false
	}

	keyBlob := string(cert.Key.Marshal())
	key, ok := fileKeys[keyBlob]
	if !ok {
		return "", nil, false
	}

	certSigner, err := ssh.NewCertSigner(cert, key)
	if err != nil {
		return "", nil, false
	}

	return keyBlob, certSigner, true
}

func BastionAvailability(sshProviderInitializer *providerinitializer.SSHProviderInitializer) preflight.Check {
	check := BastionAvailabilityCheck{SSHProviderInitializer: sshProviderInitializer, warned: &sync.Map{}}
	return preflight.Check{
		Name:        BastionAvailabilityCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Timeout:     preflight.DefaultPreflightCheckTimeout,
		Run:         check.Run,
	}
}

// BastionAvailabilityAfterInfra is the same check, asked again once the infrastructure exists.
//
// On the cloud layouts that create one — OpenStack standard, VCD with-nat, AWS withNAT — the
// bastion is not in the configuration at all until base infrastructure has been applied, so the
// pre-infra instance finds no --ssh-bastion-host and reports itself not applicable. Without a
// second look, a wrong --ssh-bastion-user or key becomes 250 one-second attempts at reaching the
// master, in which the bastion is never mentioned.
func BastionAvailabilityAfterInfra(sshProviderInitializer *providerinitializer.SSHProviderInitializer) preflight.Check {
	check := BastionAvailability(sshProviderInitializer)
	check.Phase = preflight.PhasePostInfra
	return check
}
