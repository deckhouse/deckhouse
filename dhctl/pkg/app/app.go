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
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

const (
	AppName = "dhctl"

	// Node directory layout used inside the deckhouse container.
	NodeDeckhouseDirectoryPath = "/opt/deckhouse"
	DeckhouseNodeTmpPath       = NodeDeckhouseDirectoryPath + "/tmp"
	DeckhouseNodeBinPath       = NodeDeckhouseDirectoryPath + "/bin"
)

// GlobalFlags registers application-wide flags into cmd, writing into o.
func GlobalFlags(cmd *kingpin.Application, o *options.GlobalOptions) {
	cmd.Flag("logger-type", "Format logs output of a dhctl in different ways.").
		Envar(configEnvName("LOGGER_TYPE")).
		Default("pretty").
		EnumVar(&o.LoggerType, "pretty", "json")
	cmd.Flag("tmp-dir", "Set temporary directory for debug purposes.").
		Envar(configEnvName("TMP_DIR")).
		Default(o.TmpDir).
		StringVar(&o.TmpDir)
	cmd.Flag("do-not-write-debug-log-file", `Skip write debug log into file in tmp-dir`).
		Envar(configEnvName("DO_NOT_WRITE_DEBUG_LOG")).
		Default("false").
		BoolVar(&o.DoNotWriteDebugLogFile)
	cmd.Flag("debug-log-file-path", `Write debug log into passed file instead of standard file path ${DHCTL_TMP_DIR}/state-dir/operation-data.log`).
		Envar(configEnvName("DEBUG_LOG_FILE_PATH")).
		Default("").
		StringVar(&o.DebugLogFilePath)
	cmd.Flag("progress-log-file-path", `If specified, DHCTL will write operation progress in jsonl format`).
		Envar(configEnvName("PROGRESS_LOG_FILE_PATH")).
		Default("").
		StringVar(&o.ProgressFilePath)
	cmd.Flag("download-dir", "Set directory for downloaded images and it's content").
		Envar(configEnvName("DOWNLOAD_DIR")).
		Default(o.DownloadDir).
		StringVar(&o.DownloadDir)
	cmd.Flag("download-cache-dir", "Set directory for downloaded images layers cache.").
		Envar(configEnvName("DOWNLOAD_CACHE_DIR")).
		Default(o.DownloadCacheDir).
		StringVar(&o.DownloadCacheDir)
	cmd.Flag("verbose", "Show verbose command output").
		Default("false").
		Short('v').
		BoolVar(&o.ShowProgress)
}

// DefineConfigFlags registers --config (required).
func DefineConfigFlags(cmd *kingpin.CmdClause, o *options.GlobalOptions) {
	cmd.Flag("config", `Path to a file with bootstrap configuration and declared Kubernetes resources in YAML format.
It can be go-template file (for only string keys!). Passed data contains next keys:
  cloudDiscovery - the data discovered by applying infrastructure creation utility and getting its output. It depends on the cloud provider.
`).
		Required().
		Envar(configEnvName("CONFIG")).
		StringsVar(&o.ConfigPaths)
}

// DefineSanityFlags registers the destructive-action confirmation flag.
func DefineSanityFlags(cmd *kingpin.CmdClause, o *options.GlobalOptions) {
	cmd.Flag("yes-i-am-sane-and-i-understand-what-i-am-doing", "You should double check what you are doing here.").
		Default("false").
		BoolVar(&o.SanityCheck)
}

func configEnvName(name string) string {
	return "DHCTL_CLI_" + name
}
