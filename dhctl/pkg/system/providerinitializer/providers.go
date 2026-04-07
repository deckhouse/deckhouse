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
	"os"
	"strings"

	flag "github.com/spf13/pflag"

	"github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-connection/pkg/kube"
	"github.com/deckhouse/lib-connection/pkg/provider"
	"github.com/deckhouse/lib-connection/pkg/settings"
	libcon_config "github.com/deckhouse/lib-connection/pkg/ssh/config"
)

type providerOptions struct {
	connectionConfig string
}

type ProviderOptions func(o *providerOptions)

func WithConnectionConfig(s string) ProviderOptions {
	return func(o *providerOptions) {
		o.connectionConfig = s
	}
}

// func to initialize both SSHProviderInitializer and KubeProvider
func GetProviders(ctx context.Context, params settings.ProviderParams, opts ...ProviderOptions) (*SSHProviderInitializer, pkg.KubeProvider, error) {
	sett := settings.NewBaseProviders(params)

	options := &providerOptions{}
	for _, o := range opts {
		o(options)
	}

	var config *libcon_config.ConnectionConfig
	var err error
	if len(options.connectionConfig) > 0 {
		config, err = libcon_config.ParseConnectionConfig(strings.NewReader(options.connectionConfig), sett, libcon_config.ParseWithRequiredSSHHost(false))
		if err != nil {
			return nil, nil, err
		}
	} else {
		parser := libcon_config.NewFlagsParser(sett)
		fset := flag.NewFlagSet("my-set", flag.ExitOnError)
		flags, err := parser.InitFlags(fset)
		if err != nil {
			return nil, nil, err
		}
		config, err = flags.ExtractConfig(os.Args[1:])
		if err != nil {
			return nil, nil, err
		}
	}

	sshProviderInitializer := NewSSHProviderInitializer(sett, config)

	cfg := &kube.Config{}
	runnerInterface, err := provider.GetRunnerInterface(ctx, cfg, sett, sshProviderInitializer)
	if err != nil {
		return sshProviderInitializer, nil, err
	}
	kubeProvider := provider.NewDefaultKubeProvider(sett, cfg, runnerInterface)

	return sshProviderInitializer, kubeProvider, nil
}
