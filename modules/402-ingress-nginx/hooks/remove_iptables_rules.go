package hooks

import (
	"fmt"
	"strings"

	"github.com/coreos/go-iptables/iptables"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterDeleteHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "controller",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "IngressNginxController",
			FilterFunc: objFilter,
		},
	},
}, removeIptablesRules)

func objFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {

	inlet, ok, err := unstructured.NestedString(obj.Object, "spec", "inlet")
	if err != nil {
		return nil, fmt.Errorf("couldn't get controllerVersion field from ingress controller %s: %w", obj.GetName(), err)
	}

	if ok && inlet == "HostWithFailover" {
		return true, nil
	}

	return nil, fmt.Errorf("dont have HostWithFailover inlet in %s", obj.GetName())
}

var (
	tableName = "nat"
	chainName = "ingress-failover"
	jumpRule  = strings.Fields("-p tcp -m multiport --dports 80,443 -m addrtype --dst-type LOCAL -j ingress-failover")
)

func removeIptablesRules(_ *go_hook.HookInput) error {
	ipt, err := iptables.NewWithProtocol(iptables.ProtocolIPv4)
	if err != nil {
		return fmt.Errorf("cannot connect to iptables: %w", err)
	}

	_ = ipt.DeleteIfExists(tableName, "PREROUTING", jumpRule...)
	exists, er := ipt.Exists(tableName, chainName)
	if er != nil {
		return fmt.Errorf("cannot check if %s exists: %w", chainName, err)
	}
	if exists {
		if err = ipt.ClearAndDeleteChain(tableName, chainName); err != nil {
			return err
		}
	}

	return nil
}
