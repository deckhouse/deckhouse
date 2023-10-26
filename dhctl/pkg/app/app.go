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
	AppVersion = "local"

	ConfigPath  = ""
	SanityCheck = false
	LoggerType  = "pretty"
	IsDebug     = false
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
}

func DefineConfigFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("config", "Config file path").
		Required().
		Envar(configEnvName("CONFIG")).
		StringVar(&ConfigPath)
}

func DefineSanityFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("yes-i-am-sane-and-i-understand-what-i-am-doing", "You should double check what you are doing here.").
		Default("false").
		BoolVar(&SanityCheck)
}

func configEnvName(name string) string {
	return "DHCTL_CLI_" + name
}
