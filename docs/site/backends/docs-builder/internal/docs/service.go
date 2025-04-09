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
	"os"
	"path/filepath"
	"regexp"
	"sync/atomic"

	"github.com/deckhouse/deckhouse/pkg/log"
)

var docConfValuesRegexp = regexp.MustCompile(`^openapi/(doc-.*-config-values\.yaml|conversions/v\d+\.yaml)$`)

// /app/hugo/{data||content}/modules/{module}/{channel}/{brokenFile}:53:1
// $1 - Module dir path /app/hugo/{data||content}/modules/{module}
// $2 - {module}
var assembleErrorRegexp = regexp.MustCompile(`"(?P<base>.+?/modules/(?P<module>[^/]+))/(?P<path>.+?):(?P<line>\d+):(?P<column>\d+)"`)

const (
	modulesDir = "data/modules/"
	contentDir = "content/modules/"
)

type Service struct {
	baseDir              string
	destDir              string
	isReady              atomic.Bool
	channelMappingEditor *channelMappingEditor

	logger *log.Logger
}

func NewService(baseDir, destDir string, highAvailability bool, logger *log.Logger) *Service {
	svc := &Service{
		baseDir:              baseDir,
		destDir:              destDir,
		channelMappingEditor: newChannelMappingEditor(baseDir),
		logger:               logger,
	}

	if !highAvailability {
		svc.isReady.Store(true)
	}

	// prepare module directory
	err := os.MkdirAll(filepath.Join(baseDir, modulesDir), 0700)
	if err != nil {
		svc.logger.Error("mkdir all", log.Err(err))
	}

	return svc
}

func (svc *Service) IsReady() bool {
	return svc.isReady.Load()
}
