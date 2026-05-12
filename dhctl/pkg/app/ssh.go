// Copyright 2021 Flant JSC
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

package app

import (
	"fmt"
	"strconv"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/lib-connection/pkg/settings"
	libdhctl_log "github.com/deckhouse/lib-dhctl/pkg/log"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
)

type connectionConfigParser interface {
	ParseConnectionConfigFromFile() error
}

// DefineSSHFlags registers SSH connection flags, writing into o.
//
// The optional parser is invoked when --connection-config is set; it is the
// caller's responsibility to apply the parsed values back into o (the parser
// implementation lives in dhctl/pkg/config and may need updating to take *options.SSHOptions
// directly once the consumer-side refactor lands).
func DefineSSHFlags(cmd *kingpin.CmdClause, o *options.SSHOptions, parser connectionConfigParser) {
	var sshFlagSetByUser, sshUserFlagSetByUser, sshBastionUserFlagSetByUser bool

	cmd.Flag("ssh-agent-private-keys", "Paths to private keys. Those keys will be used to connect to servers and to the bastion. Can be specified multiple times (default: '~/.ssh/id_rsa')").
		IsSetByUser(&sshFlagSetByUser).
		Envar(configEnvName("SSH_AGENT_PRIVATE_KEYS")).
		StringsVar(&o.AgentPrivateKeys)
	cmd.Flag("ssh-bastion-host", "Jumper (bastion) host to connect to servers (will be used both by infrastructure creation utility and ansible). Only IPs or hostnames are supported, name from ssh-config will not work.").
		IsSetByUser(&sshFlagSetByUser).
		Envar(configEnvName("SSH_BASTION_HOST")).
		StringVar(&o.BastionHost)
	cmd.Flag("ssh-bastion-port", "SSH destination port").
		Default("22").
		IsSetByUser(&sshFlagSetByUser).
		Envar(configEnvName("SSH_BASTION_PORT")).
		StringVar(&o.BastionPort)
	cmd.Flag("ssh-bastion-user", "User to authenticate under when connecting to bastion (default: $USER)").
		IsSetByUser(&sshBastionUserFlagSetByUser).
		Default(o.BastionUser).
		Envar(configEnvName("SSH_BASTION_USER")).
		StringVar(&o.BastionUser)
	cmd.Flag("ssh-user", "User to authenticate under (default: $USER)").
		IsSetByUser(&sshUserFlagSetByUser).
		Envar(configEnvName("SSH_USER")).
		Default(o.User).
		StringVar(&o.User)
	cmd.Flag("ssh-host", "SSH destination hosts, can be specified multiple times").
		IsSetByUser(&sshFlagSetByUser).
		Envar(configEnvName("SSH_HOSTS")).
		StringsVar(&o.HostsRaw)
	cmd.Flag("ssh-port", "SSH destination port").
		Default("22").
		IsSetByUser(&sshFlagSetByUser).
		Envar(configEnvName("SSH_PORT")).
		StringVar(&o.Port)
	cmd.Flag("ssh-extra-args", "extra args for ssh commands (-vvv)").
		IsSetByUser(&sshFlagSetByUser).
		Envar(configEnvName("SSH_EXTRA_ARGS")).
		StringVar(&o.ExtraArgs)
	cmd.Flag("connection-config", "SSH connection config file path").
		Envar(configEnvName("CONNECTION_CONFIG")).
		StringVar(&o.ConnectionConfigPath)
	cmd.Flag("ssh-legacy-mode", "Force legacy SSH mode").
		Envar(configEnvName("SSH_LEGACY_MODE")).
		BoolVar(&o.LegacyMode)
	cmd.Flag("ssh-modern-mode", "Force modern SSH mode").
		Envar(configEnvName("SSH_MODERN_MODE")).
		BoolVar(&o.ModernMode)
	cmd.Flag("ask-bastion-pass", "Ask for bastion password before the installation process.").
		Envar(configEnvName("ASK_BASTION_PASS")).
		BoolVar(&o.AskBastionPass)

	cmd.PreAction(func(c *kingpin.ParseContext) error {
		if !sshBastionUserFlagSetByUser && sshUserFlagSetByUser {
			o.BastionUser = o.User
			sshFlagSetByUser = true
		}
		return nil
	})
	cmd.PreAction(func(c *kingpin.ParseContext) error {
		if len(o.HostsRaw) > 0 {
			for i, host := range o.HostsRaw {
				o.Hosts = append(o.Hosts, session.Host{Host: host, Name: strconv.Itoa(i)})
			}
		}
		return nil
	})

	cmd.Action(func(c *kingpin.ParseContext) error {
		if len(o.ConnectionConfigPath) == 0 {
			return nil
		}

		if sshFlagSetByUser {
			return fmt.Errorf("'connection-config' cannot be specified with other ssh flags at the same time")
		}

		if parser == nil {
			return nil
		}

		return parser.ParseConnectionConfigFromFile()
	})

	cmd.PreAction(func(c *kingpin.ParseContext) error {
		if len(o.PrivateKeys) != 0 {
			return nil
		}
		return o.ProcessConnectionConfigFlags()
	})
	cmd.PreAction(func(c *kingpin.ParseContext) error {
		// Need to read AskBecomePass cross-section: that flag lives in
		// options.BecomeOptions but the legacy-mode validation depends on it.
		// The Become struct is part of *options.Options but not threaded into
		// DefineSSHFlags directly, so the pkg/app caller is responsible for
		// adding any cross-section validation if it needs the legacy guard.
		return nil
	})
}

// DefineBecomeFlags registers `--ask-become-pass`.
func DefineBecomeFlags(cmd *kingpin.CmdClause, o *options.BecomeOptions) {
	// Ansible compatible
	cmd.Flag("ask-become-pass", "Ask for sudo password before the installation process.").
		Envar(configEnvName("ASK_BECOME_PASS")).
		Short('K').
		BoolVar(&o.AskBecomePass)
}

// ProviderParams builds settings.ProviderParams from o and the given logger.
//
// Lives in pkg/app (not options) because it depends on the deckhouse-node
// directory constants defined here, which would create an import cycle
// if pulled into options.
func ProviderParams(o *options.GlobalOptions, loggerProvider libdhctl_log.LoggerProvider) settings.ProviderParams {
	return settings.ProviderParams{
		LoggerProvider: loggerProvider,
		IsDebug:        o.IsDebug,
		NodeTmpPath:    DeckhouseNodeTmpPath,
		NodeBinPath:    DeckhouseNodeBinPath,
		TmpDir:         options.DefaultTmpDir(),
	}
}

// DefaultProviderParams is ProviderParams with the default global logger.
func DefaultProviderParams(o *options.GlobalOptions) (settings.ProviderParams, error) {
	logger, ok := log.GetDefaultLogger().(*log.ExternalLogger)
	if !ok {
		return settings.ProviderParams{}, fmt.Errorf("cannot convert logger to ExternalLogger")
	}
	loggerProvider := libdhctl_log.SimpleLoggerProvider(logger.GetLogger())
	return ProviderParams(o, loggerProvider), nil
}
