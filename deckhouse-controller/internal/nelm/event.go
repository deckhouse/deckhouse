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

package nelm

import (
	"github.com/werf/nelm/pkg/legacy/progrep"
	"gopkg.in/yaml.v3"
)

type Report struct {
	Creating []objectReport `yaml:"Creating"`
}

type objectReport struct {
	Name    string   `yaml:"Name"`
	Waiting []string `yaml:"Waiting"`
}

func eventToReport(event progrep.ProgressReport) Report {
	if len(event.StageReports) == 0 {
		return Report{}
	}

	report := Report{}
	for _, op := range event.StageReports[len(event.StageReports)-1].Operations {
		if op.Type != progrep.OperationTypeTrackReadiness {
			continue
		}

		if op.Status != progrep.OperationStatusProgressing {
			continue
		}

		waiting := make([]string, len(op.WaitingFor))
		for _, obj := range op.WaitingFor {
			waiting = append(waiting, obj.String())
		}

		report.Creating = append(report.Creating, objectReport{
			Name:    op.ObjectRef.String(),
			Waiting: waiting,
		})
	}

	return report
}

func (r Report) Marshal() []byte {
	marshalled, _ := yaml.Marshal(r)
	return marshalled
}
