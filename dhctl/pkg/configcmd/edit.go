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

// Package configcmd holds the `config edit` and `config parse` commands. dhctl and
// deckhouse-controller both register them, so neither keeps a copy.
package configcmd

import (
	"context"
	"fmt"
	"maps"
	"slices"

	libcon "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg"
	"github.com/spf13/cobra"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
	"github.com/deckhouse/deckhouse/dhctl/pkg/telemetry"
)

// editSecrets are the Secret and its data key each config edit command changes, by command name.
var editSecrets = map[string]struct{ secret, dataKey string }{
	"cluster-configuration":          {"d8-cluster-configuration", "cluster-configuration.yaml"},
	"provider-cluster-configuration": {"d8-provider-cluster-configuration", "cloud-provider-cluster-configuration.yaml"},
	"static-cluster-configuration":   {"d8-static-cluster-configuration", "static-cluster-configuration.yaml"},
}

// DefineEditCommand defines the dhctl config edit command that cmd names, one of editSecrets.
func DefineEditCommand(cmd *cobra.Command, opts *options.Options) {
	defineEditCommand(cmd, opts, true)
}

// DefineKubeEditCommands adds the edit commands under parent without SSH, for deckhouse-controller:
// it runs in the cluster and reaches Kubernetes with the kube flags only.
func DefineKubeEditCommands(parent *cobra.Command, opts *options.Options) {
	for _, name := range slices.Sorted(maps.Keys(editSecrets)) {
		cmd := &cobra.Command{Use: name, Short: fmt.Sprintf("Edit %s in the Kubernetes cluster.", name), Args: cobra.NoArgs}
		parent.AddCommand(cmd)
		defineEditCommand(cmd, opts, false)
	}
}

func defineEditCommand(cmd *cobra.Command, opts *options.Options, withSSH bool) {
	name := cmd.Name()

	target, ok := editSecrets[name]
	if !ok {
		panic(fmt.Sprintf("config edit %s names no Secret", name))
	}

	app.DefineEditorConfigFlags(cmd, &opts.Render)
	app.DefineSanityFlags(cmd, &opts.Global)
	app.DefineKubeFlags(cmd, &opts.Kube)

	if withSSH {
		app.DefineSSHFlags(cmd, opts)
		app.DefineBecomeFlags(cmd, &opts.Become)
	}

	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()

		span := telemetry.SpanFromContext(ctx)
		span.SetAttributes(opts.ToSpanAttributes()...)

		sshProviderInitializer, kubeProvider, err := editProviders(ctx, opts, withSSH)
		if err != nil {
			return err
		}

		defer providerinitializer.CleanupSSHProvider(ctx, sshProviderInitializer)

		if kubeProvider == nil {
			return fmt.Errorf("kubernetes provider is not initialized")
		}

		kubeIface, err := kubeProvider.Client(ctx)
		if err != nil {
			return err
		}

		return editSecret(ctx, &client.KubernetesClient{KubeClient: kubeIface}, opts, name, target.secret, target.dataKey)
	}
}

func editSecret(ctx context.Context, kubeCl *client.KubernetesClient, opts *options.Options, name, secret, dataKey string) error {
	return operations.SecretEdit(
		ctx,
		kubeCl,
		name, "kube-system", secret, dataKey, operations.SecretLabelsFor(secret),
		&opts.Global,
		operations.EditOptions{
			Editor:      opts.Render.Editor,
			TmpDir:      opts.Global.TmpDir,
			SanityCheck: opts.Global.SanityCheck,
			OnAbsent:    operations.OnAbsentFor(secret),
		},
	)
}

// editProviders reaches Kubernetes as CLIProviders does, or with the kube flags only when the
// command has no SSH.
func editProviders(ctx context.Context, opts *options.Options, withSSH bool) (*providerinitializer.SSHProviderInitializer, libcon.KubeProvider, error) {
	if withSSH {
		return providerinitializer.CLIProviders(ctx, opts, providerinitializer.CLIProvidersOptions{RequireKube: true})
	}

	kubeProvider, err := providerinitializer.CLIKubeProviders(ctx, opts)

	return nil, kubeProvider, err
}
