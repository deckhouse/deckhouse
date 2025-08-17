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
	"fmt"
	"time"

	"github.com/flant/docs-builder/internal/metrics"
)

func (svc *Service) Delete(moduleName string, channels []string) error {
	start := time.Now()
	status := "ok"
	defer func() {
		dur := time.Since(start).Seconds()
		svc.metrics.CounterAdd(metrics.DocsBuilderDeleteTotal, 1, map[string]string{"status": status})
		svc.metrics.HistogramObserve(metrics.DocsBuilderDeleteDurationSeconds, dur, map[string]string{"status": status}, nil)
	}()

	err := svc.cleanModulesFiles(moduleName, channels)
	if err != nil {
		status = "fail"

		return fmt.Errorf("clean module files: %w", err)
	}

	err = svc.removeFromChannelMapping(moduleName, channels)
	if err != nil {
		return fmt.Errorf("remove from channel mapping:%w", err)
	}

	return nil
}

func (svc *Service) removeFromChannelMapping(moduleName string, channels []string) error {
	return svc.channelMappingEditor.edit(func(m channelMapping) {
		for _, channel := range channels {
			delete(m[moduleName][channelMappingChannels], channel)
		}

		if len(m[moduleName][channelMappingChannels]) == 0 {
			delete(m, moduleName)
		}
	})
}
