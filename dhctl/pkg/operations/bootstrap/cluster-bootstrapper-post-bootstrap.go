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
	"context"
	"fmt"

	"github.com/deckhouse/lib-connection/pkg/ssh"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/helper"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/progressbar"
)

func (b *ClusterBootstrapper) ExecPostBootstrap(ctx context.Context) error {
	nodeInterface, err := helper.GetNodeInterface(ctx, b.SSHProviderInitializer, b.SSHProviderInitializer.GetSettings())
	if err != nil {
		return err
	}

	wrapper, ok := nodeInterface.(*ssh.NodeInterfaceWrapper)
	if !ok {
		return fmt.Errorf("post bootstrap executor is not supported for local execution contexts")
	}

	if err := wrapper.Client().Start(); err != nil {
		return fmt.Errorf("unable to start ssh client: %w", err)
	}

	if err := cache.InitWithOptions(ctx, wrapper.Client().Check().String(), cache.CacheOptions{InitialState: b.InitialState, ResetInitialState: b.ResetInitialState, Cache: b.Options.Cache}); err != nil {
		return fmt.Errorf("Can not init cache: %v", err)
	}

	bootstrapState := NewBootstrapState(cache.Global())

	interactive := input.IsTerminal()
	if interactive {
		intLogger, ok := b.logger.(*log.InteractiveLogger)
		if !ok {
			return fmt.Errorf("logger is not interactive")
		}
		labelChan := intLogger.GetPhaseChan()
		phasesChan := make(chan phases.Progress, 5)
		pbParam := progressbar.NewPbParams(100, "Executing post-bootstrap script", labelChan, phasesChan)

		if err := progressbar.InitProgressBar(pbParam); err != nil {
			return err
		}

		onComplete := func() {
			pb := progressbar.GetDefaultPb()
			pb.ProgressBarPrinter.Add(100 - pb.ProgressBarPrinter.Current)
			pb.MultiPrinter.Stop()
		}
		defer onComplete()
	}

	postScriptExecutor := NewPostBootstrapScriptExecutor(b.SSHProviderInitializer, b.Options.Bootstrap.PostBootstrapScriptPath, bootstrapState).
		WithTimeout(b.Options.Bootstrap.PostBootstrapScriptTimeout)

	if err := postScriptExecutor.Execute(ctx); err != nil {
		return err
	}

	out, err := bootstrapState.PostBootstrapScriptResult(ctx)
	if err != nil {
		return err
	}

	log.InfoF("Output from post-bootstrap script:\n%s", string(out))

	return nil
}
