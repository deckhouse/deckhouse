/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkpkg "github.com/deckhouse/module-sdk/pkg"

	"github.com/deckhouse/deckhouse/ee/modules/500-operator-trivy/hooks/internal/apis/v1alpha1"
)

const (
	metricGroupName   = "deckhouse_cluster_compliance_cis"
	metricName        = "deckhouse_trivy_cis_benchmark"
	cisBenchmarkQueue = "cis_benchmark_reports"
)

type filteredComplianceReport struct {
	SummaryControls  []v1alpha1.ControlCheckSummary
	DetailedControls []*v1alpha1.ControlCheckResult
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/operator-trivy/cis_benchmark",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       cisBenchmarkQueue,
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

	filteredResult := filteredComplianceReport{}
	switch {
	case compReport.Status.DetailReport != nil:
		filteredResult.DetailedControls = compReport.Status.DetailReport.Results
	case compReport.Status.SummaryReport != nil:
		filteredResult.SummaryControls = compReport.Status.SummaryReport.SummaryControls
	}
	return filteredResult, nil
}

func cisBencmarkMetricHandler(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(metricGroupName)

	snaps := input.Snapshots.Get(cisBenchmarkQueue)
	if len(snaps) == 0 {
		input.Logger.Error("No CIS benchmark found")
		return nil
	}

	compReport := new(filteredComplianceReport)

	err := snaps[0].UnmarshalTo(compReport)
	if err != nil {
		return fmt.Errorf("cannot unmarshal filtered compliance report: %w", err)
	}

	switch {
	case compReport.DetailedControls != nil:
		generateDetailedMetrics(input.MetricsCollector, compReport.DetailedControls)
	case compReport.SummaryControls != nil:
		generateSummaryMetrics(input.MetricsCollector, compReport.SummaryControls)
	default:
		input.Logger.Error("CIS benchmark didn't run")
	}
	return nil
}

func generateSummaryMetrics(metricsCollector sdkpkg.MetricsCollector, summaryChecks []v1alpha1.ControlCheckSummary) {
	for _, controlCheck := range summaryChecks {
		var (
			totalFails float64
		)

		if controlCheck.TotalFail != nil && *controlCheck.TotalFail != 0 {
			totalFails = float64(*controlCheck.TotalFail)
		}

		generateCisBenchmarkMetric(metricsCollector, totalFails, controlCheck.ID, controlCheck.Name, controlCheck.Severity)
	}
}

func generateDetailedMetrics(metricsCollector sdkpkg.MetricsCollector, detailedChecks []*v1alpha1.ControlCheckResult) {
	for _, controlCheck := range detailedChecks {
		if controlCheck == nil {
			continue
		}
		totalFails := countTotalFailsFromDetailedChecks(controlCheck.Checks)
		generateCisBenchmarkMetric(metricsCollector, totalFails, controlCheck.ID, controlCheck.Name, controlCheck.Severity)
	}
}

func countTotalFailsFromDetailedChecks(checks []v1alpha1.ComplianceCheck) float64 {
	var totalFails float64
	for _, check := range checks {
		if !check.Success {
			totalFails++
		}
	}
	return totalFails
}

func generateCisBenchmarkMetric(metricsCollector sdkpkg.MetricsCollector, totalFails float64, id, name, severity string) {
	metricsCollector.Set(
		metricName,
		totalFails,
		map[string]string{"id": id, "name": name, "severity": severity},
		metrics.WithGroup(metricGroupName),
	)
}
