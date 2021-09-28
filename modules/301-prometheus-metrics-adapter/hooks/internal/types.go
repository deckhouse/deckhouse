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

package internal

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	MetricDaemonSet   = "daemonset"
	MetricDeployment  = "deployment"
	MetricIngress     = "ingress"
	MetricNamespace   = "namespace"
	MetricPod         = "pod"
	MetricService     = "service"
	MetricStatefulSet = "statefulset"
)

// AllMetricsTypes type (in lower case) => kind
var AllMetricsTypes = map[string]string{
	MetricDaemonSet:   "DaemonSet",
	MetricDeployment:  "Deployment",
	MetricIngress:     "Ingress",
	MetricNamespace:   "Namespace",
	MetricPod:         "Pod",
	MetricService:     "Service",
	MetricStatefulSet: "StatefulSet",
}

func MetricsTypesForNsAndCluster() map[string]string {
	res := make(map[string]string)

	for t, kind := range AllMetricsTypes {
		if t != MetricNamespace {
			res[t] = kind
		}
	}

	return res
}

var metricTypeExtractRegExp = regexp.MustCompile("^(Cluster)?(.*)Metric$")

func ExtractMetricTypeFromKind(kind string) (string, error) {
	matches := metricTypeExtractRegExp.FindStringSubmatch(kind)

	metricType := ""

	switch len(matches) {
	case 2:
		metricType = matches[1]
	case 3:
		metricType = matches[2]
	}

	metricType = strings.ToLower(metricType)

	if metricType == "" || strings.HasPrefix(metricType, "cluster") {
		return "", fmt.Errorf("cannot extract CustomMetricType from kind: %s", kind)
	}

	return metricType, nil
}
