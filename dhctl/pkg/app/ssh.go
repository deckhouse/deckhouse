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

package app

import (
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strconv"
	"strings"

	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/settings"
	sshconfig "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/ssh/config"
	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/ssh/session"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

var defaultSSHPort = strconv.Itoa(sshconfig.DefaultPort)

// DefineSSHFlags registers the SSH connection flags into opts.SSH. --connection-config cannot be
// given together with the --ssh-* flags, wherever they come from: argv or environment. Its file
// is read into opts.SSH and opts.Become once the flags are applied.
func DefineSSHFlags(cmd *cobra.Command, opts *options.Options) {
	o := &opts.SSH
	o.Port = defaultSSHPort
	o.BastionPort = defaultSSHPort

	flags := cmd.Flags()
	StringSliceFlag(flags, Flag{
		Name: "ssh-agent-private-keys",
		Env:  "DHCTL_CLI_SSH_AGENT_PRIVATE_KEYS",
		Help: "Paths to private keys. Those keys will be used to connect to servers and to the bastion. Can be specified multiple times (default: '~/.ssh/id_rsa')",
	}, &o.AgentPrivateKeys)
	StringFlag(flags, Flag{
		Name: "ssh-bastion-host",
		Env:  "DHCTL_CLI_SSH_BASTION_HOST",
		Help: "Jumper (bastion) host to connect to servers (will be used both by infrastructure creation utility and ansible). Only IPs or hostnames are supported, name from ssh-config will not work.",
	}, &o.BastionHost)
	StringFlag(flags, Flag{
		Name: "ssh-bastion-port",
		Env:  "DHCTL_CLI_SSH_BASTION_PORT",
		Help: "SSH destination port",
	}, &o.BastionPort)
	StringFlag(flags, Flag{
		Name: "ssh-bastion-user",
		Env:  "DHCTL_CLI_SSH_BASTION_USER",
		Help: "User to authenticate under when connecting to bastion (default: --ssh-user if given, else $USER)",
	}, &o.BastionUser)
	StringFlag(flags, Flag{
		Name: "ssh-user",
		Env:  "DHCTL_CLI_SSH_USER",
		Help: "User to authenticate under (default: $USER)",
	}, &o.User)
	StringSliceFlag(flags, Flag{
		Name: "ssh-host",
		Env:  "DHCTL_CLI_SSH_HOSTS",
		Help: "SSH destination hosts, can be specified multiple times",
	}, &o.HostsRaw)
	StringFlag(flags, Flag{
		Name: "ssh-port",
		Env:  "DHCTL_CLI_SSH_PORT",
		Help: "SSH destination port",
	}, &o.Port)
	StringFlag(flags, Flag{
		Name: "ssh-extra-args",
		Env:  "DHCTL_CLI_SSH_EXTRA_ARGS",
		Help: "extra args for ssh commands (-vvv)",
	}, &o.ExtraArgs)
	StringFlag(flags, Flag{
		Name: "connection-config",
		Env:  "DHCTL_CLI_CONNECTION_CONFIG",
		Help: "SSH connection config file path",
	}, &o.ConnectionConfigPath)
	BoolFlag(flags, Flag{
		Name: "ssh-legacy-mode",
		Env:  "DHCTL_CLI_SSH_LEGACY_MODE",
		Help: "Force legacy SSH mode",
	}, &o.LegacyMode)
	BoolFlag(flags, Flag{
		Name: "ssh-modern-mode",
		Env:  "DHCTL_CLI_SSH_MODERN_MODE",
		Help: "Force modern SSH mode",
	}, &o.ModernMode)
	BoolFlag(flags, Flag{
		Name: "ask-bastion-pass",
		Env:  "DHCTL_CLI_ASK_BASTION_PASS",
		Help: "Ask for bastion password before the installation process.",
	}, &o.AskBastionPass)
	BoolFlag(flags, Flag{
		Name: "force-no-private-keys",
		Env:  "DHCTL_CLI_FORCE_NO_PRIVATE_KEYS",
		Help: "Do not fall back to ~/.ssh/id_rsa when no key is given. Keys from --ssh-agent-private-keys are still used. Without --use-agent-with-no-private-keys the sudo password is asked for.",
	}, &o.ForceNoPrivateKeys)
	BoolFlag(flags, Flag{
		Name: "use-agent-with-no-private-keys",
		Env:  "DHCTL_CLI_USE_AGENT_WITH_NO_PRIVATE_KEYS",
		Help: "Require the SSH agent at SSH_AUTH_SOCK: dhctl stops when the variable is empty. With --force-no-private-keys dhctl also checks that the socket exists and asks for the sudo password only with --ask-become-pass.",
	}, &o.UseAgentWithNoPrivateKeys)

	AfterApply(cmd, func(cmd *cobra.Command) error {
		return applySSHFlags(cmd, opts)
	})
}

// connectionConfigExclusiveFlags are the flags --connection-config cannot be given with, as kingpin
// had them; so is --ssh-user when the bastion user follows it.
var connectionConfigExclusiveFlags = []string{
	"ssh-agent-private-keys",
	"ssh-bastion-host",
	"ssh-bastion-port",
	"ssh-host",
	"ssh-port",
	"ssh-extra-args",
}

// applySSHFlags runs once argv and the environment are in opts.SSH.
func applySSHFlags(cmd *cobra.Command, opts *options.Options) error {
	flags, o := cmd.Flags(), &opts.SSH
	// The bastion is reached as the SSH user unless it has a user of its own.
	bastionFollowsUser := flags.Changed("ssh-user") && !flags.Changed("ssh-bastion-user")
	if bastionFollowsUser {
		o.BastionUser = o.User
	}

	if err := normalizeSSHOptions(flags, o); err != nil {
		return err
	}

	if o.ConnectionConfigPath == "" {
		return nil
	}

	if bastionFollowsUser || slices.ContainsFunc(connectionConfigExclusiveFlags, flags.Changed) {
		return fmt.Errorf("'connection-config' cannot be specified with other ssh flags at the same time")
	}
	// The file gives the keys: a refusal of the default key next to it would ask for a password.
	if o.ForceNoPrivateKeys {
		return fmt.Errorf("'connection-config' cannot be specified with --force-no-private-keys or an empty %s", envOf(flags.Lookup("ssh-agent-private-keys")))
	}

	return config.NewConnectionConfigParser(opts).ParseConnectionConfigFromFile()
}

// normalizeSSHOptions turns the values of the SSH flags into the hosts and the keys the connection
// takes. No key given means the default one, unless keys are refused.
func normalizeSSHOptions(flags *pflag.FlagSet, o *options.SSHOptions) error {
	// Blank items are dropped, as lib-connection's environment lists did.
	o.HostsRaw = slices.DeleteFunc(o.HostsRaw, isBlank)
	for i, host := range o.HostsRaw {
		o.Hosts = append(o.Hosts, session.Host{Host: host, Name: strconv.Itoa(i)})
	}

	o.AgentPrivateKeys = slices.DeleteFunc(o.AgentPrivateKeys, isBlank)
	// A keys variable set to nothing refuses the default key, as lib-connection read it.
	if _, set := os.LookupEnv(envOf(flags.Lookup("ssh-agent-private-keys"))); set && len(o.AgentPrivateKeys) == 0 {
		o.ForceNoPrivateKeys = true
	}

	if len(o.AgentPrivateKeys) == 0 && !o.ForceNoPrivateKeys {
		o.AgentPrivateKeys = append(o.AgentPrivateKeys, options.DefaultSSHAgentPrivateKeys)
	}

	paths, err := options.ParseSSHPrivateKeyPaths(o.AgentPrivateKeys)
	if err != nil {
		return fmt.Errorf("ssh private keys: %w", err)
	}

	o.PrivateKeys = make([]sshconfig.AgentPrivateKey, 0, len(paths))
	for _, path := range paths {
		o.PrivateKeys = append(o.PrivateKeys, sshconfig.AgentPrivateKey{Key: path, IsPath: true})
	}

	return nil
}

func isBlank(s string) bool { return strings.TrimSpace(s) == "" }

// DefineBecomeFlags registers `--ask-become-pass`.
func DefineBecomeFlags(cmd *cobra.Command, o *options.BecomeOptions) {
	// Ansible compatible
	BoolFlag(cmd.Flags(), Flag{
		Name:  "ask-become-pass",
		Short: "K",
		Env:   "DHCTL_CLI_ASK_BECOME_PASS",
		Help:  "Ask for sudo password before the installation process.",
	}, &o.AskBecomePass)
}

func ProviderParams(o *options.GlobalOptions, logger *slog.Logger) settings.ProviderParams {
	return NewProviderParams(logger, o.IsDebug, o.TmpDir)
}

// NewProviderParams is the one place dhctl names what lib-connection needs from the process.
func NewProviderParams(logger *slog.Logger, isDebug bool, tmpDir string) settings.ProviderParams {
	return settings.ProviderParams{
		Logger:      logger,
		IsDebug:     isDebug,
		NodeTmpPath: DeckhouseNodeTmpPath,
		NodeBinPath: DeckhouseNodeBinPath,
		TmpDir:      tmpDir,
		AuthSock:    os.Getenv(settings.SSHAgentAuthSockEnv),
		// The kube-proxy tunnel knobs Deckhouse Commander sets for the dhctl server pod.
		KubeProxyAcceptHosts: os.Getenv("KUBE_PROXY_ACCEPT_HOSTS"),
		KubeProxyBindAddr:    os.Getenv("KUBE_PROXY_BIND_ADDR"),
	}
}
