// Copyright 2026 Flant JSC
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

package providerinitializer

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"os/user"
	"strconv"

	libcon "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg"
	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/kube"
	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/settings"
	sshconfig "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/ssh/config"
	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/ssh/utils"
	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/utils/file"
	dhlog "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-dhctl/pkg/logger"
	"github.com/hashicorp/go-multierror"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terminal"
)

// errReadPrivateKeys is the text lib-connection gave a key it could not read.
var errReadPrivateKeys = errors.New("Failed to read private keys from flags")

type CLIProvidersOptions struct {
	RequireKube bool
}

// CLIProviders is GetProviders for a dhctl command: both connections come from the options the
// command parsed from argv and the environment.
func CLIProviders(ctx context.Context, opts *options.Options, o CLIProvidersOptions) (*SSHProviderInitializer, libcon.KubeProvider, error) {
	params := app.ProviderParams(&opts.Global, dhlog.FromContext(ctx))

	ssh, err := sshConnectionConfig(ctx, settings.NewBaseProviders(params), &opts.SSH, &opts.Become)
	// A key that cannot be read leaves no SSH when kube flags are given: Kubernetes without SSH.
	if errors.Is(err, errReadPrivateKeys) && opts.Kube.IsDefined() {
		ssh, err = nil, nil
	}

	if err != nil {
		return nil, nil, fmt.Errorf("extract config: %w", err)
	}

	return GetProviders(ctx, params, kubeConfig(&opts.Kube), GetProvidersOptions{SSH: ssh, RequireKube: o.RequireKube})
}

// CLIKubeProviders is the Kubernetes half of CLIProviders, for a command without SSH flags.
func CLIKubeProviders(ctx context.Context, opts *options.Options) (libcon.KubeProvider, error) {
	// Ahead of GetProviders, whose error for this case points at SSH settings. A context alone
	// names nothing to connect to.
	if opts.Kube.Config == "" && !opts.Kube.InCluster {
		return nil, errors.New("no Kubernetes connection: give --kubeconfig or --kube-client-from-cluster")
	}

	params := app.ProviderParams(&opts.Global, dhlog.FromContext(ctx))
	_, kubeProvider, err := GetProviders(ctx, params, kubeConfig(&opts.Kube), GetProvidersOptions{RequireKube: true})

	return kubeProvider, err
}

func kubeConfig(o *options.KubeOptions) *kube.Config {
	return &kube.Config{KubeConfig: o.Config, KubeConfigContext: o.ConfigContext, KubeConfigInCluster: o.InCluster}
}

// CLISSHProviderInitializer is the SSH half of CLIProviders, for commands without Kubernetes.
func CLISSHProviderInitializer(ctx context.Context, opts *options.Options) (*SSHProviderInitializer, error) {
	sett := settings.NewBaseProviders(app.ProviderParams(&opts.Global, dhlog.FromContext(ctx)))

	ssh, err := sshConnectionConfig(ctx, sett, &opts.SSH, &opts.Become)
	if err != nil {
		return nil, fmt.Errorf("extract config: %w", err)
	}

	return NewSSHProviderInitializer(sett, ssh), nil
}

// sshConnectionConfig is the connection o describes, made the way lib-connection v0.16.1
// pkg/ssh/config/parse_flags.go ExtractConfigAfterParse made it. The keys, and the values of
// --connection-config, come resolved from the flag hook; the bastion user follows --ssh-user.
func sshConnectionConfig(ctx context.Context, sett settings.Settings, o *options.SSHOptions, become *options.BecomeOptions) (*sshconfig.ConnectionConfig, error) {
	cfg, err := sshConfig(o)
	if err != nil {
		return nil, err
	}

	if o.ForceNoPrivateKeys && o.UseAgentWithNoPrivateKeys {
		if err := file.IsExists(sett.AuthSock(), "auth socket from env "+settings.SSHAgentAuthSockEnv); err != nil {
			return nil, err
		}
	}

	hosts := make([]sshconfig.Host, 0, len(o.Hosts))
	for _, h := range o.Hosts {
		hosts = append(hosts, sshconfig.Host{Host: h.Host})
	}

	if cfg.PrivateKeys, err = readPrivateKeys(ctx, o); err != nil {
		return nil, fmt.Errorf("%w: %w", errReadPrivateKeys, err)
	}

	if cfg.BastionPassword, err = password(ctx, o.BastionPass, o.AskBastionPass, "bastion"); err != nil {
		return nil, err
	}
	// Without keys and without the agent, the sudo password is the way in.
	noKeysNoAgent := o.ForceNoPrivateKeys && !o.UseAgentWithNoPrivateKeys
	if cfg.SudoPassword, err = password(ctx, become.BecomePass, become.AskBecomePass || noKeysNoAgent, "sudo"); err != nil {
		return nil, err
	}

	connection := &sshconfig.ConnectionConfig{Config: cfg, Hosts: hosts}
	// The hosts may come from the state cache later, so none now is not an error here.
	if err := sshconfig.Validate(connection, false); err != nil {
		return nil, err
	}

	return connection, nil
}

// sshConfig is the part of the connection o gives as is, defaults filled in.
func sshConfig(o *options.SSHOptions) (*sshconfig.Config, error) {
	port, err := sshPort("ssh-port", o.Port)
	if err != nil {
		return nil, err
	}

	bastionPort, err := sshPort("ssh-bastion-port", o.BastionPort)
	if err != nil {
		return nil, err
	}

	userName, bastionUser := o.User, o.BastionUser
	if userName == "" || bastionUser == "" {
		current, err := currentUserName()
		if err != nil {
			return nil, err
		}

		userName, bastionUser = cmp.Or(userName, current), cmp.Or(bastionUser, current)
	}

	return &sshconfig.Config{
		Mode:             sshconfig.Mode{ForceLegacy: o.LegacyMode, ForceModern: o.ModernMode},
		User:             userName,
		Port:             &port,
		ExtraArgs:        o.ExtraArgs,
		BastionHost:      o.BastionHost,
		BastionPort:      &bastionPort,
		BastionUser:      bastionUser,
		ForceUseSSHAgent: o.UseAgentWithNoPrivateKeys,
	}, nil
}

// sshPort reads a port; empty and 0 are the default, as lib-connection's int flag had them.
func sshPort(flag, value string) (int, error) {
	if value == "" {
		return sshconfig.DefaultPort, nil
	}

	port, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("--%s: %q is not a port number", flag, value)
	}

	if port == 0 {
		return sshconfig.DefaultPort, nil
	}

	return port, nil
}

// currentUserName is $USER, else the user running the process, as lib-connection's flags had it.
func currentUserName() (string, error) {
	if name := os.Getenv("USER"); name != "" {
		return name, nil
	}

	current, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("get current user: %w", err)
	}

	return current.Username, nil
}

// readPrivateKeys reads every key the flag hook resolved once, asking for the passphrase of an
// encrypted one unless --connection-config gave it. No key at all from the flags means the default
// one is missing; the --connection-config file has no default key and may list none.
func readPrivateKeys(ctx context.Context, o *options.SSHOptions) ([]sshconfig.AgentPrivateKey, error) {
	noKeys := len(o.PrivateKeys) == 0 && !o.ForceNoPrivateKeys
	if noKeys && o.ConnectionConfigPath == "" {
		return nil, fmt.Errorf("default private key %s is missing", options.DefaultSSHAgentPrivateKeys)
	}

	keys := make([]sshconfig.AgentPrivateKey, 0, len(o.PrivateKeys))
	seen := map[string]bool{}

	var errs *multierror.Error

	for i, key := range o.PrivateKeys {
		if seen[key.Key] {
			continue
		}

		seen[key.Key] = true

		name, data := fmt.Sprintf("%d of --connection-config", i+1), []byte(key.Key)
		if key.IsPath {
			read, err := os.ReadFile(key.Key)
			if err != nil {
				errs = multierror.Append(errs, fmt.Errorf("read private key file: %w", err))
				continue
			}

			name, data = key.Key, read
		}

		passphrase, err := readPrivateKey(ctx, name, data, key.Passphrase, terminal.AskPassword)
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("cannot parse private key %s: %w", name, err))
			continue
		}

		key.Passphrase = passphrase
		keys = append(keys, key)
	}

	if err := errs.ErrorOrNil(); err != nil {
		return nil, err
	}

	return keys, nil
}

// readPrivateKey parses the key data named name and returns the passphrase it takes: the given
// one, or the one ask gets from the terminal when the key is encrypted and none was given.
func readPrivateKey(ctx context.Context, name string, data []byte, passphrase string, ask func(context.Context, string) ([]byte, error)) (string, error) {
	_, err := utils.ParseSSHPrivateKey(ctx, data, name, passphrase)
	if !errors.Is(err, utils.ErrPassphraseMissing) {
		return passphrase, err
	}

	asked, err := ask(ctx, fmt.Sprintf("Enter passphrase for ssh key %q: ", name))
	if err != nil {
		return "", err
	}

	if _, err := utils.ParseSSHPrivateKey(ctx, data, name, string(asked)); err != nil {
		return "", fmt.Errorf("wrong passphrase for ssh key %q: %w", name, err)
	}

	return string(asked), nil
}

// password is the given one, else the one asked for in the terminal when ask is set.
func password(ctx context.Context, given string, ask bool, what string) (string, error) {
	if given != "" {
		return given, nil
	}

	if !ask {
		return "", nil
	}

	answer, err := terminal.AskPassword(ctx, fmt.Sprintf("[%s] Password: ", what))
	if err != nil {
		return "", fmt.Errorf("Cannot get %s password: %w", what, err)
	}

	return string(answer), nil
}
