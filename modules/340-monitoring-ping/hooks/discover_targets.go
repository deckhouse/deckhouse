package hooks

import (
	"encoding/json"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// nodeTarget is a piece of configuration for ping exporter. It represents a single node instance.
type nodeTarget struct {
	Name    string `json:"name"`
	Address string `json:"ipAddress"`
}

// externalTarget is a piece of configuration for ping exporter. It represents a single site or external host.
type externalTarget struct {
	Name string `json:"name,omitempty"`
	Host string `json:"host,omitempty"`
}

type targets struct {
	Cluster  []nodeTarget     `json:"cluster_targets"`
	External []externalTarget `json:"external_targets"`
}

func newTargets() *targets {
	return &targets{
		Cluster:  make([]nodeTarget, 0),
		External: make([]externalTarget, 0),
	}
}

func getAddress(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	node := &v1.Node{}
	err := sdk.FromUnstructured(obj, node)
	if err != nil {
		return nil, err
	}

	target := nodeTarget{Name: node.Name}
	for _, address := range node.Status.Addresses {
		if address.Type == v1.NodeInternalIP {
			target.Address = address.Address
			break
		}
	}

	return target, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "addresses",
			ApiVersion: "v1",
			Kind:       "Node",
			FilterFunc: getAddress,
		},
	},
}, discoverNodes)

func discoverNodes(input *go_hook.HookInput) error {
	const (
		externalTargetsPath = "monitoringPing.externalTargets"
		internalTargetsPath = "monitoringPing.internal.targets"
	)

	combinedTargets := newTargets()

	for _, address := range input.Snapshots["addresses"] {
		convertedAddress := address.(nodeTarget)
		combinedTargets.Cluster = append(combinedTargets.Cluster, convertedAddress)
	}

	for _, target := range input.Values.Get(externalTargetsPath).Array() {
		var parsedExternalTarget externalTarget
		if err := json.Unmarshal([]byte(target.Raw), &parsedExternalTarget); err != nil {
			return err
		}
		combinedTargets.External = append(combinedTargets.External, parsedExternalTarget)
	}

	input.Values.Set(internalTargetsPath, combinedTargets)
	return nil
}
