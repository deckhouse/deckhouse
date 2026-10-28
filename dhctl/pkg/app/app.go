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

const (
	AppName = "dhctl"

	// Node directory layout used inside the deckhouse container.
	NodeDeckhouseDirectoryPath = "/opt/deckhouse"
	DeckhouseNodeTmpPath       = NodeDeckhouseDirectoryPath + "/tmp"
	DeckhouseNodeBinPath       = NodeDeckhouseDirectoryPath + "/bin"
)

func GlobalFlags(root *cobra.Command, o *options.GlobalOptions) {
	flags := root.PersistentFlags()
	EnumFlag(flags, Flag{
		Name: "logger-type",
		Env:  "DHCTL_CLI_LOGGER_TYPE",
		Help: "Format the log output of dhctl in different ways.",
	}, &o.LoggerType, "pretty", "json")
	StringFlag(flags, Flag{
		Name: "tmp-dir",
		Env:  "DHCTL_CLI_TMP_DIR",
		Help: "Set the temporary directory for debug purposes.",
	}, &o.TmpDir)
	BoolFlag(flags, Flag{
		Name: "do-not-write-debug-log-file",
		Env:  "DHCTL_CLI_DO_NOT_WRITE_DEBUG_LOG",
		Help: `Skip writing the debug log to a file in tmp-dir`,
	}, &o.DoNotWriteDebugLogFile)
	StringFlag(flags, Flag{
		Name: "debug-log-file-path",
		Env:  "DHCTL_CLI_DEBUG_LOG_FILE_PATH",
		Help: `Write debug log into passed file instead of standard file path ${DHCTL_TMP_DIR}/state-dir/operation-data.log`,
	}, &o.DebugLogFilePath)
	StringFlag(flags, Flag{
		Name: "progress-log-file-path",
		Env:  "DHCTL_CLI_PROGRESS_LOG_FILE_PATH",
		Help: `If specified, DHCTL will write operation progress in jsonl format`,
	}, &o.ProgressFilePath)
	StringFlag(flags, Flag{
		Name: "download-dir",
		Env:  "DHCTL_CLI_DOWNLOAD_DIR",
		Help: "Set the directory for downloaded images and their content",
	}, &o.DownloadDir)
	StringFlag(flags, Flag{
		Name:   "download-cache-dir",
		Env:    "DHCTL_CLI_DOWNLOAD_CACHE_DIR",
		Help:   "Deprecated and ignored: images are kept unpacked by digest under --download-dir.",
		Hidden: true,
	}, &o.DownloadCacheDir)
	BoolFlag(flags, Flag{Name: "verbose", Short: "v", Help: "Show verbose command output"}, &o.ShowProgress)
}

// DefineConfigFlags registers --config (required).
func DefineConfigFlags(cmd *cobra.Command, o *options.GlobalOptions) {
	StringsFlag(cmd.Flags(), Flag{Name: "config", Env: "DHCTL_CLI_CONFIG", Help: `Path to a file with bootstrap configuration and declared Kubernetes resources in YAML format.
It can be a go-template file (string keys only!). The passed data contains the following keys:
  cloudDiscovery - the data discovered by running the infrastructure creation utility and getting its output. It depends on the cloud provider.
`, Required: true}, &o.ConfigPaths)
}

// DefineSanityFlags registers the destructive-action confirmation flag.
func DefineSanityFlags(cmd *cobra.Command, o *options.GlobalOptions) {
	BoolFlag(cmd.Flags(), Flag{
		Name: "yes-i-am-sane-and-i-understand-what-i-am-doing",
		Help: "You should double-check what you are doing here.",
	}, &o.SanityCheck)
}
