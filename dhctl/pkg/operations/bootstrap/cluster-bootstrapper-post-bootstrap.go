// Copyright 2023 Flant JSC
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

package bootstrap

import (
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
)

func (b *ClusterBootstrapper) ExecPostBootstrap() error {
	if restore, err := b.applyParams(); err != nil {
		return err
	} else {
		defer restore()
	}

	sshClient, err := ssh.NewInitClientFromFlagsWithHosts(true)
	if err != nil {
		return nil
	}

	if err = cache.InitWithOptions(sshClient.Check().String(), cache.CacheOptions{InitialState: b.InitialState, ResetInitialState: b.ResetInitialState}); err != nil {
		return fmt.Errorf("Can not init cache: %v", err)
	}

	bootstrapState := NewBootstrapState(cache.Global())

	postScriptExecutor := NewPostBootstrapScriptExecutor(sshClient, app.PostBootstrapScriptPath, bootstrapState).
		WithTimeout(app.PostBootstrapScriptTimeout)

	if err := postScriptExecutor.Execute(); err != nil {
		return err
	}

	out, err := bootstrapState.PostBootstrapScriptResult()
	if err != nil {
		return err
	}

	fmt.Printf("Output from post-bootstrap script:\n%s", string(out))

	return nil
}
