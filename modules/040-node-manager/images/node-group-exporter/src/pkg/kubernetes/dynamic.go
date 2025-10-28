/*
Copyright 2025 Flant JSC

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

package kubernetes

import (
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

const (
	// NodeGroupLabelKey is the label key used by Deckhouse to identify node groups
	NodeGroupLabelKey = "node.deckhouse.io/group"

	// InformerResyncPeriod is the resync period for Kubernetes informers
	InformerResyncPeriod = 10 * time.Minute
)

// NodeGroupGVR is the GroupVersionResource for Deckhouse NodeGroup
var NodeGroupGVR = schema.GroupVersionResource{
	Group:    "deckhouse.io",
	Version:  "v1",
	Resource: "nodegroups",
}

// NewDynamicClient creates a new dynamic client
func NewDynamicClient(config *rest.Config) (dynamic.Interface, error) {
	return dynamic.NewForConfig(config)
}
