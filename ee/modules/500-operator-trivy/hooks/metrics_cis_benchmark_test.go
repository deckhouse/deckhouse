/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: operator-trivy :: hooks :: metrics for cis benchmark ::", func() {

	f := HookExecutionConfigInit("", "")
	f.RegisterCRD("aquasecurity.github.io", "v1alpha1", "ClusterComplianceReport", false)

	assertMetricLabels := func(metricLabels, expectedLabels map[string]string) {
		Expect(len(metricLabels)).To(Equal(len(expectedLabels)))
		for k := range metricLabels {
			Expect(metricLabels[k]).To(Equal(expectedLabels[k]))
		}
	}

	assertMetric := func(metrics []operation.MetricOperation, metricName, id string, value float64, expectedLabels map[string]string) {
		metricIndex := -1
		for i, m := range metrics {
			if m.Name == metricName && m.Labels["id"] == id {
				assertMetricLabels(m.Labels, expectedLabels)
				Expect(m.Value).To(Equal(ptr.To(value)))
				metricIndex = i
				break
			}
		}

		Expect(metricIndex >= 0).To(BeTrue())
	}

	Context(":: empty_cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context(":: cis_report_not_ready", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(testGetNotReadyClippedCisBecnmark()))
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	type cisMetricsTestCase struct {
		name       string
		testCase   string
		totalFails []float64
	}

	cisReports := []cisMetricsTestCase{
		{name: ":: summary_cis_report_ready", testCase: testGetReadyClippedSummaryCisBecnmark(), totalFails: []float64{0, 0, 10}},
		{name: ":: detailed_cis_report_ready", testCase: testGetReadyClippedDetailedCisBecnmark(), totalFails: []float64{1, 1, 0}},
	}

	testGenerateMetrics := func(tc cisMetricsTestCase) {
		Context(tc.name, func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(tc.testCase))
				f.RunHook()
			})

			It("Hook must execute successfully", func() {
				Expect(f).To(ExecuteSuccessfully())
			})

			It("Sets metric proper values", func() {
				metrics := f.MetricsCollector.CollectedMetrics()
				metricName := "deckhouse_trivy_cis_benchmark"
				assertMetric(metrics, metricName, "1.1.1", tc.totalFails[0], testGetLabelsMap("1.1.1", "Ensure that the API server pod specification file permissions are set to 600 or more restrictive", "HIGH"))
				assertMetric(metrics, metricName, "1.1.2", tc.totalFails[1], testGetLabelsMap("1.1.2", "Ensure that the API server pod specification file ownership is set to root:root", "MEDIUM"))
				assertMetric(metrics, metricName, "1.1.3", tc.totalFails[2], testGetLabelsMap("1.1.3", "Ensure that the controller manager pod specification file permissions are set to 600 or more restrictive", "LOW"))
			})
		})
	}

	for _, tc := range cisReports {
		testGenerateMetrics(tc)
	}
})

func testGetNotReadyClippedCisBecnmark() string {
	return `
---
apiVersion: aquasecurity.github.io/v1alpha1
kind: ClusterComplianceReport
metadata:
  name: cis
  labels:
    app: operator-trivy
    app.kubernetes.io/managed-by": Helm
    heritage: deckhouse,
    module: operator-trivy
spec:
  cron: 0 */6 * * *
  reportType: summary
  compliance:
    id: cis
    title: CIS Kubernetes Benchmarks v1.23
    description: CIS Kubernetes Benchmarks
    relatedResources:
      - https://www.cisecurity.org/benchmark/kubernetes
    version: "1.0"
    controls:
      - id: 1.1.1
        name:
          Ensure that the API server pod specification file permissions are set to
          600 or more restrictive
        description:
          Ensure that the API server pod specification file has permissions
          of 600 or more restrictive
        checks:
          - id: AVD-KCV-0048
        severity: HIGH
      - id: 1.1.2
        name:
          Ensure that the API server pod specification file ownership is set to
          root:root
        description:
          Ensure that the API server pod specification file ownership is set
          to root:root
        checks:
          - id: AVD-KCV-0049
        severity: MEDIUM
      - id: 1.1.3
        name:
          Ensure that the controller manager pod specification file permissions are
          set to 600 or more restrictive
        description:
          Ensure that the controller manager pod specification file has
          permissions of 600 or more restrictive
        checks:
          - id: AVD-KCV-0050
        severity: LOW`
}

func testGetReadyClippedSummaryCisBecnmark() string {
	return testGetNotReadyClippedCisBecnmark() + `
status:
  summary:
    failCount: 1
    passCount: 2
  summaryReport:
    controlCheck:
      - id: 1.1.1
        name: Ensure that the API server pod specification file permissions are set
          to 600 or more restrictive
        severity: HIGH
        totalFail: 0
      - id: 1.1.2
        name: Ensure that the API server pod specification file ownership is set to
          root:root
        severity: MEDIUM
      - id: 1.1.3
        name: Ensure that the controller manager pod specification file permissions
          are set to 600 or more restrictive
        severity: LOW
        totalFail: 10`
}
func testGetReadyClippedDetailedCisBecnmark() string {
	return testGetNotReadyClippedCisBecnmark() + `
status:
  summary:
    failCount: 1
    passCount: 2
  detailReport:
    description: CIS Kubernetes Benchmarks
    id: cis
    relatedVersion:
    - https://www.cisecurity.org/benchmark/kubernetes
    results:
      - checks:
          - checkID: 'test'
            severity: 'severity'
            success: false
        description: Ensure that the API server pod specification file has permissions
          of 600 or more restrictive
        id: 1.1.1
        name: Ensure that the API server pod specification file permissions are set to
          600 or more restrictive
        severity: HIGH
      - checks:
          - checkID: 'test'
            severity: 'severity'
            success: false
        description: Ensure that the API server pod specification file ownership is set
          to root:root
        id: 1.1.2
        name: Ensure that the API server pod specification file ownership is set to root:root
        severity: MEDIUM
      - checks:
          - checkID: 'test'
            severity: 'severity'
            success: true
          - checkID: 'test'
            severity: 'severity'
            success: true
        description: Ensure that the controller manager pod specification file has permissions
          of 600 or more restrictive
        id: 1.1.3
        name: Ensure that the controller manager pod specification file permissions are
          set to 600 or more restrictive
        severity: LOW`
}

func testGetLabelsMap(id, name, severity string) map[string]string {
	return map[string]string{"id": id, "name": name, "severity": severity}
}
