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
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
	"github.com/spf13/fsync"

	"github.com/flant/docs-builder/internal/metrics"
)

var docConfValuesRegexp = regexp.MustCompile(`^openapi/(doc-.*-config-values\.yaml|conversions/v\d+\.yaml)$`)

// /app/hugo/{data||content}/modules/{module}/{channel}/{brokenFile}:53:1
// $1 - Module dir path /app/hugo/{data||content}/modules/{module}
// $2 - {module}
var assembleErrorRegexp = regexp.MustCompile(`"(?P<base>.+?/modules/(?P<module>[^/]+))/(?P<path>.+?):(?P<line>\d+):(?P<column>\d+)"`)

const (
	hugoInitDir = "/app/hugo-init/"
	modulesDir  = "data/modules/"
	contentDir  = "content/modules/"
)

type Service struct {
	baseDir              string
	destDir              string
	isReady              atomic.Bool
	channelMappingEditor *channelMappingEditor

	logger  *log.Logger
	metrics *metricsstorage.MetricStorage
}

func NewService(baseDir, destDir string, highAvailability bool, logger *log.Logger, ms *metricsstorage.MetricStorage) *Service {
	svc := &Service{
		baseDir:              baseDir,
		destDir:              destDir,
		channelMappingEditor: newChannelMappingEditor(baseDir),
		logger:               logger,
		metrics:              ms,
	}

	if !highAvailability {
		svc.isReady.Store(true)
	}

	// prepare module directory
	err := os.MkdirAll(filepath.Join(baseDir, modulesDir), 0700)
	if err != nil {
		svc.logger.Error("mkdir all", log.Err(err))
	}

	syncer := fsync.NewSyncer()
	syncer.NoChmod = true
	syncer.NoTimes = true
	// do not delete files in baseDir
	syncer.DeleteFilter = func(_ fsync.FileInfo) bool {
		return false
	}

	oldLocation := filepath.Join(hugoInitDir)
	newLocation := filepath.Join(svc.baseDir)
	err = syncer.Sync(newLocation, oldLocation)
	if err != nil {
		svc.logger.Error("sync init folder with base dir", log.Err(err))
	}

	svc.metrics.AddCollectorFunc(func(s metricsstorage.Storage) {
		modulesCount, err := svc.channelMappingEditor.getModulesCount()
		if err != nil {
			svc.logger.Warn("can not read modules count from channel mapping editor")
		}

		s.GaugeSet(metrics.DocsBuilderCachedModules, float64(modulesCount), nil)
	})

	return svc
}

func (svc *Service) IsReady() bool {
	return svc.isReady.Load()
}
