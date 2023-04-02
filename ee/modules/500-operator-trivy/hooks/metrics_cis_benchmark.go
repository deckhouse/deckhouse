/*
Copyright 2023 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hooks

import (
	"errors"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/500-operator-trivy/hooks/internal/apis/v1alpha1"
)

const (
	metricGroupName = "deckhouse_cluster_compliance_cis"
	metricName      = "deckhouse_trivy_cis_benchmark"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/operator-trivy/cis_benchmark",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "cluster_compliance_reports",
			ApiVersion: "aquasecurity.github.io/v1alpha1",
			Kind:       "ClusterComplianceReport",
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"module": "operator-trivy",
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"cis"},
			},
			FilterFunc: filterClusterComplianceReport,
		},
	},
}, cisBencmarkMetricHandler)

func filterClusterComplianceReport(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	compReport := &v1alpha1.ClusterComplianceReport{}

	err := sdk.FromUnstructured(obj, compReport)
	if err != nil {
		return nil, err
	}
	return compReport.Status.SummaryReport, nil
}

func cisBencmarkMetricHandler(input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(metricGroupName)

	snap := input.Snapshots["cluster_compliance_reports"]
	if len(snap) < 1 {
		input.LogEntry.Errorln("No CIS benchmarks found")
		return nil
	}

	cisReport, ok := snap[0].(*v1alpha1.SummaryReport)
	if !ok {
		return errors.New("can't use snapshot as SummaryReport")
	}

	if cisReport == nil {
		input.LogEntry.Errorln("CIS benchmark didn't run")
		return nil
	}

	for _, controlCheck := range cisReport.SummaryControls {
		var totalFail float64
		if controlCheck.TotalFail != nil {
			totalFail = float64(*controlCheck.TotalFail)
		}

		input.MetricsCollector.Set(
			metricName,
			totalFail,
			map[string]string{"id": controlCheck.ID, "name": controlCheck.Name, "severity": controlCheck.Severity},
			metrics.WithGroup(metricGroupName),
		)
	}
	return nil
}
