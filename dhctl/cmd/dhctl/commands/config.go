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
	"io"
	"os"

	"gopkg.in/alecthomas/kingpin.v2"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kpcontext"
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

	return metaConfig, nil
}

func DefineRenderBashibleBundle(cmd *kingpin.CmdClause, opts *options.Options) *kingpin.CmdClause {
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

		templateData, err := metaConfig.ConfigForBashibleBundleTemplate(ctx, "$MY_IP")
		if err != nil {
			return err
		}

		templateController := template.NewTemplateController(opts.Render.BashibleBundleDir)
		l.Info(fmt.Sprintf("Bundle Dir: %q", templateController.TmpDir))

		return template.PrepareBashibleBundle(
			ctx,
			templateController,
			templateData,
			metaConfig.ProviderName,
			"",
			&opts.Global,
		)
	}

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)
		l := logger.FromContext(ctx)

		return logger.RunProcess(ctx, l, "Prepare Bashible Bundle", runFunc)
	})
}

func DefineRenderMasterBootstrap(cmd *kingpin.CmdClause, opts *options.Options) *kingpin.CmdClause {
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

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)
		l := logger.FromContext(ctx)

		return logger.RunProcess(ctx, l, "Prepare Bashible Bundle", runFunc)
	})
}

func DefineRenderControlPlaneAndPKI(cmd *kingpin.CmdClause, opts *options.Options) *kingpin.CmdClause {
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

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)
		l := logger.FromContext(ctx)

		return logger.RunProcess(ctx, l, "Prepare Kubeadm Config", runFunc)
	})
}

func DefineCommandParseClusterConfiguration(cmd *kingpin.CmdClause, opts *options.Options) *kingpin.CmdClause {
	app.DefineInputOutputRenderFlags(cmd, &opts.Render)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)

		var err error
		var metaConfig *config.MetaConfig

		validatorProvider := infrastructureprovider.MetaConfigValidatorProvider()

		// Should be fixed in kingpin repo or shell-operator and others should migrate to github.com/alecthomas/kingpin.
		// https://github.com/flant/kingpin/pull/1
		// replace gopkg.in/alecthomas/kingpin.v2 => github.com/flant/kingpin is not working
		if opts.Render.ParseInputFile == "" {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("read configs from stdin: %v", err)
			}

			metaConfig, err = config.ParseConfigFromData(
				ctx,
				string(data),
				validatorProvider,
				&opts.Global,
				config.ValidateOptionStrictUnmarshal(true),
			)
			if err != nil {
				return err
			}
		} else {
			metaConfig, err = config.ParseConfig(ctx, []string{opts.Render.ParseInputFile}, validatorProvider, &opts.Global)
			if err != nil {
				return err
			}
		}

		output := metaConfig.MarshalFullConfig()
		switch opts.Render.ParseOutput {
		case "yaml":
			output, _ = yaml.JSONToYAML(output)
		case "json":
		default:
			return fmt.Errorf("unknown output type: %s", opts.Render.ParseOutput)
		}

		fmt.Print(string(output))
		return nil
	})
}

func DefineCommandParseCloudDiscoveryData(cmd *kingpin.CmdClause, opts *options.Options) *kingpin.CmdClause {
	app.DefineInputOutputRenderFlags(cmd, &opts.Render)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		_ = kpcontext.ExtractContext(c)

		var err error
		var data []byte

		if opts.Render.ParseInputFile == "" {
			data, err = io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("read cloud-discovery-data from stdin: %v", err)
			}
		} else {
			data, err = os.ReadFile(opts.Render.ParseInputFile)
			if err != nil {
				return fmt.Errorf("loading input file: %v", err)
			}
		}

		schemaStore := config.NewSchemaStore(&opts.Global)
		_, err = schemaStore.Validate(&data)
		if err != nil {
			return fmt.Errorf("validate cloud_discovery_data: %v", err)
		}

		var output []byte
		switch opts.Render.ParseOutput {
		case "yaml":
			output, _ = yaml.JSONToYAML(data)
		case "json":
			output = data
		default:
			return fmt.Errorf("unknown output type: %s", opts.Render.ParseOutput)
		}

		fmt.Print(string(output))
		return nil
	})
}
