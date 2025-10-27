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
