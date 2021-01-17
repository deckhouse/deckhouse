package hooks

import (
	"encoding/json"
	"fmt"

	"github.com/flant/addon-operator/pkg/utils"
	"github.com/flant/addon-operator/pkg/utils/values_store"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/candictl/pkg/config"
)

var _ = sdk.Register(&ClusterDiscoveryHook{})

type ClusterDiscoveryHook struct {
	sdk.CommonGoHook
}

func (c *ClusterDiscoveryHook) Metadata() sdk.HookMetadata {
	return c.CommonMetadataFromRuntime()
}

func (c *ClusterDiscoveryHook) Config() *sdk.HookConfig {
	return c.CommonGoHook.Config(&sdk.HookConfig{
		YamlConfig: `
    configVersion: v1
    kubernetes:
    - name: cluster_configuration
      group: main
      keepFullObjectsInMemory: false
      apiVersion: v1
      kind: Secret
      namespace:
        nameSelector:
          matchNames: [kube-system]
      nameSelector:
        matchNames: [d8-cluster-configuration]
      jqFilter: '.data."cluster-configuration.yaml" | @base64d'
`,
		MainHandler: c.Main,
	})
}

/**
Original shell hook:

function set_values_from_cluster_configuration_yaml() {
  cluster_configuration_json=$(echo "$1" | deckhouse-controller helper cluster-configuration | jq -r '.clusterConfiguration')

  values::set global.clusterConfiguration "$cluster_configuration_json"

  values::set global.discovery.podSubnet "$(echo "$cluster_configuration_json" | jq -r '.podSubnetCIDR')"
  values::set global.discovery.serviceSubnet "$(echo "$cluster_configuration_json" | jq -r '.serviceSubnetCIDR')"
}

function __main__() {
  if context::has snapshots.cluster_configuration.0; then
    set_values_from_cluster_configuration_yaml "$(context::get snapshots.cluster_configuration.0.filterResult)"
  else
    values::unset global.clusterConfiguration
  fi
}

*/
func (c *ClusterDiscoveryHook) Main(input *sdk.BindingInput) (*sdk.BindingOutput, error) {
	out := &sdk.BindingOutput{
		MemoryValuesPatches: &utils.ValuesPatch{
			Operations: []*utils.ValuesPatchOperation{},
		},
	}

	s, ok := input.BindingContext.Snapshots["cluster_configuration"]
	if ok && len(s) > 0 {
		var err error

		// FilterResult is a YAML encoded as a JSON string. Unmarshal it.
		configYaml, err := JSONStringToGoString(s[0].FilterResult)
		if err != nil {
			return nil, err
		}

		var metaConfig *config.MetaConfig
		metaConfig, err = config.ParseConfigFromData(configYaml)
		if err != nil {
			return nil, err
		}

		ops := []*utils.ValuesPatchOperation{}

		ops = append(ops, &utils.ValuesPatchOperation{
			Op:    "add",
			Path:  "/global/clusterConfiguration",
			Value: metaConfig.ClusterConfig,
		})

		podSubnetCIDR, ok := metaConfig.ClusterConfig["podSubnetCIDR"]
		if ok {
			ops = append(ops, &utils.ValuesPatchOperation{
				Op:    "add",
				Path:  "/global/discovery/podSubnet",
				Value: podSubnetCIDR,
			})
			//values::set global.discovery.podSubnet "$(echo "$cluster_configuration_json" | jq -r '.podSubnetCIDR')"
		} else {
			return nil, fmt.Errorf("no podSubnetCIDR field in clusterConfiguration")
		}

		serviceSubnetCIDR, ok := metaConfig.ClusterConfig["serviceSubnetCIDR"]
		if ok {
			ops = append(ops, &utils.ValuesPatchOperation{
				Op:    "add",
				Path:  "/global/discovery/serviceSubnet",
				Value: serviceSubnetCIDR,
			})
			//values::set global.discovery.serviceSubnet "$(echo "$cluster_configuration_json" | jq -r '.serviceSubnetCIDR')"
		} else {
			return nil, fmt.Errorf("no serviceSubnetCIDR field in clusterConfiguration")
		}

		clusterDomain, ok := metaConfig.ClusterConfig["clusterDomain"]
		if ok {
			ops = append(ops, &utils.ValuesPatchOperation{
				Op:    "add",
				Path:  "/global/discovery/clusterDomain",
				Value: clusterDomain,
			})
		} else {
			return nil, fmt.Errorf("no clusterDomain field in clusterConfiguration")
		}

		out.MemoryValuesPatches.Operations = ops
	} else {
		// no cluster configuration — unset global value if there is one.
		vs := values_store.NewValuesStoreFromValues(input.Values)
		if vs.Get("global.clusterConfiguration").Exists() {
			out.MemoryValuesPatches.Operations = []*utils.ValuesPatchOperation{
				{
					Op:   "remove",
					Path: "/global/clusterConfiguration",
				},
			}
		}
	}

	return out, nil
}

func JSONStringToGoString(jsonString string) (string, error) {
	var res string
	err := json.Unmarshal([]byte(jsonString), &res)
	if err != nil {
		return "", err
	}
	return res, nil
}
