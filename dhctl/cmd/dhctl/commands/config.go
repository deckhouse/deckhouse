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
	"context"
	"fmt"

	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-dhctl/pkg/logger"
	"github.com/spf13/cobra"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/module/controlplane"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/bootstrap/registry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

// loadConfigForRender loads the config of a render command together with the check the loader no
// longer makes for it: the loader also serves bootstrap on a cluster whose control plane dhctl did
// not create, and that cluster carries no ClusterConfiguration. Rendering without one fails
// silently - ConfigForBashibleBundleTemplate and ClusterConfigMap range over an empty map and
// return no error - so every one of these commands would write out a bundle with no cluster domain
// and no Kubernetes version and report success.
func loadConfigForRender(ctx context.Context, command string, opts *options.Options) (*config.MetaConfig, error) {
	metaConfig, err := config.LoadConfigFromFile(
		ctx,
		opts.Global.ConfigPaths,
		infrastructureprovider.MetaConfigValidatorProvider(),
		&opts.Global,
	)
	if err != nil {
		return nil, err
	}

	if !metaConfig.HasClusterConfiguration() {
		return nil, fmt.Errorf("%s requires ClusterConfiguration: everything it renders is built out of the cluster domain, Kubernetes version and cluster type it carries", command)
	}

	if err := metaConfig.RequireNetwork(); err != nil {
		return nil, err
	}

	return metaConfig, nil
}

func DefineRenderBashibleBundle(cmd *cobra.Command, opts *options.Options) {
	app.DefineConfigFlags(cmd, &opts.Global)
	app.DefineRenderConfigFlags(cmd, &opts.Render)
	app.DefineImgBundleFlags(cmd, &opts.Registry)

	runFunc := func(ctx context.Context) error {
		l := logger.FromContext(ctx)

		// Registry shoud run before LoadConfigFromFile
		registryStop, err := registry.InitFromConfig(
			ctx,
			l,
			opts.Global.ConfigPaths,
			opts.Registry.ImgBundlePath,
		)
		if err != nil {
			return err
		}
		defer registryStop()

		metaConfig, err := loadConfigForRender(ctx, "dhctl config render bashible-bundle", opts)
		if err != nil {
			return err
		}

		return renderBashibleBundle(ctx, metaConfig, opts)
	}

	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		l := logger.FromContext(ctx)

		return logger.RunProcess(ctx, l, "Prepare Bashible Bundle", runFunc)
	}
}

// renderBashibleBundle writes the first master's bundle, the one bootstrap runs: its node
// agent starts from the seed dhctl builds, so the scripts are rendered from the same data.
func renderBashibleBundle(ctx context.Context, metaConfig *config.MetaConfig, opts *options.Options) error {
	templateData, err := template.FirstMasterBashibleData(ctx, metaConfig, "$MY_IP", opts.Global.CandiDir)
	if err != nil {
		return fmt.Errorf("collect the first master's bundle data: %w", err)
	}

	templateController := template.NewTemplateController(opts.Render.BashibleBundleDir)
	logger.FromContext(ctx).Info(fmt.Sprintf("Bundle Dir: %q", templateController.TmpDir))

	return template.PrepareBashibleBundle(
		ctx,
		templateController,
		templateData,
		metaConfig.ProviderName,
		"",
		&opts.Global,
	)
}

func DefineRenderMasterBootstrap(cmd *cobra.Command, opts *options.Options) {
	app.DefineConfigFlags(cmd, &opts.Global)
	app.DefineRenderConfigFlags(cmd, &opts.Render)
	app.DefineImgBundleFlags(cmd, &opts.Registry)

	runFunc := func(ctx context.Context) error {
		l := logger.FromContext(ctx)

		// Registry shoud run before LoadConfigFromFile
		registryStop, err := registry.InitFromConfig(
			ctx,
			l,
			opts.Global.ConfigPaths,
			opts.Registry.ImgBundlePath,
		)
		if err != nil {
			return err
		}
		defer registryStop()

		metaConfig, err := loadConfigForRender(ctx, "dhctl config render master-bootstrap-scripts", opts)
		if err != nil {
			return err
		}

		templateController := template.NewTemplateController(opts.Render.BashibleBundleDir)
		l.Info(fmt.Sprintf("Bundle Dir: %q", templateController.TmpDir))

		return template.PrepareBootstrap(ctx, templateController, "127.0.0.1", metaConfig, &opts.Global)
	}

	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		l := logger.FromContext(ctx)

		return logger.RunProcess(ctx, l, "Prepare Bashible Bundle", runFunc)
	}
}

func DefineRenderControlPlaneAndPKI(cmd *cobra.Command, opts *options.Options) {
	app.DefineConfigFlags(cmd, &opts.Global)
	app.DefineRenderConfigFlags(cmd, &opts.Render)
	app.DefineImgBundleFlags(cmd, &opts.Registry)

	runFunc := func(ctx context.Context) error {
		l := logger.FromContext(ctx)

		// Registry shoud run before LoadConfigFromFile
		registryStop, err := registry.InitFromConfig(
			ctx,
			l,
			opts.Global.ConfigPaths,
			opts.Registry.ImgBundlePath,
		)
		if err != nil {
			return err
		}
		defer registryStop()

		metaConfig, err := loadConfigForRender(ctx, "dhctl config render control-plane-manifests", opts)
		if err != nil {
			return err
		}

		extractor := controlplane.NewSettingsExtractor(
			metaConfig,
			config.NewSchemaStore(&opts.Global),
			config.GetEdition(),
			l,
		)

		controlPlaneConfig, err := extractor.TemplateConfigForBootstrap("")
		if err != nil {
			return err
		}

		templateController := template.NewTemplateController(opts.Render.BashibleBundleDir)
		l.Info(fmt.Sprintf("Bundle Dir: %q", templateController.TmpDir))

		if err := template.PrepareControlPlaneManifests(ctx, templateController, controlPlaneConfig, &opts.Global); err != nil {
			return err
		}
		// "localhost"/"127.0.0.1" are placeholders for the render-only command;
		// the resulting PKI is not used to start a real cluster.
		return template.PreparePKI(templateController, "localhost", "127.0.0.1", "127.0.0.1", controlPlaneConfig)
	}

	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		l := logger.FromContext(ctx)

		return logger.RunProcess(ctx, l, "Prepare Kubeadm Config", runFunc)
	}
}
