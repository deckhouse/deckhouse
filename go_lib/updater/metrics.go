// Copyright 2024 Flant JSC
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

package updater

import "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"

type MetricLabels map[string]string

const (
	ManualApprovalRequired     = "manualApproval"
	DisruptionApprovalRequired = "disruptionApproval"
	RequirementsNotMet         = "requirementsNotMet"
	ReleaseQueueDepth          = "releaseQueueDepth"
	NotificationNotSent        = "notificationNotSent"
)

func NewReleaseMetricLabels(release v1alpha1.Release) MetricLabels {
	labels := make(map[string]string, 6)
	labels[ManualApprovalRequired] = "false"
	labels[DisruptionApprovalRequired] = "false"
	labels[RequirementsNotMet] = "false"
	labels[ReleaseQueueDepth] = "nil"
	labels["name"] = release.GetName()
	labels[NotificationNotSent] = "false"

	if _, ok := release.(*v1alpha1.ModuleRelease); ok {
		labels["moduleName"] = release.GetModuleName()
	}

	return labels
}

type MetricsUpdater[R v1alpha1.Release] interface {
	UpdateReleaseMetric(string, MetricLabels)
	PurgeReleaseMetric(string)
}
