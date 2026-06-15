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
	"context"
	"fmt"
	"os"
	"strings"

	flag "github.com/spf13/pflag"

	libcon "github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-connection/pkg/kube"
	"github.com/deckhouse/lib-connection/pkg/provider"
	"github.com/deckhouse/lib-connection/pkg/settings"
	libcon_config "github.com/deckhouse/lib-connection/pkg/ssh/config"

	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

type providerOptions struct {
	connectionConfig    string
	kubeFlagsDefined    bool
	requireKubeProvider bool
	kubeConfig          *kube.Config
}

type ProviderOptions func(o *providerOptions)

func WithConnectionConfig(s string) ProviderOptions {
	return func(o *providerOptions) {
		o.connectionConfig = s
	}
}

func WithKubeFlagsDefined(b bool) ProviderOptions {
	return func(o *providerOptions) {
		o.kubeFlagsDefined = b
	}
}

func WithRequiredKubeProvider() ProviderOptions {
	return func(o *providerOptions) {
		o.requireKubeProvider = true
	}
}

// WithKubeConfig makes GetProviders use the supplied kube settings as the
// source of truth instead of re-parsing CLI/env flags through lib-connection.
// Callers (dhctl commands) already have the values populated in opts.Kube by
// kingpin, including programmatic overrides; re-parsing flags inside the
// initializer dropped any value that wasn't on os.Args / env at call time.
func WithKubeConfig(kubeConfig, kubeConfigContext string, inCluster bool) ProviderOptions {
	return func(o *providerOptions) {
		o.kubeConfig = &kube.Config{
			KubeConfig:          kubeConfig,
			KubeConfigContext:   kubeConfigContext,
			KubeConfigInCluster: inCluster,
		}
	}
}

func GetSSHProviderInitializer(ctx context.Context, params settings.ProviderParams, opts ...ProviderOptions) (*SSHProviderInitializer, error) {
	baseProviderSettings := settings.NewBaseProviders(params)
	return getProviderInitializer(baseProviderSettings, opts...)
}

// func to initialize both SSHProviderInitializer and KubeProvider
func GetProviders(ctx context.Context, params settings.ProviderParams, opts ...ProviderOptions) (*SSHProviderInitializer, libcon.KubeProvider, error) {
	options := newProviderOptions(opts...)
	baseProviderSettings := settings.NewBaseProviders(params)

	sshProviderInitializer, err := getProviderInitializer(baseProviderSettings, opts...)
	if err != nil {
		return nil, nil, err
	}

	cfg, err := resolveKubeConfig(baseProviderSettings, options)
	if err != nil {
		return nil, nil, err
	}

	if options.requireKubeProvider && cfg.OverSSH() {
		if sshProviderInitializer == nil || !sshProviderInitializer.CheckHosts() {
			return sshProviderInitializer, nil, ErrSSHHostRequiredForKubernetesConnection
		}
	}

	runnerInterface, err := provider.GetRunnerInterface(
		ctx,
		cfg,
		baseProviderSettings,
		sshProviderInitializer,
	)
	if err != nil {
		return sshProviderInitializer, nil, err
	}
	kubeProvider := provider.NewDefaultKubeProvider(baseProviderSettings, cfg, runnerInterface)

	return sshProviderInitializer, kubeProvider, nil
}

// resolveKubeConfig returns the kube.Config GetProviders should use.
// When WithKubeConfig was supplied, the caller's already-parsed kube settings
// (opts.Kube populated by kingpin, including programmatic overrides) win and we
// skip lib-connection's parser entirely. Otherwise we fall back to the legacy
// flag-parsing path for callers that don't have a parsed options struct
// (notably the server path that drives connections from a config blob).
func resolveKubeConfig(baseProviderSettings *settings.BaseProviders, options *providerOptions) (*kube.Config, error) {
	if options.kubeConfig != nil {
		return options.kubeConfig, nil
	}

	parser := kube.NewFlagsParser(baseProviderSettings)
	fset := flag.NewFlagSet("my-set", flag.ExitOnError)
	flags, err := parser.InitFlags(fset)
	if err != nil {
		return nil, err
	}
	return flags.ExtractConfig()
}

func getProviderInitializer(baseProviderSettings *settings.BaseProviders, opts ...ProviderOptions) (*SSHProviderInitializer, error) {
	options := newProviderOptions(opts...)

	var config *libcon_config.ConnectionConfig
	var err error
	var sshProviderInitializer *SSHProviderInitializer
	if len(options.connectionConfig) > 0 {
		config, err = libcon_config.ParseConnectionConfig(
			strings.NewReader(options.connectionConfig),
			baseProviderSettings,
			libcon_config.ParseWithRequiredSSHHost(false),
		)
		if err != nil {
			return nil, err
		}
	} else {
		// loggerProvider should be forced to non-interactive to ask for password, because our wrapper hides all Info* and Warn* output
		sett := baseProviderSettings.Clone()
		loggerProvider := log.NonInteractiveLoggerProvider()
		sett.WithLogger(loggerProvider)
		parser := libcon_config.NewFlagsParser(sett)
		parser.WithEnvsPrefix(global.SSHEnvsPrefix)
		fset := flag.NewFlagSet("my-set", flag.ExitOnError)
		flags, err := parser.InitFlags(fset)
		if err != nil {
			return nil, fmt.Errorf("init flags: %w", err)
		}
		config, err = flags.ExtractConfig(os.Args[1:])
		if err != nil {
			if strings.Contains(err.Error(), "Failed to read private keys from flags") && options.kubeFlagsDefined {
				return nil, nil
			}
			return nil, fmt.Errorf("extract config: %w", err)
		}
	}

	sshProviderInitializer = NewSSHProviderInitializer(baseProviderSettings, config)
	return sshProviderInitializer, nil
}

func newProviderOptions(opts ...ProviderOptions) *providerOptions {
	options := &providerOptions{}
	for _, o := range opts {
		o(options)
	}
	return options
}
