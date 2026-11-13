// Copyright 2023 Flant JSC
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

package docs

import (
	"regexp"

	"github.com/spf13/afero"

	"github.com/deckhouse/deckhouse/pkg/log"
)

var docConfValuesRegexp = regexp.MustCompile(`^openapi/(doc-.*-config-values\.yaml|conversions/v\d+\.yaml)$`)

// <workDir>/site/{data||content}/modules/{module}/{version}/{brokenFile}:53:1
// (the site of a version is built in a temporary directory, see RenderVersion)
// $1 - Module dir path <workDir>/site/{data||content}/modules/{module}
// $2 - {module}
var assembleErrorRegexp = regexp.MustCompile(`"(?P<base>.+?/modules/(?P<module>[^/]+))/(?P<path>.+?):(?P<line>\d+):(?P<column>\d+)"`)

const (
	hugoInitDir = "/app/hugo-init/"
	modulesDir  = "data/modules/"
	contentDir  = "content/modules/"
)

// Service is a Hugo site in baseDir: the site of one module version
// (RenderVersion) or of the list of modules (RenderModulesList).
type Service struct {
	baseDir string

	// wrapDestination is passed to Hugo as hugo.Flags.WrapDestination.
	wrapDestination func(afero.Fs) afero.Fs

	logger *log.Logger
}
