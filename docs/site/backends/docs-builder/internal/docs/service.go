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

	"k8s.io/klog/v2"
)

var docConfValuesRegexp = regexp.MustCompile(`^openapi/doc-.*-config-values\.yaml$`)
var assembleErrorRegexp = regexp.MustCompile(`error building site: assemble: (\x1b\[1;36m)?"(?P<path>.+):(?P<line>\d+):(?P<column>\d+)"(\x1b\[0m)?:`)

const (
	modulesDir = "data/modules/"
	contentDir = "content/modules/"
)

type Service struct {
	baseDir              string
	destDir              string
	isReady              atomic.Bool
	channelMappingEditor *channelMappingEditor
}

func NewService(baseDir, destDir string, highAvailability bool) *Service {
	svc := &Service{
		baseDir:              baseDir,
		destDir:              destDir,
		channelMappingEditor: newChannelMappingEditor(baseDir),
	}

	if !highAvailability {
		svc.isReady.Store(true)
	}

	// prepare module directory
	err := os.MkdirAll(filepath.Join(baseDir, modulesDir), 0700)
	if err != nil {
		klog.Error(err)
	}

	return svc
}

func (svc *Service) IsReady() bool {
	return svc.isReady.Load()
}
