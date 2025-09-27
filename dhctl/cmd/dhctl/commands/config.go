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
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

var (
	deckhouseDir           = "/deckhouse"
	kubeadmTemplateOpenAPI = deckhouseDir + "/candi/control-plane-kubeadm/openapi.yaml"
)

func DefineRenderBashibleBundle(cmd *kingpin.CmdClause) *kingpin.CmdClause {
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
			templateController,
			templateData,
			metaConfig.ProviderName,
			"",
		)
	}

	cmd.Action(func(c *kingpin.ParseContext) error {
		return log.Process("bootstrap", "Prepare Bashible Bundle", runFunc)
	})

	return cmd
}

func DefineRenderMasterBootstrap(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineConfigFlags(cmd)
	app.DefineRenderConfigFlags(cmd)

	runFunc := func() error {
		logger := log.GetDefaultLogger()

		metaConfig, err := config.LoadConfigFromFile(
			context.TODO(),
			app.ConfigPaths,
			infrastructureprovider.MetaConfigPreparatorProvider(
				infrastructureprovider.NewPreparatorProviderParams(logger),
			))
		if err != nil {
			return err
		}

		templateController := template.NewTemplateController(app.RenderBashibleBundleDir)
		log.InfoF("Bundle Dir: %q\n\n", templateController.TmpDir)

		return template.PrepareBootstrap(templateController, "127.0.0.1", metaConfig)
	}

	cmd.Action(func(c *kingpin.ParseContext) error {
		return log.Process("bootstrap", "Prepare Bashible Bundle", runFunc)
	})

	return cmd
}

func DefineRenderKubeadmConfig(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineConfigFlags(cmd)
	app.DefineRenderConfigFlags(cmd)

	runFunc := func() error {
		templateData, err := config.ParseBashibleConfig(app.ConfigPaths, kubeadmTemplateOpenAPI)
		if err != nil {
			return err
		}

		templateController := template.NewTemplateController(app.RenderBashibleBundleDir)
		log.InfoF("Bundle Dir: %q\n\n", templateController.TmpDir)

		return template.PrepareKubeadmConfig(templateController, templateData)
	}

	cmd.Action(func(c *kingpin.ParseContext) error {
		return log.Process("bootstrap", "Prepare Kubeadm Config", runFunc)
	})

	return cmd
}

func DefineCommandParseClusterConfiguration(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineInputOutputRenderFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
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
			metaConfig, err = config.ParseConfigFromData(context.TODO(), string(data), preparatorProvider)
			if err != nil {
				return err
			}
		} else {
			metaConfig, err = config.ParseConfig(context.TODO(), []string{app.ParseInputFile}, preparatorProvider)
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

	return cmd
}

func DefineCommandParseCloudDiscoveryData(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineInputOutputRenderFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
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

		schemaStore := config.NewSchemaStore()
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

	return cmd
}

func InitGlobalVars(pwd string) {
	deckhouseDir = pwd + "/deckhouse"
	kubeadmTemplateOpenAPI = deckhouseDir + "/candi/control-plane-kubeadm/openapi.yaml"
}
