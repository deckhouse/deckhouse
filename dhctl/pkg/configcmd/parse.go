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

package configcmd

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
)

func DefineCommandParseClusterConfiguration(cmd *cobra.Command, opts *options.Options) {
	app.DefineInputOutputRenderFlags(cmd, &opts.Render)

	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()

		var (
			err        error
			metaConfig *config.MetaConfig
		)

		validatorProvider := infrastructureprovider.MetaConfigValidatorProvider()

		// Without --file the documents come from stdin.
		if readsStdin(&opts.Render) {
			data, err := io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return fmt.Errorf("read configs from stdin: %w", err)
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
			if output, err = yaml.JSONToYAML(output); err != nil {
				return fmt.Errorf("convert cluster configuration to YAML: %w", err)
			}
		case "json":
		default:
			return fmt.Errorf("unknown output type: %s", opts.Render.ParseOutput)
		}

		if _, err := fmt.Fprint(cmd.OutOrStdout(), string(output)); err != nil {
			return fmt.Errorf("print cluster configuration: %w", err)
		}

		return nil
	}
}

// readsStdin is no --file or a lone dash: kingpin read "-" as an empty value.
func readsStdin(o *options.RenderOptions) bool {
	return o.ParseInputFile == "" || o.ParseInputFile == "-"
}

func DefineCommandParseCloudDiscoveryData(cmd *cobra.Command, opts *options.Options) {
	app.DefineInputOutputRenderFlags(cmd, &opts.Render)

	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()

		var (
			err  error
			data []byte
		)

		if readsStdin(&opts.Render) {
			data, err = io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return fmt.Errorf("read cloud-discovery-data from stdin: %w", err)
			}
		} else {
			data, err = os.ReadFile(opts.Render.ParseInputFile)
			if err != nil {
				return fmt.Errorf("loading input file: %w", err)
			}
		}

		schemaStore := config.NewSchemaStore(&opts.Global)

		_, err = schemaStore.Validate(ctx, &data)
		if err != nil {
			return fmt.Errorf("validate cloud_discovery_data: %w", err)
		}

		var output []byte

		switch opts.Render.ParseOutput {
		case "yaml":
			if output, err = yaml.JSONToYAML(data); err != nil {
				return fmt.Errorf("convert cloud discovery data to YAML: %w", err)
			}
		case "json":
			output = data
		default:
			return fmt.Errorf("unknown output type: %s", opts.Render.ParseOutput)
		}

		if _, err := fmt.Fprint(cmd.OutOrStdout(), string(output)); err != nil {
			return fmt.Errorf("print cloud discovery data: %w", err)
		}

		return nil
	}
}
