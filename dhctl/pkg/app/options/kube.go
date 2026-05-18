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

package options

import (
	otattribute "go.opentelemetry.io/otel/attribute"
)

// KubeOptions describes how dhctl reaches the Kubernetes API.
type KubeOptions struct {
	Config        string
	ConfigContext string
	InCluster     bool
}

// IsDefined reports whether any of the kube flags were explicitly set.
// Replaces the previous pkg/app.KubeFlagsDefined helper.
func (o *KubeOptions) IsDefined() bool {
	return len(o.Config) > 0 || len(o.ConfigContext) > 0 || o.InCluster
}

func (o *KubeOptions) ToSpanAttributes() []otattribute.KeyValue {
	return []otattribute.KeyValue{
		// otattribute.String("kube.kubeconfig", o.Config), // todo: validate than here NO private data
		otattribute.String("kube.kubeconfig-context", o.ConfigContext),
		otattribute.Bool("kube.in-cluster", o.InCluster),
	}
}
