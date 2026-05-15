// Copyright 2021 Flant JSC
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

	"github.com/deckhouse/deckhouse/dhctl/pkg/telemetry"
	"gopkg.in/alecthomas/kingpin.v2"
	v1 "k8s.io/api/coordination/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kpcontext"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/lease"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/lock"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

const autoConvergerErrorFmt = `Error: converge locked by auto-converger.
If you are confident in your actions, you can use the flag "--yes-i-am-sane-and-i-understand-what-i-am-doing"

Lock info:

%s
`

func DefineReleaseConvergeLockCommand(cmd *kingpin.CmdClause, opts *options.Options) *kingpin.CmdClause {
	app.DefineSanityFlags(cmd, &opts.Global)
	app.DefineSSHFlags(cmd, &opts.SSH, nil)
	app.DefineBecomeFlags(cmd, &opts.Become)
	app.DefineKubeFlags(cmd, &opts.Kube)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)

		span := telemetry.SpanFromContext(ctx)
		span.SetAttributes(opts.ToSpanAttributes()...)

		params, err := app.DefaultProviderParams(&opts.Global)
		if err != nil {
			return err
		}

		sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(
			ctx,
			params,
			providerinitializer.WithKubeFlagsDefined(opts.Kube.IsDefined()),
			providerinitializer.WithRequiredKubeProvider(),
		)
		if err != nil {
			return err
		}

		if kubeProvider == nil {
			return fmt.Errorf("kubernetes provider is not initialized")
		}
		if sshProviderInitializer != nil {
			defer sshProviderInitializer.Cleanup(ctx)
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

			approve := c.WithMessage(fmt.Sprintf("Do you want to release lock:\n\n%s", info)).Ask()
			if !approve {
				return fmt.Errorf("Don't confirm release lock")
			}

			return nil
		}

		cnf := lock.GetLockLeaseConfig("lock-releaser", opts.SSH.User)
		return lease.RemoveLease(ctx, kubeCl, cnf, confirm)
	})
}
