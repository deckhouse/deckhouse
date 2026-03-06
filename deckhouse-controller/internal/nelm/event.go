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
	"fmt"

	"github.com/werf/nelm/pkg/legacy/progrep"
	"gopkg.in/yaml.v3"
)

// TrackingEvent represents a helm release tracking event containing objects currently being created.
type TrackingEvent struct {
	Creating []objectReport `yaml:"Creating,omitempty"`
}

// objectReport describes a single Kubernetes object being tracked, along with
// the resources it is waiting on before becoming ready.
type objectReport struct {
	Name    string   `yaml:"Name"`
	Waiting []string `yaml:"Waiting,omitempty"`
}

// reportToTrackingEvent converts a nelm ProgressReport into an Event by extracting
// all readiness-tracking operations that are still progressing from the latest stage.
func reportToTrackingEvent(report progrep.ProgressReport) TrackingEvent {
	if len(report.StageReports) == 0 {
		return TrackingEvent{}
	}

	event := TrackingEvent{}
	// Only inspect the most recent stage report.
	for _, op := range report.StageReports[len(report.StageReports)-1].Operations {
		if op.Type != progrep.OperationTypeTrackReadiness {
			continue
		}

		if op.Status != progrep.OperationStatusProgressing {
			continue
		}

		waiting := make([]string, 0, len(op.WaitingFor))
		for _, obj := range op.WaitingFor {
			waiting = append(waiting, obj.String())
		}

		event.Creating = append(event.Creating, objectReport{
			Name:    formatObjectRef(op.ObjectRef),
			Waiting: waiting,
		})
	}

	return event
}

func formatObjectRef(ref progrep.ObjectRef) string {
	if ref.Group != "" {
		return fmt.Sprintf("%s.%s/%s", ref.Kind, ref.Group, ref.Name)
	}
	return fmt.Sprintf("%s/%s", ref.Kind, ref.Name)
}

// String marshals the Event to YAML for human-readable output.
func (e TrackingEvent) String() string {
	marshalled, _ := yaml.Marshal(e)
	return string(marshalled)
}
