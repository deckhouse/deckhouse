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

package helper

import (
	"context"

	libcon "github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-connection/pkg/provider"
	"github.com/deckhouse/lib-connection/pkg/settings"
	"github.com/deckhouse/lib-connection/pkg/ssh"
	"github.com/deckhouse/lib-connection/pkg/ssh/local"
)

// get ssh node wrapper if hosts are not empty; otherwise, get local NodeInterface
func GetNodeInterface(sshProviderinitializer provider.SSHProviderInitializer, ctx context.Context, settings *settings.BaseProviders) (libcon.Interface, error) {
	sshProvider, err := sshProviderinitializer.GetSSHProvider(ctx)
	if err != nil {
		return local.NewNodeInterface(settings), nil
	}

	sshClient, err := sshProvider.Client(ctx)
	if err != nil {
		return nil, err
	}

	return ssh.NewNodeInterfaceWrapper(sshClient, settings), nil
}
