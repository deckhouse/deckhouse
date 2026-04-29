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

package bundle

import (
	"context"
	"fmt"

	"github.com/deckhouse/lib-dhctl/pkg/log"

	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/cmd"
	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

type RegistryParams struct {
	BundlePath     string
	LoggerProvider log.LoggerProvider
}

func (params RegistryParams) Validate() error {
	if params.BundlePath == "" {
		return fmt.Errorf("bundle path is required")
	}

	if params.LoggerProvider == nil {
		return fmt.Errorf("logger provider is required")
	}

	return nil
}

type StopRegistry func()

func StartRegistry(ctx context.Context, params RegistryParams) (StopRegistry, error) {
	if err := params.Validate(); err != nil {
		return nil, err
	}

	logger := params.LoggerProvider()

	cfg := cmd.ServerConfig{
		Bundle: cmd.BundleConfig{
			Logger:     newLogger(logger, true),
			BundlePath: params.BundlePath,
		},
		Registry: cmd.RegistryConfig{
			Logger:   newLogger(logger, false),
			RepoPath: constant.BundleRepoPath,
			HTTP: cmd.HTTPServerConfig{
				Address: constant.BundleAddressWithPort,
			},
		},
	}

	serv, err := cmd.NewServer(ctx, cfg)
	if err != nil {
		return nil, err
	}

	return func() {
		serv.Stop(context.Background())
	}, nil
}
