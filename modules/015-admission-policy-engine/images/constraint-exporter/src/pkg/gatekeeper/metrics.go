/*
Copyright 2022 Flant JSC

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

package gatekeeper

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	prefix = "d8_gatekeeper_exporter"

	Up = prometheus.NewDesc(
		prometheus.BuildFQName(prefix, "", "up"),
		"Was the last Gatekeeper scraper scorecard query successful.",
		nil, nil,
	)
	ConstraintViolation = prometheus.NewDesc(
		prometheus.BuildFQName(prefix, "", "constraint_violations"),
		"OPA violations for all constraints",
		[]string{"kind", "name", "violating_kind", "violating_name", "violating_namespace", "violation_msg", "violation_enforcement", "source_type"}, nil,
	)
	ConstraintInformation = prometheus.NewDesc(
		prometheus.BuildFQName(prefix, "", "constraint_information"),
		"Some general information of all constraints",
		[]string{"kind", "name", "enforcementAction", "totalViolations"}, nil,
	)
)

func ExportViolations(constraints []Constraint) []prometheus.Metric {
	m := make([]prometheus.Metric, 0)
	for _, c := range constraints {
		if c.Status.TotalViolations == 0 {
			metric := prometheus.MustNewConstMetric(ConstraintViolation, prometheus.GaugeValue, 0, c.Meta.Kind, c.Meta.Name, "", "", "", "", "", c.Meta.SourceType)
			m = append(m, metric)
		} else{
			for _, v := range c.Status.Violations {
				metric := prometheus.MustNewConstMetric(ConstraintViolation, prometheus.GaugeValue, 1, c.Meta.Kind, c.Meta.Name, v.Kind, v.Name, v.Namespace, v.Message, v.EnforcementAction, c.Meta.SourceType)
				m = append(m, metric)
			}
		}
	}
	return m
}

func ExportConstraintInformation(constraints []Constraint) []prometheus.Metric {
	m := make([]prometheus.Metric, 0)
	for _, c := range constraints {
		metric := prometheus.MustNewConstMetric(ConstraintInformation, prometheus.GaugeValue, c.Status.TotalViolations, c.Meta.Kind, c.Meta.Name, c.Spec.EnforcementAction, fmt.Sprintf("%f", c.Status.TotalViolations))
		m = append(m, metric)
	}
	return m
}
