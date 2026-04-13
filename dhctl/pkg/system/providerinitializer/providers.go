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
)

type providerOptions struct {
	connectionConfig string
	kubeFlagsDefined bool
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

// func to initialize both SSHProviderInitializer and KubeProvider
func GetProviders(ctx context.Context, params settings.ProviderParams, opts ...ProviderOptions) (*SSHProviderInitializer, libcon.KubeProvider, error) {
	baseProviderSettings := settings.NewBaseProviders(params)

	sshProviderInitializer, err := getProviderInitializer(baseProviderSettings, opts...)
	if err != nil {
		return nil, nil, err
	}

	parser := kube.NewFlagsParser(baseProviderSettings)
	fset := flag.NewFlagSet("my-set", flag.ExitOnError)
	flags, err := parser.InitFlags(fset)
	if err != nil {
		return nil, nil, err
	}
	cfg, err := flags.ExtractConfig()
	if err != nil {
		return nil, nil, err
	}

	runnerInterface, err := provider.GetRunnerInterface(ctx,
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

func getProviderInitializer(baseProviderSettings *settings.BaseProviders, opts ...ProviderOptions) (*SSHProviderInitializer, error) {
	options := &providerOptions{}
	for _, o := range opts {
		o(options)
	}

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
		parser := libcon_config.NewFlagsParser(baseProviderSettings)
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
