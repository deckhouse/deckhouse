// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sshclient

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/name212/govalue"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/clissh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/gossh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
)

// Config bundles every SSH-related setting needed to construct an SSH client
// without reaching for package globals. CLI/RPC entry points build it from
// *options.Options and pass it down.
type Config struct {
	// Mode flags: explicit toggles for the SSH backend. When neither is set,
	// the dispatch falls back to "modern unless private keys are provided".
	LegacyMode bool
	ModernMode bool

	// PrivateKeys lists the SSH private-key file paths to load.
	PrivateKeys []string
	// Passphrases optionally maps key path → passphrase.
	Passphrases map[string]string

	// Connection target.
	Hosts       []session.Host
	User        string
	Port        string
	BastionHost string
	BastionPort string
	BastionUser string
	BastionPass string
	ExtraArgs   string

	// BecomePass is forwarded to spawned remote commands for sudo.
	BecomePass string
	// TmpDir is the local scratch directory used by the SSH backend for
	// temp files (bashible bundles, key materialisation).
	TmpDir string
	// IsDebug toggles ssh -vvv style debug logging on every spawned subprocess.
	IsDebug bool
}

// IsModernMode reports whether the modern (gossh) backend should be used.
func (c Config) IsModernMode() bool {
	return c.ModernMode || len(c.PrivateKeys) == 0
}

// IsLegacyMode reports whether the legacy (clissh) backend should be used.
func (c Config) IsLegacyMode() bool {
	return c.LegacyMode || (len(c.PrivateKeys) > 0 && !c.ModernMode)
}

// useLegacy is the shared dispatch decision used by every constructor below.
func (c Config) useLegacy() bool {
	switch {
	case c.LegacyMode:
		return true
	case c.ModernMode:
		return false
	case len(c.PrivateKeys) > 0:
		return true
	default:
		return false
	}
}

// session builds a session.Session populated from c.
func (c Config) session() *session.Session {
	return session.NewSession(session.Input{
		AvailableHosts:  c.Hosts,
		User:            c.User,
		Port:            c.Port,
		BastionHost:     c.BastionHost,
		BastionPort:     c.BastionPort,
		BastionUser:     c.BastionUser,
		BastionPassword: c.BastionPass,
		ExtraArgs:       c.ExtraArgs,
	})
}

// privateKeys converts c.PrivateKeys into the AgentPrivateKey form expected
// by lower-level SSH clients. Passphrases stay attached to the Client (via
// the shared Passphrases map) and are not embedded here.
func (c Config) privateKeys() []session.AgentPrivateKey {
	keys := make([]session.AgentPrivateKey, 0, len(c.PrivateKeys))
	for _, k := range c.PrivateKeys {
		keys = append(keys, session.AgentPrivateKey{Key: k})
	}
	return keys
}

func (c Config) clisshConfig() clissh.Config {
	return clissh.Config{
		BecomePass:  c.BecomePass,
		TmpDir:      c.TmpDir,
		IsDebug:     c.IsDebug,
		Passphrases: c.Passphrases,
	}
}

func (c Config) gosshConfig() gossh.Config {
	return gossh.Config{
		BecomePass:  c.BecomePass,
		TmpDir:      c.TmpDir,
		IsDebug:     c.IsDebug,
		Passphrases: c.Passphrases,
	}
}

type SSHProviderFunc func() (node.SSHClient, error)

type SSHProvider interface {
	Client() (node.SSHClient, error)
	SwitchClient(ctx context.Context, sess *session.Session, privateKeys []session.AgentPrivateKey, oldSSHClient node.SSHClient) (node.SSHClient, error)
}

type DefaultSSHProviderWithFunc struct {
	provider       SSHProviderFunc
	clientOpts     *ClientOptions
	cfg            Config
	loggerProvider log.LoggerProvider

	mu                     sync.Mutex
	legacyPrivateKeysAdded map[string]struct{}
}

func NewDefaultSSHProviderWithFunc(provider SSHProviderFunc) *DefaultSSHProviderWithFunc {
	return &DefaultSSHProviderWithFunc{
		provider:               provider,
		clientOpts:             nil,
		legacyPrivateKeysAdded: make(map[string]struct{}),
		loggerProvider:         log.SilentLoggerProvider(),
	}
}

func (p *DefaultSSHProviderWithFunc) WithOptions(opts *ClientOptions) *DefaultSSHProviderWithFunc {
	p.clientOpts = opts
	return p
}

// WithConfig records the SSH Config used by SwitchClient when constructing a
// replacement client. Returns the receiver for chaining.
func (p *DefaultSSHProviderWithFunc) WithConfig(cfg Config) *DefaultSSHProviderWithFunc {
	p.cfg = cfg
	return p
}

func (p *DefaultSSHProviderWithFunc) WithLoggerProvider(provider log.LoggerProvider) *DefaultSSHProviderWithFunc {
	p.loggerProvider = provider
	return p
}

func (p *DefaultSSHProviderWithFunc) Client() (node.SSHClient, error) {
	if govalue.IsNil(p.provider) {
		return nil, fmt.Errorf("SSH provider not passed")
	}

	client, err := p.provider()
	if err != nil {
		return nil, err
	}

	// only add key for provide ssh
	_ = p.getAndAddPrivateKeysAdded(client.PrivateKeys())

	return client, nil
}

func (p *DefaultSSHProviderWithFunc) SwitchClient(ctx context.Context, sess *session.Session, privateKeys []session.AgentPrivateKey, oldSSHClient node.SSHClient) (node.SSHClient, error) {
	if p.cfg.IsModernMode() {
		logger := p.logger()
		logger.LogDebugF("Old SSH Client: %-v\n", oldSSHClient)
		logger.LogDebugLn("Stopping old SSH client")
		oldSSHClient.Stop()

		sleep := 10 * time.Second
		logger.LogInfoF("Switch to new client. Waiting for '%s' for stopped old SSH client\n", sleep)
		// todo ugly solution we need to add waiting function after stop in clients
		// wait for keep-alive goroutine will exit
		time.Sleep(sleep)
	}

	privateKeysToAdd := p.getAndAddPrivateKeysAdded(privateKeys)

	clientOpts := ClientOptions{}
	if p.clientOpts != nil {
		clientOpts = *p.clientOpts
	}
	return newClientFromSession(ctx, sess, privateKeysToAdd, clientOpts, p.cfg), nil
}

func (p *DefaultSSHProviderWithFunc) logger() log.Logger {
	return log.SafeProvideLogger(p.loggerProvider)
}

func (p *DefaultSSHProviderWithFunc) getAndAddPrivateKeysAdded(newKeys []session.AgentPrivateKey) []session.AgentPrivateKey {
	p.mu.Lock()
	defer p.mu.Unlock()

	modernNode := p.cfg.IsModernMode()
	initNewAgent := p.isInitNewAgent()
	// for modern mode add new agent
	if modernNode || initNewAgent {
		p.logger().LogDebugF("Return all keys: modern mode = '%v'; init new agent = '%v'\n", modernNode, initNewAgent)
		return newKeys
	}

	toAdd := make([]session.AgentPrivateKey, 0, len(newKeys))
	for _, key := range newKeys {
		keyPath := key.Key
		if _, ok := p.legacyPrivateKeysAdded[keyPath]; !ok {
			toAdd = append(toAdd, key)
			p.legacyPrivateKeysAdded[keyPath] = struct{}{}
		}
	}

	return toAdd
}

func (p *DefaultSSHProviderWithFunc) isInitNewAgent() bool {
	if govalue.IsNil(p.clientOpts) {
		return false
	}

	return p.clientOpts.InitializeNewAgent
}

type ClientOptions struct {
	InitializeNewAgent bool
}

// NewInitClientFromConfig builds an SSH client from cfg and runs Start().
// It is the Config-based replacement for the deprecated NewInitClientFromFlags.
//
// Callers that need the operator to enter the sudo password interactively
// should call terminal.AskBecomePassword themselves (against the actual
// *options.BecomeOptions) before threading the result into cfg.BecomePass.
func NewInitClientFromConfig(ctx context.Context, cfg Config) (node.SSHClient, error) {
	if len(cfg.Hosts) == 0 {
		return nil, nil
	}

	client := newClientFromSession(ctx, cfg.session(), cfg.privateKeys(), ClientOptions{}, cfg)
	if err := client.Start(); err != nil {
		return nil, err
	}

	return client, nil
}

// NewInitClientFromConfigWithHosts is NewInitClientFromConfig but errors when
// no hosts are configured.
func NewInitClientFromConfigWithHosts(ctx context.Context, cfg Config) (node.SSHClient, error) {
	if len(cfg.Hosts) == 0 {
		return nil, fmt.Errorf("Hosts not passed")
	}
	return NewInitClientFromConfig(ctx, cfg)
}

// NewClient builds an SSH client from an explicit Session/key set. cfg
// supplies the per-Client knobs (BecomePass, TmpDir, IsDebug, Passphrases,
// mode flags).
func NewClient(ctx context.Context, sess *session.Session, privateKeys []session.AgentPrivateKey, cfg Config) node.SSHClient {
	return newClientFromSession(ctx, sess, privateKeys, ClientOptions{}, cfg)
}

// NewClientWithOptions is NewClient with extra knobs (currently just
// InitializeNewAgent).
func NewClientWithOptions(ctx context.Context, sess *session.Session, privateKeys []session.AgentPrivateKey, clientOptions ClientOptions, cfg Config) node.SSHClient {
	return newClientFromSession(ctx, sess, privateKeys, clientOptions, cfg)
}

func newClientFromSession(ctx context.Context, sess *session.Session, privateKeys []session.AgentPrivateKey, clientOptions ClientOptions, cfg Config) node.SSHClient {
	if cfg.useLegacy() {
		return clissh.NewClientFromConfig(sess, privateKeys, clientOptions.InitializeNewAgent, cfg.clisshConfig())
	}
	return gossh.NewClientFromConfig(ctx, sess, privateKeys, cfg.gosshConfig())
}

// NewClientFromConfig builds a non-init SSH client from cfg.
func NewClientFromConfig(ctx context.Context, cfg Config) (node.SSHClient, error) {
	return newClientFromSession(ctx, cfg.session(), cfg.privateKeys(), ClientOptions{}, cfg), nil
}

// NewClientFromConfigWithHosts is NewClientFromConfig but errors when no hosts
// are configured.
func NewClientFromConfigWithHosts(ctx context.Context, cfg Config) (node.SSHClient, error) {
	if len(cfg.Hosts) == 0 {
		return nil, fmt.Errorf("Hosts not passed")
	}
	return NewClientFromConfig(ctx, cfg)
}

// Credentials carries the per-host SSH credentials used by
// NewClientFromCredentials when constructing a one-shot SSH client (e.g. for
// the static-instance flow).
type Credentials struct {
	User                string
	SSHPort             int
	PrivateSSHKey       string
	SudoPasswordEncoded string
	BastionHost         string
	BastionPort         string
	BastionUser         string
	BastionPassword     string
	BastionKeys         []session.AgentPrivateKey
}

// NewClientFromCredentials builds a one-shot SSH client targeting `host` with
// the supplied credentials. tmpDir locates the scratch directory used for
// the temporary key file; pass options.DefaultTmpDir() / opts.Global.TmpDir.
func NewClientFromCredentials(ctx context.Context, host string, cred Credentials, tmpDir string) (node.SSHClient, error) {
	input := session.Input{
		AvailableHosts:  []session.Host{{Host: host}},
		User:            cred.User,
		Port:            strconv.Itoa(cred.SSHPort),
		BecomePass:      cred.SudoPasswordEncoded,
		BastionHost:     cred.BastionHost,
		BastionPort:     cred.BastionPort,
		BastionUser:     cred.BastionUser,
		BastionPassword: cred.BastionPassword,
	}

	var keys []session.AgentPrivateKey
	keys = append(keys, cred.BastionKeys...)

	if cred.PrivateSSHKey != "" {
		tmpFile, err := os.CreateTemp(tmpDir, "sshkey-for-staticinstance-*")
		if err != nil {
			return nil, fmt.Errorf("Cannot create temp file for SSH key: %w", err)
		}
		defer tmpFile.Close()

		if _, err = tmpFile.WriteString(cred.PrivateSSHKey); err != nil {
			return nil, fmt.Errorf("Cannot write SSH key to temp file: %w", err)
		}

		keys = append(keys, session.AgentPrivateKey{Key: tmpFile.Name()})
	}

	settings := session.NewSession(input)
	client := gossh.NewClientFromConfig(ctx, settings, keys, gossh.Config{
		TmpDir: tmpDir,
	})
	return client, nil
}

// IsModernMode reports the current dispatch decision. Prefer Config.IsModernMode
// where the Config is available.
func IsModernMode(cfg Config) bool {
	return cfg.IsModernMode()
}

// IsLegacyMode reports the current dispatch decision. Prefer Config.IsLegacyMode
// where the Config is available.
func IsLegacyMode(cfg Config) bool {
	return cfg.IsLegacyMode()
}
