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

package app

import (
	"github.com/spf13/cobra"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

// DefineRenderConfigFlags registers --bundle-dir.
func DefineRenderConfigFlags(cmd *cobra.Command, o *options.RenderOptions) {
	StringFlag(cmd.Flags(), Flag{
		Name: "bundle-dir",
		Env:  "DHCTL_CLI_BUNDLE_DIR",
		Help: "Directory to render the bashible bundle into.",
	}, &o.BashibleBundleDir)
}

// DefineEditorConfigFlags registers --editor.
func DefineEditorConfigFlags(cmd *cobra.Command, o *options.RenderOptions) {
	StringFlag(cmd.Flags(), Flag{Name: "editor", Env: "DHCTL_CLI_EDITOR", Help: "Your favourite editor."}, &o.Editor)
}

// DefineInputOutputRenderFlags registers --file and --output.
func DefineInputOutputRenderFlags(cmd *cobra.Command, o *options.RenderOptions) {
	flags := cmd.Flags()
	StringFlag(flags, Flag{
		Name:  "file",
		Short: "f",
		Env:   "DHCTL_CLI_FILE",
		Help:  "Input file name with YAML-documents.",
	}, &o.ParseInputFile)

	EnumFlag(flags, Flag{
		Name:  "output",
		Short: "o",
		Env:   "DHCTL_CLI_OUTPUT",
		Help:  "Output format (JSON or YAML).",
	}, &o.ParseOutput, "yaml", "json")
}
