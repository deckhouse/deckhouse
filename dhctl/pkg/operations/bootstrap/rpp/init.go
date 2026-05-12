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

package rpp

import (
	"context"

	libcon "github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-connection/pkg/ssh"
	"github.com/deckhouse/lib-dhctl/pkg/log"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config/directoryconfig"
)

type Cleanup func()

type InitOpt func(*RegistryPackagesProxy)

type InitParams struct {
	MetaConfig     *config.MetaConfig
	Node           libcon.Interface
	LoggerProvider log.LoggerProvider
	SignCheck      bool
	DirsConfig     *directoryconfig.DirectoryConfig
}

func noCleanup() {}

func Init(ctx context.Context, params InitParams, opts ...InitOpt) (Cleanup, error) {
	configGetter := NewClientConfigGetter(params.MetaConfig.Registry.Settings.RemoteData)

	clusterDomain := params.MetaConfig.GetClusterDomain()

	rpp := NewRegistryPackagesProxy(clusterDomain, configGetter, params.LoggerProvider).
		WithSignCheck(params.SignCheck).
		WithDirectoryConfig(params.DirsConfig)

	for _, o := range opts {
		o(rpp)
	}

	if err := rpp.Start(ctx); err != nil {
		return noCleanup, err
	}

	if wrapper, ok := params.Node.(*ssh.NodeInterfaceWrapper); ok {
		if err := rpp.upTunnel(ctx, wrapper.Client()); err != nil {
			rpp.Stop()
			return noCleanup, err
		}
	}

	return func() {
		rpp.Stop()
	}, nil
}
