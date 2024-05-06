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
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	AppName                  = "dhctl"
	VersionFile              = "/deckhouse/version"
	DeckhouseImageDigestFile = "/deckhouse/image_digest"

	// NodeDeckhouseDirectoryPath deckhouse operating directory path.
	NodeDeckhouseDirectoryPath = "/opt/deckhouse"

	// DeckhouseNodeTmpPath deckhouse directory for temporary files.
	DeckhouseNodeTmpPath = NodeDeckhouseDirectoryPath + "/tmp"
	// DeckhouseNodeBinPath deckhouse directory for binary files.
	DeckhouseNodeBinPath = NodeDeckhouseDirectoryPath + "/bin"
)

var TmpDirName = filepath.Join(os.TempDir(), "dhctl")

var (
	// AppVersion is overridden in CI environment via a linker "-X" flag with a CI commit tag or just "dev" if there is none.
	// "local" is kept for manual builds only
	AppVersion = "local"

	ConfigPaths = make([]string, 0)
	SanityCheck = false
	LoggerType  = "pretty"
	IsDebug     = false

	DoNotWriteDebugLogFile = false
)

func init() {
	if os.Getenv("DHCTL_DEBUG") == "yes" {
		IsDebug = true
	}
	file, err := os.OpenFile(VersionFile, os.O_RDONLY, 0644)
	if err != nil {
		return
	}
	defer file.Close()
	buf := make([]byte, 30)
	n, err := file.Read(buf)
	if n > 0 && (errors.Is(err, io.EOF) || err == nil) {
		AppVersion = strings.TrimSpace(string(buf))
		AppVersion = strings.Replace(AppVersion, "\n", "", -1)
	}
}

func GlobalFlags(cmd *kingpin.Application) {
	cmd.Flag("logger-type", "Format logs output of a dhctl in different ways.").
		Envar(configEnvName("LOGGER_TYPE")).
		Default("pretty").
		EnumVar(&LoggerType, "pretty", "simple", "json")
	cmd.Flag("tmp-dir", "Set temporary directory for debug purposes.").
		Envar(configEnvName("TMP_DIR")).
		Default(TmpDirName).
		StringVar(&TmpDirName)
	cmd.Flag("do-not-write-debug-log-file", `Skip write debug log into file in tmp-dir`).
		Envar(configEnvName("DO_NOT_WRITE_DEBUG_LOG")).
		Default("false").
		BoolVar(&DoNotWriteDebugLogFile)
}

func DefineConfigFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("config", `Path to a file with bootstrap configuration and declared Kubernetes resources in YAML format.
It can be go-template file (for only string keys!). Passed data contains next keys:
  cloudDiscovery - the data discovered by applying Terrfarorm and getting its output. It depends on the cloud provider.
`).
		Required().
		Envar(configEnvName("CONFIG")).
		StringsVar(&ConfigPaths)
}

func DefineSanityFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("yes-i-am-sane-and-i-understand-what-i-am-doing", "You should double check what you are doing here.").
		Default("false").
		BoolVar(&SanityCheck)
}

func DefineSkipDebugFile(cmd *kingpin.CmdClause) {

}

func configEnvName(name string) string {
	return "DHCTL_CLI_" + name
}
