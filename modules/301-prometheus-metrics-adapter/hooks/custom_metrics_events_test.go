/*
Copyright 2021 Flant JSC

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
	"fmt"
	"math/rand"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/modules/301-prometheus-metrics-adapter/hooks/internal"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	ns1 = "ns1"
	ns2 = "ns2"
)

func choiceAnotherMetricType(cur string) string {
	a := make([]string, 0)
	for t := range internal.MetricsTypesForNsAndCluster() {
		if cur != t {
			a = append(a, t)
		}
	}

	indx := rand.Intn(len(a))
	return a[indx]
}

func metricTypesSetClusterWithNs() []string {
	return []string{choiceAnotherMetricType("")}
}

func genName(prefix, mType string) string {
	return fmt.Sprintf("%s-metric-%s", prefix, mType)
}

func genQuery(op, mType string) string {
	return fmt.Sprintf(`%s (go_goroutines{job="prometheus",type="%s"},<<.LabelMatchers>>}) by (<<.GroupBy>>)`, op, mType)
}

type metricForTest struct {
	internal.CustomMetric
	state string
}

func (t *metricForTest) FullPathForQuery() string {
	if t.Namespace == "" {
		return fmt.Sprintf("%s.cluster", t.PathToNamedContainer())
	}

	ns := strings.ReplaceAll(t.Namespace, ".", `\.`)
	return fmt.Sprintf("%s.%s", t.PathToNamespacedContainer(), ns)
}

func (t *metricForTest) PathToNamedContainer() string {
	name := strings.ReplaceAll(t.Name, ".", `\.`)
	return fmt.Sprintf("%s.%s.%s", internal.MetricsStatePathToRoot, t.Type, name)
}

func (t *metricForTest) PathToNamespacedContainer() string {
	return fmt.Sprintf("%s.%s", t.PathToNamedContainer(), internal.NamespacedPart)
}

func (t *metricForTest) assertQuery(f *HookExecutionConfig, msg string) {
	path := t.FullPathForQuery()
	expectNewQuery := f.ValuesGet(path).String()
	Expect(expectNewQuery).To(Equal(t.Query), msg)
}

func newMetricForTest(mType, ns, name, query string) metricForTest {
	state := fmt.Sprintf(`
apiVersion: deckhouse.io/v1beta1
kind: %s
metadata:
  name: %s
  namespace: %s
spec:
  query: %s

`, MetricKind(mType), name, ns, query)

	return metricForTest{
		state: state,
		CustomMetric: internal.CustomMetric{
			Type:      mType,
			Name:      name,
			Query:     query,
			Namespace: ns,
		},
	}
}

func newClusterMetricForTest(mType, name, query string) metricForTest {
	state := fmt.Sprintf(`
apiVersion: deckhouse.io/v1beta1
kind: %s
metadata:
  name: %s
spec:
  query: %s

`, ClusterMetricKind(mType), name, query)

	return metricForTest{
		state: state,
		CustomMetric: internal.CustomMetric{
			Type:  mType,
			Name:  name,
			Query: query,
		},
	}
}

func setMetricsToState(f *HookExecutionConfig, metrics []metricForTest) {
	s := make([]string, 0, len(metrics))
	for _, m := range metrics {
		s = append(s, m.state)
	}

	JoinKubeResourcesAndSet(f, s...)
}

func changeQuery(metrics []metricForTest, index int, newQuery string) []metricForTest {
	metricsWithChanged := make([]metricForTest, len(metrics))
	copy(metricsWithChanged, metrics)
	m := metrics[index]
	metricsWithChanged[index] = newMetricForTest(m.Type, m.Namespace, m.Name, newQuery)

	return metricsWithChanged
}
func namespacedMetricsSet(mType string) []metricForTest {
	anotherType := choiceAnotherMetricType(mType)

	return []metricForTest{
		// metric for change
		newMetricForTest(mType, ns1, genName("f", mType), genQuery("sum", mType)),
		// another metric in same namespace and type
		newMetricForTest(mType, ns1, genName("s", mType), genQuery("rate", mType)),
		// another metric type in same namespace
		newMetricForTest(anotherType, ns1, genName("t", anotherType), genQuery("sum", anotherType)),
		// another name space same type and name
		newMetricForTest(mType, ns2, genName("f", mType), genQuery("max", mType)),
		// cluster metric with same type
		newClusterMetricForTest(mType, genName("f", mType), genQuery("rate", mType)),
		// cluster metric with another type
		newClusterMetricForTest(anotherType, genName("f", anotherType), genQuery("rate", anotherType)),
		// namespace metric for same namespace
		newMetricForTest(internal.MetricNamespace, ns1, genName("f", internal.MetricNamespace), genQuery("sum", internal.MetricNamespace)),
	}
}

func clusterMetricsSet(mType string) []metricForTest {
	anotherType := choiceAnotherMetricType(mType)

	return []metricForTest{
		// cluster metric
		newClusterMetricForTest(mType, genName("f", mType), genQuery("sum", mType)),
		// another cluster metric with same type
		newClusterMetricForTest(mType, genName("s", mType), genQuery("rate", mType)),
		// cluster metric with another type
		newClusterMetricForTest(anotherType, genName("f", anotherType), genQuery("min", anotherType)),
		// namespaced metric with same type
		newMetricForTest(mType, ns1, genName("f", mType), genQuery("sum", mType)),
		// namespace metric
		newMetricForTest(internal.MetricNamespace, ns1, genName("f", internal.MetricNamespace), genQuery("sum", internal.MetricNamespace)),
	}
}

func namespaceMetricsSet(_ string) []metricForTest {
	mType := internal.MetricNamespace
	anotherType := choiceAnotherMetricType(mType)

	return []metricForTest{
		// namespace metric
		newMetricForTest(mType, ns1, genName("f", mType), genQuery("sum", mType)),
		// another namespace metric for same namespace
		newMetricForTest(mType, ns1, genName("s", mType), genQuery("rate", mType)),
		// namespace metric for another namespace but with same name
		newMetricForTest(mType, ns2, genName("f", mType), genQuery("sum", mType)),
		// another namespaced type metric for same namespace
		newMetricForTest(anotherType, ns1, genName("f", anotherType), genQuery("max", anotherType)),
		// another namespaced type metric for another namespace
		newMetricForTest(anotherType, ns2, genName("f", anotherType), genQuery("max", anotherType)),
		// cluster metric
		newClusterMetricForTest(anotherType, genName("f", anotherType), genQuery("rate", anotherType)),
	}
}

var _ = Describe("Prometheus metrics adapter :: custom_metrics_events ::", func() {
	f := HookExecutionConfigInit(`{"prometheusMetricsAdapter":{"internal": {}}}`, "")

	// register metrics
	for t := range internal.MetricsTypesForNsAndCluster() {
		f.RegisterCRD("deckhouse.io", "v1beta1", MetricKind(t), true)
		f.RegisterCRD("deckhouse.io", "v1beta1", ClusterMetricKind(t), false)
	}

	f.RegisterCRD("deckhouse.io", "v1beta1", MetricKind(internal.MetricNamespace), false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("sets map for every metrics type", func() {
			Expect(f).To(ExecuteSuccessfully())

			for mType := range internal.AllMetricsTypes {
				p := fmt.Sprintf("%s.%s", internal.MetricsStatePathToRoot, mType)
				c := f.ValuesGet(p)

				Expect(c.Exists()).To(BeTrue())
				Expect(c.IsObject()).To(BeTrue())
				Expect(c.Map()).To(HaveLen(0))
			}
		})
	})

	Context("Creating metrics", func() {
		type stateGeneratorFun func(typeForNew, nsForNew string) metricForTest

		newNamespacedMetricStateGen := func(typeForNew, nsForNew string) metricForTest {
			return newMetricForTest(typeForNew, nsForNew, genName("s", typeForNew), genQuery("rate", typeForNew))
		}

		wrapMetricForAssert := func(m metricForTest) func(string, string) metricForTest {
			return func(_ string, _ string) metricForTest {
				return m
			}
		}

		assertAddNewMetric := func(curStateGen stateGeneratorFun, newStateGen stateGeneratorFun) func(newTp, ns string) {
			return func(typeForNew, nsForNew string) {
				curMetric := curStateGen(typeForNew, nsForNew)
				newMetric := newStateGen(typeForNew, nsForNew)

				JoinKubeResourcesAndSet(f,
					curMetric.state,
					newMetric.state,
				)
				f.RunHook()

				Expect(f).To(ExecuteSuccessfully())

				newMetric.assertQuery(f, "sets new metric query into values")
				curMetric.assertQuery(f, "keeps query for exists metric")
			}
		}

		for _, mType := range metricTypesSetClusterWithNs() {
			Context(fmt.Sprintf("Namespaced metrics: %s", mType), func() {
				firstMetric := newMetricForTest(mType, ns1, genName("f", mType), genQuery("sum", mType))

				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(firstMetric.state))
					f.RunHook()
				})

				It("sets query into values", func() {
					Expect(f).To(ExecuteSuccessfully())

					firstMetric.assertQuery(f, "")
				})

				DescribeTable("adds another metric",
					assertAddNewMetric(wrapMetricForAssert(firstMetric), newNamespacedMetricStateGen),
					Entry("with same type and same namespace", mType, ns1),
					Entry("with same type and another namespace", mType, ns2),
					Entry("with another type and same namespace", choiceAnotherMetricType(mType), ns1),
					Entry("with another type and another namespace", choiceAnotherMetricType(mType), ns2),
					Entry("with 'namespace' type and same namespace", internal.MetricNamespace, ns1),
					Entry("with 'namespace' type and another namespace", internal.MetricNamespace, ns2),
				)
			})

			Context(fmt.Sprintf("Cluster metrics: %s", mType), func() {
				var firstMetric = newClusterMetricForTest(mType, genName("f", mType), genQuery("sum", mType))

				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(firstMetric.state))
					f.RunHook()
				})

				It("sets query into values", func() {
					Expect(f).To(ExecuteSuccessfully())

					firstMetric.assertQuery(f, "")
				})

				It("creates empty object for namespaced metrics", func() {
					Expect(f).To(ExecuteSuccessfully())

					obj := f.ValuesGet(firstMetric.PathToNamespacedContainer())

					Expect(obj.Exists()).To(BeTrue())
					Expect(obj.IsObject()).To(BeTrue())
					Expect(obj.Map()).To(HaveLen(0))
				})

				newClusterMetricStateGen := func(_, _ string) metricForTest {
					return newClusterMetricForTest(mType, genName("s", mType), genQuery("rate", mType))
				}

				DescribeTable("adds another cluster metric",
					assertAddNewMetric(wrapMetricForAssert(firstMetric), newClusterMetricStateGen),
					Entry("with same type", mType, ""),
					Entry("with another type", choiceAnotherMetricType(mType), ""),
				)

				DescribeTable("adds another namespaced metric",
					assertAddNewMetric(wrapMetricForAssert(firstMetric), newNamespacedMetricStateGen),
					Entry("with same type", mType, ns1),
					Entry("with another type ", choiceAnotherMetricType(mType), ns1),
					Entry("with 'namespace' type", internal.MetricNamespace, ns1),
				)
			})
		}

		Context("Namespace metrics", func() {
			const t = internal.MetricNamespace
			firstMetric := newMetricForTest(t, ns1, genName("f", t), genQuery("sum", t))

			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(firstMetric.state))
				f.RunHook()
			})

			It("sets query into values", func() {
				Expect(f).To(ExecuteSuccessfully())

				firstMetric.assertQuery(f, "")
			})

			DescribeTable("adds another namespace metric",
				assertAddNewMetric(wrapMetricForAssert(firstMetric), newNamespacedMetricStateGen),
				Entry("with same namespace", t, ns1),
				Entry("with another namespace ", t, ns2),
			)
		})

		DescribeTable("Correct handle dot symbols", func(metric metricForTest) {
			f.BindingContexts.Set(f.KubeStateSet(metric.state))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())

			metric.assertQuery(f, "creates query")
		},
			Entry("in name", newMetricForTest(choiceAnotherMetricType(""), ns1, "name.with.dot", genQuery("sum", "not-matter"))),
			Entry("in namespace", newMetricForTest(choiceAnotherMetricType(""), "ns.with.dot", "f", genQuery("sum", "not-matter"))),
		)
	})

	Context("Editing metrics", func() {
		assertEdit := func(mType string, metricsCreator func(string) []metricForTest, changeFun func([]metricForTest, int, string) []metricForTest) {
			metrics := metricsCreator(mType)

			setMetricsToState(f, metrics)
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully(), "sets original state")

			newQuery := genQuery("min", mType)
			editMetricIndex := 0

			metricsWithChanged := changeFun(metrics, editMetricIndex, newQuery)

			setMetricsToState(f, metricsWithChanged)
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully(), "edits metric run successfully")

			editedMetrics := metrics[editMetricIndex]
			Expect(editedMetrics.Query).ToNot(Equal(newQuery), "should metrics are different")
			expectNewQuery := f.ValuesGet(editedMetrics.FullPathForQuery()).String()
			Expect(expectNewQuery).To(Equal(newQuery), "changes query into values")

			for i, m := range metrics {
				if i != editMetricIndex {
					m.assertQuery(f, "does not change another metrics")
				}
			}
		}

		for _, mType := range metricTypesSetClusterWithNs() {
			changeClusterQuery := func(metrics []metricForTest, index int, newQuery string) []metricForTest {
				metricsWithChanged := make([]metricForTest, len(metrics))
				copy(metricsWithChanged, metrics)
				m := metrics[index]
				metricsWithChanged[index] = newClusterMetricForTest(m.Type, m.Name, newQuery)
				return metricsWithChanged
			}

			DescribeTable("Changing metrics query by kind",
				assertEdit,
				Entry(fmt.Sprintf("Namespaced metrics: %s", mType), mType, namespacedMetricsSet, changeQuery),
				Entry(fmt.Sprintf("Cluster metrics: %s", mType), mType, clusterMetricsSet, changeClusterQuery),
			)
		}

		DescribeTable("Changing metrics query by kind",
			assertEdit,
			Entry("Namespace metrics", internal.MetricNamespace, namespaceMetricsSet, changeQuery),
		)

	})

	Context("Deleting metrics", func() {
		assertDeleted := func(mType string, metricsCreator func(string) []metricForTest) {
			metrics := metricsCreator(mType)
			setMetricsToState(f, metrics)
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully(), "sets original state")

			deleteMetricIndex := 0
			metricsAfterDelete := metrics[deleteMetricIndex+1:]

			setMetricsToState(f, metricsAfterDelete)
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully(), "deletes metric run successfully")

			exists := f.ValuesGet(metrics[deleteMetricIndex].FullPathForQuery()).Exists()
			Expect(exists).To(BeFalse(), "deletes query from values")

			for i, m := range metrics {
				if i != deleteMetricIndex {
					m.assertQuery(f, "does not change another metrics")
				}
			}
		}

		assertCleanup := func(metric metricForTest) {
			setMetricsToState(f, []metricForTest{metric})
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully(), "sets original state")

			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully(), "deletes metric run successfully")

			exists := f.ValuesGet(metric.PathToNamedContainer()).Exists()
			Expect(exists).To(BeFalse(), "cleanups named container")

		}

		for _, mType := range metricTypesSetClusterWithNs() {
			DescribeTable("Deleting metric by kind",
				assertDeleted,
				Entry(fmt.Sprintf("Namespaced metrics: %s", mType), mType, namespacedMetricsSet),
				Entry(fmt.Sprintf("Cluster metrics: %s", mType), mType, clusterMetricsSet),
			)

			DescribeTable("Cleaning named map container after delete all metrics",
				assertCleanup,
				Entry(fmt.Sprintf("Namespaced metrics: %s", mType), namespacedMetricsSet(mType)[0]),
				Entry(fmt.Sprintf("Cluster metrics: %s", mType), clusterMetricsSet(mType)[0]),
			)
		}

		DescribeTable("Deleting metric by kind",
			assertDeleted,
			Entry("Namespace metrics", internal.MetricNamespace, namespaceMetricsSet),
		)

		DescribeTable("Cleaning named map container after delete all metrics",
			assertCleanup,
			Entry(fmt.Sprintf("Namespaced metrics: %s", internal.MetricNamespace), namespacedMetricsSet(internal.MetricNamespace)[0]),
		)
	})
})
