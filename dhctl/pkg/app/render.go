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

import "gopkg.in/alecthomas/kingpin.v2"

var (
	RenderBashibleBundleDir = ""
	BundleName              = ""

	ParseInputFile = ""
	ParseOutput    = "json"

	Editor = ""
)

func DefineRenderConfigFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("bundle-dir", "Directory to render bashible bundle.").
		Envar(configEnvName("BUNDLE_DIR")).
		StringVar(&RenderBashibleBundleDir)
}

func DefineRenderBundleFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("bundle-name", "Bundle name for render bashible bundle.").
		Envar(configEnvName("BUNDLE_NAME")).
		Required().
		StringVar(&BundleName)
}

func DefineEditorConfigFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("editor", "Your favourite editor.").
		Envar(configEnvName("EDITOR")).
		StringVar(&Editor)
}

func DefineInputOutputRenderFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("file", "Input file name with YAML-documents.").
		Short('f').
		Envar(configEnvName("FILE")).
		StringVar(&ParseInputFile)

	cmd.Flag("output", "Output format (JSON or YAML).").
		Short('o').
		Envar(configEnvName("OUTPUT")).
		EnumVar(&ParseOutput, "yaml", "json")
}
