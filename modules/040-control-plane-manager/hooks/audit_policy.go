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

package hooks

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	apiv1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/runtime/serializer/versioning"
	audit "k8s.io/apiserver/pkg/apis/audit/v1"
	"sigs.k8s.io/yaml"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        moduleQueue,
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "kube_audit_policy_secret",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"audit-policy"},
			},
			FilterFunc: filterAuditSecret,
		},
		{
			Name:       "configmaps_with_extra_audit_policy",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			LabelSelector: &apiv1.LabelSelector{
				MatchLabels: map[string]string{
					"control-plane-manager.deckhouse.io/extra-audit-policy-config": "",
				},
			},
			FilterFunc: filterConfigMap,
		},
	},
}, handleAuditPolicy)

func filterAuditSecret(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var sec v1.Secret

	err := sdk.FromUnstructured(unstructured, &sec)
	if err != nil {
		return nil, err
	}

	data := sec.Data["audit-policy.yaml"]

	return data, nil
}

func filterConfigMap(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var cm v1.ConfigMap

	err := sdk.FromUnstructured(unstructured, &cm)
	if err != nil {
		return nil, err
	}

	yamlData := struct {
		ServiceAccounts []string `yaml:"serviceAccounts"`
	}{ServiceAccounts: make([]string, 0)}

	if data, ok := cm.Data["basicAuditPolicy"]; ok {
		err = yaml.Unmarshal([]byte(data), &yamlData)
		if err != nil {
			return nil, fmt.Errorf("invalid basicAuditPolicy format - yaml expected: %s", err)
		}
	}

	return ConfigMapInfo{
		ServiceAccounts: yamlData.ServiceAccounts,
	}, nil
}

func handleAuditPolicy(input *go_hook.HookInput) error {
	var policy audit.Policy

	if input.Values.Get("controlPlaneManager.apiserver.basicAuditPolicyEnabled").Bool() {
		appendBasicPolicyRules(&policy, input.Snapshots["configmaps_with_extra_audit_policy"])
	}

	snap := input.Snapshots["kube_audit_policy_secret"]
	if input.Values.Get("controlPlaneManager.apiserver.auditPolicyEnabled").Bool() && len(snap) > 0 {
		data := snap[0].([]byte)
		err := appendAdditionalPolicyRules(&policy, &data)
		if err != nil {
			return err
		}
	}

	if len(policy.Rules) == 0 {
		input.Values.Remove("controlPlaneManager.internal.auditPolicy")
		return nil
	}

	data, err := serializePolicy(&policy)
	if err != nil {
		return err
	}
	input.Values.Set("controlPlaneManager.internal.auditPolicy", data)
	return nil
}

func appendBasicPolicyRules(policy *audit.Policy, extraData []go_hook.FilterResult) {
	var appendDropResourcesRule = func(resource audit.GroupResources) {
		rule := audit.PolicyRule{
			Level: audit.LevelNone,
			Resources: []audit.GroupResources{
				resource,
			},
		}
		policy.Rules = append(policy.Rules, rule)
	}

	// Drop events on endpoints, endpointslices and events resources.
	appendDropResourcesRule(audit.GroupResources{
		Group:     "",
		Resources: []string{"endpoints", "endpointslices", "events"},
	})
	// Drop leader elections based on leases resource.
	appendDropResourcesRule(audit.GroupResources{
		Group:     "coordination.k8s.io",
		Resources: []string{"leases"},
	})
	// Drop cert-manager's leader elections based on configmap resources.
	appendDropResourcesRule(audit.GroupResources{
		Group:         "",
		Resources:     []string{"configmaps"},
		ResourceNames: []string{"cert-manager-cainjector-leader-election", "cert-manager-controller"},
	})
	// Drop verticalpodautoscalercheckpoints.
	appendDropResourcesRule(audit.GroupResources{
		Group:     "autoscaling.k8s.io",
		Resources: []string{"verticalpodautoscalercheckpoints"},
	})
	// Drop patches of verticalpodautoscalers by recommender.
	{
		rule := audit.PolicyRule{
			Level: audit.LevelNone,
			Verbs: []string{"patch"},
			Users: []string{"system:serviceaccount:kube-system:d8-vertical-pod-autoscaler-recommender"},
			Resources: []audit.GroupResources{
				{
					Group:     "autoscaling.k8s.io",
					Resources: []string{"verticalpodautoscalers"},
				},
			},
		}
		policy.Rules = append(policy.Rules, rule)
	}
	// Drop upmeterhookprobes.
	appendDropResourcesRule(audit.GroupResources{
		Group:     "deckhouse.io",
		Resources: []string{"upmeterhookprobes"},
	})
	// Drop everything related to d8-upmeter namespace.
	{
		rule := audit.PolicyRule{
			Level:      audit.LevelNone,
			Namespaces: []string{"d8-upmeter"},
		}
		policy.Rules = append(policy.Rules, rule)
	}
	// Drop ingress nginx leader elections based on configmaps.
	{
		rule := audit.PolicyRule{
			Level:      audit.LevelNone,
			Verbs:      []string{"update"},
			Users:      []string{"system:serviceaccount:d8-ingress-nginx:ingress-nginx"},
			Namespaces: []string{"d8-ingress-nginx"},
			Resources: []audit.GroupResources{
				{
					Group:     "",
					Resources: []string{"configmaps"},
				},
			},
		}
		policy.Rules = append(policy.Rules, rule)
	}
	// Drop authrequests created by dex health-check probe.
	{
		rule := audit.PolicyRule{
			Level:      audit.LevelNone,
			Verbs:      []string{"create", "delete"},
			Users:      []string{"system:serviceaccount:d8-user-authn:dex"},
			Namespaces: []string{"d8-user-authn"},
			Resources: []audit.GroupResources{
				{
					Group:     "dex.coreos.com",
					Resources: []string{"authrequests"},
				},
			},
		}
		policy.Rules = append(policy.Rules, rule)
	}

	// A rule collecting logs about actions of service accounts from system namespaces.
	{
		rule := audit.PolicyRule{
			Level:      audit.LevelMetadata,
			Verbs:      []string{"create", "update", "patch", "delete"},
			Users:      auditPolicyBasicServiceAccounts,
			UserGroups: []string{"system:serviceaccounts"},
			OmitStages: []audit.Stage{
				audit.StageRequestReceived,
			},
		}

		// Append sa from extra ConfigMaps
		if len(extraData) > 0 {
			users := rule.Users
			for _, cmSnap := range extraData {
				configMap := cmSnap.(ConfigMapInfo)
				users = append(users, configMap.ServiceAccounts...)
			}
			rule.Users = users
		}

		policy.Rules = append(policy.Rules, rule)
	}
	// A rule collecting logs about actions taken on the resources in system namespaces.
	{
		rule := audit.PolicyRule{
			Level:      audit.LevelMetadata,
			Verbs:      []string{"create", "update", "patch", "delete"},
			Namespaces: auditPolicyBasicNamespaces,
			OmitStages: []audit.Stage{
				audit.StageRequestReceived,
			},
		}
		policy.Rules = append(policy.Rules, rule)
	}
	// Collect all LIST operations for all namespaces since they consume a lot of
	// apiserver memory sometimes and are a mean to debug OOMs.
	{
		rule := audit.PolicyRule{
			Level:      audit.LevelMetadata,
			Verbs:      []string{"list"},
			Namespaces: []string{}, // every namespace
			// no stage omitted, since apiserver might crash with OOM before it responds, and we want to catch it
		}
		policy.Rules = append(policy.Rules, rule)
	}
}

func appendAdditionalPolicyRules(policy *audit.Policy, data *[]byte) error {
	var p audit.Policy
	err := yaml.UnmarshalStrict(*data, &p)
	if err != nil {
		return fmt.Errorf("invalid audit-policy.yaml format: %s", err)
	}

	policy.OmitStages = append(policy.OmitStages, p.OmitStages...)
	policy.Rules = append(policy.Rules, p.Rules...)

	return nil
}

func serializePolicy(policy *audit.Policy) (string, error) {
	schema := runtime.NewScheme()
	builder := runtime.SchemeBuilder{
		audit.AddToScheme,
	}
	err := builder.AddToScheme(schema)
	if err != nil {
		return "", err
	}
	serializer := json.NewSerializerWithOptions(
		json.DefaultMetaFactory, schema, schema,
		json.SerializerOptions{
			Yaml:   true,
			Pretty: true,
			Strict: true,
		},
	)
	buf := bytes.NewBuffer(nil)
	versioningCodec := versioning.NewDefaultingCodecForScheme(schema, serializer, nil, nil, nil)
	err = versioningCodec.Encode(policy, buf)
	if err != nil {
		return "", fmt.Errorf("invalid final Policy format: %s", err)
	}

	data := strings.Replace(buf.String(), "metadata:\n  creationTimestamp: null\n", "", 1)
	return base64.StdEncoding.EncodeToString([]byte(data)), nil
}

type ConfigMapInfo struct {
	ServiceAccounts []string
}
