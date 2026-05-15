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

package options

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	otattribute "go.opentelemetry.io/otel/attribute"
)

// GlobalOptions holds settings shared by every dhctl command.
type GlobalOptions struct {
	TmpDir                 string
	LoggerType             string
	IsDebug                bool
	DoNotWriteDebugLogFile bool
	DebugLogFilePath       string
	ProgressFilePath       string
	DownloadDir            string
	DownloadCacheDir       string
	ConfigPaths            []string
	SanityCheck            bool
	ShowProgress           bool
}

func (o GlobalOptions) ToSpanAttributes() []otattribute.KeyValue {
	return []otattribute.KeyValue{
		otattribute.String("global.tmpDir", o.TmpDir),
		otattribute.String("global.loggerType", o.LoggerType),
		otattribute.Bool("global.isDebug", o.IsDebug),
		otattribute.Bool("global.doNotWriteDebugLogFile", o.DoNotWriteDebugLogFile),
		otattribute.String("global.debugLogFilePath", o.DebugLogFilePath),
		otattribute.String("global.progressFilePath", o.ProgressFilePath),
		otattribute.String("global.downloadDir", o.DownloadDir),
		otattribute.String("global.downloadCacheDir", o.DownloadCacheDir),
		otattribute.StringSlice("global.configPaths", o.ConfigPaths),
	}
}

// NewGlobalOptions returns GlobalOptions with defaults applied.
//
// The DHCTL_DEBUG environment variable is honored here so commands receive
// the same IsDebug behavior the previous package init() used to set.
func NewGlobalOptions() GlobalOptions {
	return GlobalOptions{
		TmpDir:           DefaultTmpDir(),
		LoggerType:       "pretty",
		IsDebug:          os.Getenv("DHCTL_DEBUG") == "yes",
		DownloadDir:      DefaultTmpDir(),
		DownloadCacheDir: filepath.Join(DefaultTmpDir(), "cache"),
		ConfigPaths:      make([]string, 0),
	}
}

// BuildInfo carries version/edition metadata loaded once at startup.
type BuildInfo struct {
	AppVersion string
	AppEdition string

	DeckhouseDir string
	VersionFile  string
	EditionFile  string
}

func (o BuildInfo) ToSpanAttributes() []otattribute.KeyValue {
	return []otattribute.KeyValue{
		otattribute.String("buildinfo.appVersion", o.AppVersion),
		otattribute.String("buildinfo.appEdition", o.AppEdition),
		otattribute.String("buildinfo.deckhouseDir", o.DeckhouseDir),
		otattribute.String("buildinfo.versionFile", o.VersionFile),
		otattribute.String("buildinfo.editionFile", o.EditionFile),
	}
}

// AppVersion and AppEdition are populated by the linker via "-X" build flags
// (see dhctl/Makefile). They must not be assigned at runtime.
//
//nolint:gochecknoglobals // linker-injected build metadata
var (
	AppVersion = "local"
	AppEdition = "local"
)

// LoadBuildInfo populates BuildInfo using the linker-injected AppVersion/
// AppEdition as defaults, then overrides them with the contents of the
// version/edition files under deckhouseDir if they exist.
func LoadBuildInfo(deckhouseDir string) BuildInfo {
	bi := BuildInfo{
		AppVersion:   AppVersion,
		AppEdition:   AppEdition,
		DeckhouseDir: deckhouseDir,
		VersionFile:  filepath.Join(deckhouseDir, "version"),
		EditionFile:  filepath.Join(deckhouseDir, "edition"),
	}
	readBuildFile(&bi.AppVersion, bi.VersionFile)
	readBuildFile(&bi.AppEdition, bi.EditionFile)
	return bi
}

func readBuildFile(dst *string, filePath string) {
	file, err := os.OpenFile(filePath, os.O_RDONLY, 0644)
	if err != nil {
		return
	}
	defer file.Close()
	buf := make([]byte, 30)
	n, err := file.Read(buf)
	if n > 0 && (errors.Is(err, io.EOF) || err == nil) {
		*dst = strings.TrimSpace(string(buf[:n]))
		*dst = strings.ReplaceAll(*dst, "\n", "")
	}
}
