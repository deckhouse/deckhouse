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
	"context"
	"fmt"
	"io"
	"os"

	"gopkg.in/alecthomas/kingpin.v2"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kpcontext"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

var (
	deckhouseDir = "/deckhouse"
)

func DefineRenderBashibleBundle(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineConfigFlags(cmd)
	app.DefineRenderConfigFlags(cmd)

	runFunc := func(ctx context.Context) error {
		logger := log.GetDefaultLogger()

		metaConfig, err := config.LoadConfigFromFile(
			ctx,
			app.ConfigPaths,
			infrastructureprovider.MetaConfigPreparatorProvider(
				infrastructureprovider.NewPreparatorProviderParams(logger),
			),
			app.GetDirConfig(),
		)
		if err != nil {
			return err
		}

		templateData, err := metaConfig.ConfigForBashibleBundleTemplate("$MY_IP")
		if err != nil {
			return err
		}

		templateController := template.NewTemplateController(app.RenderBashibleBundleDir)
		log.InfoF("Bundle Dir: %q\n\n", templateController.TmpDir)

		return template.PrepareBashibleBundle(
			ctx,
			templateController,
			templateData,
			metaConfig.ProviderName,
			"",
			app.GetDirConfig(),
		)
	}

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)

		return log.ProcessCtx(ctx, "bootstrap", "Prepare Bashible Bundle", runFunc)
	})
}

func DefineRenderMasterBootstrap(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineConfigFlags(cmd)
	app.DefineRenderConfigFlags(cmd)

	runFunc := func(ctx context.Context) error {
		logger := log.GetDefaultLogger()

		metaConfig, err := config.LoadConfigFromFile(
			ctx,
			app.ConfigPaths,
			infrastructureprovider.MetaConfigPreparatorProvider(
				infrastructureprovider.NewPreparatorProviderParams(logger),
			),
			app.GetDirConfig(),
		)
		if err != nil {
			return err
		}

		templateController := template.NewTemplateController(app.RenderBashibleBundleDir)
		log.InfoF("Bundle Dir: %q\n\n", templateController.TmpDir)
		return template.PrepareBootstrap(ctx, templateController, "127.0.0.1", metaConfig, app.GetDirConfig())
	}

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)

		return log.ProcessCtx(ctx, "bootstrap", "Prepare Bashible Bundle", runFunc)
	})
}

func DefineRenderControlPlaneAndPKI(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineConfigFlags(cmd)
	app.DefineRenderConfigFlags(cmd)

	runFunc := func() error {
		logger := log.GetDefaultLogger()

		metaConfig, err := config.LoadConfigFromFile(
			context.TODO(),
			app.ConfigPaths,
			infrastructureprovider.MetaConfigPreparatorProvider(
				infrastructureprovider.NewPreparatorProviderParams(logger),
			),
			app.GetDirConfig(),
		)
		if err != nil {
			return err
		}

		templateData, err := metaConfig.ConfigForControlPlaneTemplates("")
		if err != nil {
			return err
		}

		templateController := template.NewTemplateController(app.RenderBashibleBundleDir)
		log.InfoF("Bundle Dir: %q\n\n", templateController.TmpDir)
		if err := template.PrepareControlPlaneManifests(templateController, templateData, app.GetDirConfig()); err != nil {
			return err
		}
		// "localhost"/"127.0.0.1" are placeholders for the render-only command;
		// the resulting PKI is not used to start a real cluster.
		return template.PreparePKI(templateController, "localhost", "127.0.0.1", "127.0.0.1", templateData)
	}

	cmd.Action(func(c *kingpin.ParseContext) error {
		return log.Process("bootstrap", "Prepare ControlPlaneManifest and PKI", runFunc)
	})

		return log.ProcessCtx(ctx, "bootstrap", "Prepare Kubeadm Config", runFunc)
	})
}

func DefineCommandParseClusterConfiguration(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineInputOutputRenderFlags(cmd)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)

		var err error
		var metaConfig *config.MetaConfig

		logger := log.GetDefaultLogger()

		preparatorProvider := infrastructureprovider.MetaConfigPreparatorProvider(
			infrastructureprovider.NewPreparatorProviderParams(logger),
		)

		// Should be fixed in kingpin repo or shell-operator and others should migrate to github.com/alecthomas/kingpin.
		// https://github.com/flant/kingpin/pull/1
		// replace gopkg.in/alecthomas/kingpin.v2 => github.com/flant/kingpin is not working
		if app.ParseInputFile == "" {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("read configs from stdin: %v", err)
			}

			metaConfig, err = config.ParseConfigFromData(
				ctx,
				string(data),
				preparatorProvider,
				app.GetDirConfig(),
				config.ValidateOptionStrictUnmarshal(true),
			)
			if err != nil {
				return err
			}
		} else {
			metaConfig, err = config.ParseConfig(ctx, []string{app.ParseInputFile}, preparatorProvider, app.GetDirConfig())
			if err != nil {
				return err
			}
		}

		output := metaConfig.MarshalFullConfig()
		switch app.ParseOutput {
		case "yaml":
			output, _ = yaml.JSONToYAML(output)
		case "json":
		default:
			return fmt.Errorf("unknown output type: %s", app.ParseOutput)
		}

		fmt.Print(string(output))
		return nil
	})
}

func DefineCommandParseCloudDiscoveryData(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineInputOutputRenderFlags(cmd)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		_ = kpcontext.ExtractContext(c)

		var err error
		var data []byte

		if app.ParseInputFile == "" {
			data, err = io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("read cloud-discovery-data from stdin: %v", err)
			}
		} else {
			data, err = os.ReadFile(app.ParseInputFile)
			if err != nil {
				return fmt.Errorf("loading input file: %v", err)
			}
		}

		schemaStore := config.NewSchemaStore(app.GetDirConfig())
		_, err = schemaStore.Validate(&data)
		if err != nil {
			return fmt.Errorf("validate cloud_discovery_data: %v", err)
		}

		var output []byte
		switch app.ParseOutput {
		case "yaml":
			output, _ = yaml.JSONToYAML(data)
		case "json":
			output = data
		default:
			return fmt.Errorf("unknown output type: %s", app.ParseOutput)
		}

		fmt.Print(string(output))
		return nil
	})
}

func InitGlobalVars(pwd string) {
	deckhouseDir = pwd + "/deckhouse"
}
