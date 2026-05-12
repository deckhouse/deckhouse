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

// Mirror of the parse-* helpers from dhctl/cmd/dhctl/commands/config.go.
// Drift is enforced by tools/check-dhctl-cmd-drift.sh.

package dhctlcli

import (
	"fmt"
	"io"
	"os"

	"gopkg.in/alecthomas/kingpin.v2"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kpcontext"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func DefineCommandParseClusterConfiguration(cmd *kingpin.CmdClause, opts *options.Options) *kingpin.CmdClause {
	app.DefineInputOutputRenderFlags(cmd, &opts.Render)

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
		if opts.Render.ParseInputFile == "" {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("read configs from stdin: %v", err)
			}

			metaConfig, err = config.ParseConfigFromData(
				ctx,
				string(data),
				preparatorProvider,
				opts.DirConfig(),
				config.ValidateOptionStrictUnmarshal(true),
			)
			if err != nil {
				return err
			}
		} else {
			metaConfig, err = config.ParseConfig(ctx, []string{opts.Render.ParseInputFile}, preparatorProvider, opts.DirConfig())
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

		schemaStore := config.NewSchemaStore(opts.DirConfig())
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
