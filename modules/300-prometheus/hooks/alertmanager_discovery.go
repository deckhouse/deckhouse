package hooks

import (
	"encoding/json"

	"github.com/flant/addon-operator/pkg/utils"
	"github.com/flant/addon-operator/sdk"
)

var _ = sdk.Register(&AlertManagerDiscovery{})

type AlertManagerDiscovery struct {
	sdk.CommonGoHook
}

func (c *AlertManagerDiscovery) Metadata() sdk.HookMetadata {
	return c.CommonMetadataFromRuntime()
}

func (c *AlertManagerDiscovery) Config() *sdk.HookConfig {
	return c.CommonGoHook.Config(&sdk.HookConfig{
		YamlConfig: `
    configVersion: v1
    kubernetes:
    - name: alertmanager_services
      group: main
      keepFullObjectsInMemory: false
      apiVersion: v1
      kind: Service
      labelSelector:
        matchExpressions:
        - key: prometheus.deckhouse.io/alertmanager
          operator: Exists
      jqFilter: |
        {
          "prometheus": (.metadata.labels."prometheus.deckhouse.io/alertmanager"),
          "service":
          {
            "namespace": .metadata.namespace,
            "name": .metadata.name,
            "port": (if .spec.ports[0] then .spec.ports[0].name // .spec.ports[0].port else null end),
            "pathPrefix": (.metadata.annotations."prometheus.deckhouse.io/alertmanager-path-prefix" // "/")
          }
        }
`,
		MainHandler: c.Main,
	})
}

type AlertmanagerService struct {
	Prometheus string                  `json:"prometheus"`
	Service    AlertmanagerServiceInfo `json:"service"`
}

type AlertmanagerServiceInfo struct {
	Name       string      `json:"name"`
	Namespace  string      `json:"namespace"`
	PathPrefix string      `json:"pathPrefix"`
	Port       interface{} `json:"port"`
}

/*
Shell variant:

  function __main__() {
    alertmanagers="$(context::jq -rc '
      [.snapshots.alertmanager_services[] | .filterResult] |
      reduce .[] as $i (
        {}; .[$i.prometheus] = (.[$i.prometheus] // []) + [$i.service]
      )
    ')"
    values::set prometheus.internal.alertmanagers "${alertmanagers}"
  }
*/
func (c *AlertManagerDiscovery) Main(input *sdk.BindingInput) (*sdk.BindingOutput, error) {
	alertManagers, err := MergeAlertManagers(input)
	if err != nil {
		return &sdk.BindingOutput{Error: err}, err
	}

	return &sdk.BindingOutput{
		MemoryValuesPatches: &utils.ValuesPatch{
			Operations: []*utils.ValuesPatchOperation{
				{
					Op:    "add",
					Path:  "/prometheus/internal/alertmanagers",
					Value: alertManagers,
				},
			},
		},
	}, nil
}

// Snapshots should contain key "alertmanager_services"
// Group service objects into arrays by prometheus fields.
// in:
// {"snapshots":{"alertmanager_services":[
//   {"filterResult":{"prometheus":"prom-one", "service":{"name":"srvOne", ...}}},
//   {"filterResult":{"prometheus":"prom-one", "service":{"name":"srvTwo", ...}}},
//   {"filterResult":{"prometheus":"longterm", "service":{"name":"AnotherSrv", ...}}}
//  ]}}
// out:
// {"prom-one":[{"name":"srvOne", ...}, {"name":"srvTwo", ...}],
//  "longterm":[{"name":"AnotherSrv", ...}]
// }
func MergeAlertManagers(input *sdk.BindingInput) (interface{}, error) {
	services, ok := input.BindingContext.Snapshots["alertmanager_services"]
	if !ok {
		return struct{}{}, nil
	}
	alertmanagers := map[string][]interface{}{}
	for _, srv := range services {
		var alertmanagerService AlertmanagerService
		err := json.Unmarshal([]byte(srv.FilterResult), &alertmanagerService)
		if err != nil {
			return "", err
		}

		if _, ok := alertmanagers[alertmanagerService.Prometheus]; !ok {
			alertmanagers[alertmanagerService.Prometheus] = make([]interface{}, 0)
		}
		alertmanagers[alertmanagerService.Prometheus] = append(alertmanagers[alertmanagerService.Prometheus], alertmanagerService.Service)
	}

	return alertmanagers, nil
}
