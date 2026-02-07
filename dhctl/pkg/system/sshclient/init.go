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

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/clissh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/gossh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
)

type SSHProviderFunc func() (node.SSHClient, error)

type SSHProvider interface {
	Client() (node.SSHClient, error)
	SwitchClient(ctx context.Context, sess *session.Session, privateKeys []session.AgentPrivateKey, oldSSHClient node.SSHClient) (node.SSHClient, error)
}

type DefaultSSHProviderWithFunc struct {
	provider       SSHProviderFunc
	opts           *ClientOptions
	loggerProvider log.LoggerProvider

	mu                     sync.Mutex
	legacyPrivateKeysAdded map[string]struct{}
}

func NewDefaultSSHProviderWithFunc(provider SSHProviderFunc) *DefaultSSHProviderWithFunc {
	return &DefaultSSHProviderWithFunc{
		provider:               provider,
		opts:                   nil,
		legacyPrivateKeysAdded: make(map[string]struct{}),
		loggerProvider:         log.SilentLoggerProvider(),
	}
}

func (p *DefaultSSHProviderWithFunc) WithOptions(opts *ClientOptions) *DefaultSSHProviderWithFunc {
	p.opts = opts
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
	if IsModernMode() {
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

	if p.opts != nil {
		return NewClientWithOptions(ctx, sess, privateKeysToAdd, *p.opts), nil
	}
	return NewClient(ctx, sess, privateKeysToAdd), nil
}

func (p *DefaultSSHProviderWithFunc) logger() log.Logger {
	return log.SafeProvideLogger(p.loggerProvider)
}

func (p *DefaultSSHProviderWithFunc) getAndAddPrivateKeysAdded(newKeys []session.AgentPrivateKey) []session.AgentPrivateKey {
	p.mu.Lock()
	defer p.mu.Unlock()

	modernNode := IsModernMode()
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
	if govalue.IsNil(p.opts) {
		return false
	}

	return p.opts.InitializeNewAgent
}

type ClientOptions struct {
	InitializeNewAgent bool
}

func NewInitClientFromFlags(ctx context.Context, askPassword bool) (node.SSHClient, error) {
	switch {
	case app.SSHLegacyMode:
		// if set --ssh-legacy-mode
		return clissh.NewInitClientFromFlags(askPassword)
	case app.SSHModernMode:
		// if set --ssh-modern-mode
		return gossh.NewInitClientFromFlags(ctx, askPassword)
	case len(app.SSHPrivateKeys) > 0:
		// if flags doesn't set, but we have private keys
		return clissh.NewInitClientFromFlags(askPassword)
	default:
		return gossh.NewInitClientFromFlags(ctx, askPassword)
	}
}

func NewInitClientFromFlagsWithHosts(ctx context.Context, askPassword bool) (node.SSHClient, error) {
	if len(app.SSHHosts) == 0 {
		return nil, fmt.Errorf("Hosts not passed")
	}

	return NewInitClientFromFlags(ctx, askPassword)
}

func NewClient(ctx context.Context, sess *session.Session, privateKeys []session.AgentPrivateKey) node.SSHClient {
	return NewClientWithOptions(ctx, sess, privateKeys, ClientOptions{})
}

func NewClientWithOptions(ctx context.Context, sess *session.Session, privateKeys []session.AgentPrivateKey, clientOptions ClientOptions) node.SSHClient {
	switch {
	case app.SSHLegacyMode:
		// if set --ssh-legacy-mode
		client := clissh.NewClient(sess, privateKeys, clientOptions.InitializeNewAgent)
		return client
	case app.SSHModernMode:
		// if set --ssh-modern-mode
		return gossh.NewClient(ctx, sess, privateKeys)
	case len(app.SSHPrivateKeys) > 0:
		// if flags doesn't set, but we have private keys
		client := clissh.NewClient(sess, privateKeys, clientOptions.InitializeNewAgent)
		return client
	default:
		return gossh.NewClient(ctx, sess, privateKeys)
	}
}

func NewClientFromFlags(ctx context.Context) (node.SSHClient, error) {
	switch {
	case app.SSHLegacyMode:
		return clissh.NewClientFromFlags(), nil
	case app.SSHModernMode:
		return gossh.NewClientFromFlags(ctx)
	case len(app.SSHPrivateKeys) > 0:
		return clissh.NewClientFromFlags(), nil
	default:
		return gossh.NewClientFromFlags(ctx)
	}
}

func NewClientFromFlagsWithHosts(ctx context.Context) (node.SSHClient, error) {
	if len(app.SSHHosts) == 0 {
		return nil, fmt.Errorf("Hosts not passed")
	}

	return NewClientFromFlags(ctx)
}

type ClientConfig struct {
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

func NewClientFromConfig(ctx context.Context, host string, cred ClientConfig) (node.SSHClient, error) {
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
		tmpFile, err := os.CreateTemp(app.TmpDirName, "sshkey-for-staticinstance-*")
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
	client := gossh.NewClient(ctx, settings, keys)
	return client, nil
}

func IsModernMode() bool {
	return app.SSHModernMode || len(app.SSHPrivateKeys) == 0
}

func IsLegacyMode() bool {
	return app.SSHLegacyMode || (len(app.SSHPrivateKeys) > 0 && !app.SSHModernMode)
}
