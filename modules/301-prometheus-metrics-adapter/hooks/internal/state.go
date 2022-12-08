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

const (
	MetricsStatePathToRoot = "prometheusMetricsAdapter.internal.customMetrics"

	ClusteredPart  = "cluster"
	NamespacedPart = "namespaced"
)

type CustomMetric struct {
	Type      string
	Namespace string
	Name      string
	Query     string
}

// MetricsQueriesState
// Examples for full path
// namespaced
//
//	prometheusMetricsAdapter.internal.customMetrics.pod.name.namespaced.ns1
//
// cluster
//
//	prometheusMetricsAdapter.internal.customMetrics.pod.name.cluster
//
// in this state we have all keys after 'customMetrics'
// we collect all current metrics and replace 'customMetrics'
// 'customMetrics' is map which should have (metric type) 'pod, ingress... etc' keys
// in constructor we create it
type MetricsQueriesState struct {
	State map[string]map[string]interface{}
}

func NewMetricsQueryValues() *MetricsQueriesState {
	state := make(map[string]map[string]interface{})
	for t := range AllMetricsTypes {
		state[t] = make(map[string]interface{})
	}

	return &MetricsQueriesState{
		State: state,
	}
}

func (s *MetricsQueriesState) AddMetric(m *CustomMetric) {
	stateForNameRaw, ok := s.State[m.Type][m.Name]
	var stateForName map[string]interface{}
	if ok {
		stateForName = stateForNameRaw.(map[string]interface{})
	} else {
		stateForName = map[string]interface{}{
			NamespacedPart: make(map[string]interface{}),
		}
	}

	queryState := stateForName
	queryKey := ClusteredPart

	if m.Namespace != "" {
		queryKey = m.Namespace
		// map always is here. see above
		queryState = stateForName[NamespacedPart].(map[string]interface{})
	}

	queryState[queryKey] = m.Query
	s.State[m.Type][m.Name] = stateForName
}
