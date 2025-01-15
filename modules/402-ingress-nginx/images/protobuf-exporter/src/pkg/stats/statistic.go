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

package stats

import "github.com/prometheus/client_golang/prometheus"

var (
	Messages = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "protobuf_exporter_messages_total",
			Help: "The total number of metric messages seen.",
		},
		[]string{"type"},
	)
	Errors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "protobuf_exporter_errors_total",
			Help: "The number of errors encountered.",
		},
		[]string{"type"},
	)
)

func init() {
	prometheus.MustRegister(Messages)
	prometheus.MustRegister(Errors)
}
