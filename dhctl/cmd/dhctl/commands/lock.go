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

package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	v1 "k8s.io/api/coordination/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/lease"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/lock"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
	"github.com/deckhouse/deckhouse/dhctl/pkg/telemetry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

const autoConvergerErrorFmt = `Error: converge locked by auto-converger.
If you are confident in your actions, you can use the flag "--yes-i-am-sane-and-i-understand-what-i-am-doing"

Lock info:

%s
`

func DefineReleaseConvergeLockCommand(cmd *cobra.Command, opts *options.Options) {
	app.DefineSanityFlags(cmd, &opts.Global)
	app.DefineSSHFlags(cmd, opts)
	app.DefineBecomeFlags(cmd, &opts.Become)
	app.DefineKubeFlags(cmd, &opts.Kube)

	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()

		span := telemetry.SpanFromContext(ctx)
		span.SetAttributes(opts.ToSpanAttributes()...)

		sshProviderInitializer, kubeProvider, err := providerinitializer.CLIProviders(ctx, opts, providerinitializer.CLIProvidersOptions{RequireKube: true})
		if err != nil {
			return err
		}

		defer providerinitializer.CleanupSSHProvider(ctx, sshProviderInitializer)

		if kubeProvider == nil {
			return fmt.Errorf("kubernetes provider is not initialized")
		}

		kube, err := kubeProvider.Client(ctx)
		if err != nil {
			return err
		}
		kubeCl := &client.KubernetesClient{KubeClient: kube}

		confirm := func(l *v1.Lease) error {
			if opts.Global.SanityCheck {
				return nil
			}

			info, _ := lease.LockInfo(l)

			if *l.Spec.HolderIdentity == lock.AutoConvergerIdentity {
				return fmt.Errorf(autoConvergerErrorFmt, info)
			}

			c := input.NewConfirmation()

			approve := c.WithMessage(fmt.Sprintf("Do you want to release the lock:\n\n%s", info)).Ask()
			if !approve {
				return fmt.Errorf("Lock release was not confirmed")
			}

			return nil
		}

		cnf := lock.GetLockLeaseConfig(ctx, "lock-releaser", opts.SSH.User)
		return lease.RemoveLease(ctx, kubeCl, cnf, confirm)
	}
}
