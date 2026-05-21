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
	"context"
	"encoding/base64"
	"errors"
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

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
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

func handleAuditPolicy(_ context.Context, input *go_hook.HookInput) error {
	var policy audit.Policy

	// Start with adding basic policies.
	if input.Values.Get("controlPlaneManager.apiserver.basicAuditPolicyEnabled").Bool() {
		extraData, err := sdkobjectpatch.UnmarshalToStruct[ConfigMapInfo](input.Snapshots, "configmaps_with_extra_audit_policy")
		if err != nil {
			return fmt.Errorf("failed to unmarshal configmaps_with_extra_audit_policy snapshot: %w", err)
		}
		appendBasicPolicyRules(&policy, extraData, nil)
		// Add policies for virtualization module.
		appendVirtualizationPolicyRules(&policy, nil)
	}

	// Append custom policies if secret is present.
	auditPolicyDataSnaps, err := sdkobjectpatch.UnmarshalToStruct[[]byte](input.Snapshots, "kube_audit_policy_secret")
	if err != nil {
		return fmt.Errorf("failed to unmarshal kube_audit_policy_secret snapshot: %w", err)
	}
	if input.Values.Get("controlPlaneManager.apiserver.auditPolicyEnabled").Bool() && len(auditPolicyDataSnaps) > 0 {
		auditPolicyData := auditPolicyDataSnaps[0]
		err := appendAdditionalPolicyRules(&policy, &auditPolicyData)
		if err != nil {
			return err
		}
	}
	// Unauthenticated requests are taken by directing all Metadata level requests with `UserGroups` with `system:authenticated` to None and then taking all remaining Metadata level logs
	// There should always be a last rule
	if input.Values.Get("controlPlaneManager.apiserver.basicAuditPolicyEnabled").Bool() {
		appendUnauthenticatedRules(&policy, nil)
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

func appendBasicPolicyRules(policy *audit.Policy, extraData []ConfigMapInfo, docs *[]AuditPolicyRuleWithDescription) {
	appendDropResourcesRule := func(resource audit.GroupResources, descriptionEN, descriptionRU string) {
		description_en := descriptionEN
		description_ru := descriptionRU
		rule := audit.PolicyRule{
			Level: audit.LevelNone,
			Resources: []audit.GroupResources{
				resource,
			},
		}
		appendPolicyRuleWithDescription(policy, docs, AuditPolicySectionBasic, description_en, description_ru, rule)
	}

	appendDropResourcesRule(
		audit.GroupResources{Group: "", Resources: []string{"endpoints", "endpointslices", "events"}},
		"Do not log frequent updates for Endpoints, EndpointSlices, and Events.",
		"Не логировать частые обновления Endpoints, EndpointSlice и Event.",
	)
	appendDropResourcesRule(
		audit.GroupResources{Group: "coordination.k8s.io", Resources: []string{"leases"}},
		"Do not log leader election operations on Lease resources.",
		"Не логировать операции выбора лидера на ресурсах Lease.",
	)
	appendDropResourcesRule(
		audit.GroupResources{Group: "", Resources: []string{"configmaps"}, ResourceNames: []string{"cert-manager-cainjector-leader-election", "cert-manager-controller"}},
		"Do not log cert-manager leader election ConfigMaps.",
		"Не логировать ConfigMap cert-manager, используемые для выбора лидера.",
	)
	appendDropResourcesRule(
		audit.GroupResources{Group: "autoscaling.k8s.io", Resources: []string{"verticalpodautoscalercheckpoints"}},
		"Do not log VerticalPodAutoscalerCheckpoints resources.",
		"Не логировать ресурсы VerticalPodAutoscalerCheckpoints.",
	)

	{
		description_en := "Do not log PATCH operations on VerticalPodAutoscaler from recommender."
		description_ru := "Не логировать PATCH операций VerticalPodAutoscaler от recommender."
		rule := audit.PolicyRule{
			Level: audit.LevelNone,
			Verbs: []string{"patch"},
			Users: []string{"system:serviceaccount:kube-system:d8-vertical-pod-autoscaler-recommender"},
			Resources: []audit.GroupResources{{
				Group:     "autoscaling.k8s.io",
				Resources: []string{"verticalpodautoscalers"},
			}},
		}
		appendPolicyRuleWithDescription(policy, docs, AuditPolicySectionBasic, description_en, description_ru, rule)
	}

	appendDropResourcesRule(
		audit.GroupResources{Group: "deckhouse.io", Resources: []string{"upmeterhookprobes"}},
		"Do not log UpmeterHookProbes resources.",
		"Не логировать ресурсы UpmeterHookProbes.",
	)

	{
		description_en := "Do not log any operations in d8-upmeter namespace."
		description_ru := "Не логировать любые операции в пространстве имён d8-upmeter."
		rule := audit.PolicyRule{Level: audit.LevelNone, Namespaces: []string{"d8-upmeter"}}
		appendPolicyRuleWithDescription(policy, docs, AuditPolicySectionBasic, description_en, description_ru, rule)
	}

	{
		description_en := "Do not log ingress-nginx leader election updates in ConfigMaps."
		description_ru := "Не логировать обновления ConfigMap ingress-nginx для выборов лидера."
		rule := audit.PolicyRule{
			Level:      audit.LevelNone,
			Verbs:      []string{"update"},
			Users:      []string{"system:serviceaccount:d8-ingress-nginx:ingress-nginx"},
			Namespaces: []string{"d8-ingress-nginx"},
			Resources: []audit.GroupResources{{
				Group:     "",
				Resources: []string{"configmaps"},
			}},
		}
		appendPolicyRuleWithDescription(policy, docs, AuditPolicySectionBasic, description_en, description_ru, rule)
	}

	{
		description_en := "Do not log dex health-check create/delete operations on AuthRequest resources."
		description_ru := "Не логировать операции create/delete AuthRequest от health-check dex."
		rule := audit.PolicyRule{
			Level:      audit.LevelNone,
			Verbs:      []string{"create", "delete"},
			Users:      []string{"system:serviceaccount:d8-user-authn:dex"},
			Namespaces: []string{"d8-user-authn"},
			Resources: []audit.GroupResources{{
				Group:     "dex.coreos.com",
				Resources: []string{"authrequests"},
			}},
		}
		appendPolicyRuleWithDescription(policy, docs, AuditPolicySectionBasic, description_en, description_ru, rule)
	}

	{
		description_en := "Log create and delete operations for Node resources with request/response payload."
		description_ru := "Логировать операции create/delete для ресурсов Node с телом запроса/ответа."
		rule := audit.PolicyRule{
			Level: audit.LevelRequestResponse,
			Verbs: []string{"create", "delete"},
			Resources: []audit.GroupResources{{
				Group:     "",
				Resources: []string{"nodes"},
			}},
		}
		appendPolicyRuleWithDescription(policy, docs, AuditPolicySectionBasic, description_en, description_ru, rule)
	}

	{
		description_en := "Log kubectl logs requests (pods/log) at Metadata level."
		description_ru := "Логировать запросы kubectl logs (pods/log) на уровне Metadata."
		rule := audit.PolicyRule{
			Level: audit.LevelMetadata,
			Resources: []audit.GroupResources{{
				Group:     "",
				Resources: []string{"pods/log"},
			}},
		}
		appendPolicyRuleWithDescription(policy, docs, AuditPolicySectionBasic, description_en, description_ru, rule)
	}

	{
		description_en := "Log create/update/patch/delete operations from system service accounts (kube-system, d8-*)."
		description_ru := "Логировать операции create/update/patch/delete от системных ServiceAccount (kube-system, d8-*)."
		rule := audit.PolicyRule{
			Level:      audit.LevelMetadata,
			Verbs:      []string{"create", "update", "patch", "delete"},
			Users:      auditPolicyBasicServiceAccounts,
			UserGroups: []string{"system:serviceaccounts"},
			OmitStages: []audit.Stage{
				audit.StageRequestReceived,
			},
		}

		if len(extraData) > 0 {
			users := rule.Users
			for _, configMap := range extraData {
				users = append(users, configMap.ServiceAccounts...)
			}
			rule.Users = users
		}

		appendPolicyRuleWithDescription(policy, docs, AuditPolicySectionBasic, description_en, description_ru, rule)
	}

	{
		description_en := "Log create/update/patch/delete operations for Pod resources."
		description_ru := "Логировать операции create/update/patch/delete для ресурсов Pod."
		rule := audit.PolicyRule{
			Level: audit.LevelRequest,
			Resources: []audit.GroupResources{{
				Resources: []string{"pods"},
			}},
			Verbs: []string{"create", "delete", "patch", "update"},
			OmitStages: []audit.Stage{
				audit.StageRequestReceived,
			},
		}
		appendPolicyRuleWithDescription(policy, docs, AuditPolicySectionBasic, description_en, description_ru, rule)
	}

	{
		description_en := "Log create/update/patch/delete operations in system namespaces (kube-system, d8-*)."
		description_ru := "Логировать операции create/update/patch/delete в системных пространствах имён (kube-system, d8-*)."
		rule := audit.PolicyRule{
			Level:      audit.LevelMetadata,
			Verbs:      []string{"create", "update", "patch", "delete"},
			Namespaces: auditPolicyBasicNamespaces,
			OmitStages: []audit.Stage{
				audit.StageRequestReceived,
			},
		}
		appendPolicyRuleWithDescription(policy, docs, AuditPolicySectionBasic, description_en, description_ru, rule)
	}

	{
		description_en := "Log all LIST operations in all namespaces."
		description_ru := "Логировать все LIST-запросы во всех пространствах имён."
		rule := audit.PolicyRule{
			Level:      audit.LevelMetadata,
			Verbs:      []string{"list"},
			Namespaces: []string{},
		}
		appendPolicyRuleWithDescription(policy, docs, AuditPolicySectionBasic, description_en, description_ru, rule)
	}

	{
		description_en := "Log create and delete operations for ServiceAccount resources."
		description_ru := "Логировать операции create/delete для ресурсов ServiceAccount."
		rule := audit.PolicyRule{
			Level: audit.LevelMetadata,
			Resources: []audit.GroupResources{{
				Group:     "",
				Resources: []string{"serviceaccounts"},
			}},
			Verbs: []string{"create", "delete"},
			OmitStages: []audit.Stage{
				audit.StageRequestReceived,
			},
		}
		appendPolicyRuleWithDescription(policy, docs, AuditPolicySectionBasic, description_en, description_ru, rule)
	}

	{
		description_en := "Log create/update/delete/patch operations for Role and ClusterRole resources."
		description_ru := "Логировать операции create/update/delete/patch для ресурсов Role и ClusterRole."
		rule := audit.PolicyRule{
			Level: audit.LevelRequest,
			Resources: []audit.GroupResources{{
				Group:     "rbac.authorization.k8s.io",
				Resources: []string{"roles", "clusterroles"},
			}},
			Verbs: []string{"create", "update", "delete", "patch"},
			OmitStages: []audit.Stage{
				audit.StageRequestReceived,
			},
		}
		appendPolicyRuleWithDescription(policy, docs, AuditPolicySectionBasic, description_en, description_ru, rule)
	}

	{
		description_en := "Log create/update/delete operations for ClusterRoleBinding resources."
		description_ru := "Логировать операции create/update/delete для ресурсов ClusterRoleBinding."
		rule := audit.PolicyRule{
			Level: audit.LevelRequest,
			Resources: []audit.GroupResources{{
				Group:     "rbac.authorization.k8s.io",
				Resources: []string{"clusterrolebindings"},
			}},
			Verbs: []string{"create", "update", "delete"},
			OmitStages: []audit.Stage{
				audit.StageRequestReceived,
			},
		}
		appendPolicyRuleWithDescription(policy, docs, AuditPolicySectionBasic, description_en, description_ru, rule)
	}

	{
		description_en := "Log attach and ephemeral container related pod subresource operations."
		description_ru := "Логировать операции с pod subresource для attach и ephemeral-контейнеров."
		rule := audit.PolicyRule{
			Level: audit.LevelRequest,
			Resources: []audit.GroupResources{{
				Resources: []string{"pods/attach", "pods/ephemeralcontainers"},
			}},
			Verbs: []string{"get", "patch", "create"},
			OmitStages: []audit.Stage{
				audit.StageRequestReceived,
			},
		}
		appendPolicyRuleWithDescription(policy, docs, AuditPolicySectionBasic, description_en, description_ru, rule)
	}

	// Capture kubectl get logs requests.
	{
		rule := audit.PolicyRule{
			Level: audit.LevelRequest,
			Resources: []audit.GroupResources{
				{
					Resources: []string{"pods/log"},
				},
			},
			Verbs: []string{"get"},
			OmitStages: []audit.Stage{
				audit.StageRequestReceived,
			},
		}
		policy.Rules = append(policy.Rules, rule)
	}
}

func appendVirtualizationPolicyRules(policy *audit.Policy, docs *[]AuditPolicyRuleWithDescription) {
	{
		description_en := "Log creation of VirtualMachineOperation resources with request/response payload."
		description_ru := "Логировать создание ресурсов VirtualMachineOperation с телом запроса/ответа."
		rule := audit.PolicyRule{
			Level: audit.LevelRequestResponse,
			Verbs: []string{"create"},
			Resources: []audit.GroupResources{{
				Group:     "virtualization.deckhouse.io",
				Resources: []string{"virtualmachineoperations"},
			}},
		}
		appendPolicyRuleWithDescription(policy, docs, AuditPolicySectionVirtualization, description_en, description_ru, rule)
	}
	{
		description_en := "Log create/update/patch/delete operations for virtualization.deckhouse.io resources."
		description_ru := "Логировать операции create/update/patch/delete для ресурсов virtualization.deckhouse.io."
		rule := audit.PolicyRule{
			Level:     audit.LevelMetadata,
			Verbs:     []string{"create", "update", "patch", "delete"},
			Resources: []audit.GroupResources{{Group: "virtualization.deckhouse.io"}},
		}
		appendPolicyRuleWithDescription(policy, docs, AuditPolicySectionVirtualization, description_en, description_ru, rule)
	}
	{
		description_en := "Log update/patch operations for internal virtualization subresources."
		description_ru := "Логировать операции update/patch для внутренних virtualization subresources."
		rule := audit.PolicyRule{
			Level: audit.LevelMetadata,
			Verbs: []string{"update", "patch"},
			Resources: []audit.GroupResources{{
				Group:     "internal.virtualization.deckhouse.io",
				Resources: []string{"internalvirtualizationvirtualmachineinstances"},
			}},
		}
		appendPolicyRuleWithDescription(policy, docs, AuditPolicySectionVirtualization, description_en, description_ru, rule)
	}
	{
		description_en := "Log GET operations for subresources.virtualization.deckhouse.io API group."
		description_ru := "Логировать GET-операции для API-группы subresources.virtualization.deckhouse.io."
		rule := audit.PolicyRule{
			Level:     audit.LevelMetadata,
			Verbs:     []string{"get"},
			Resources: []audit.GroupResources{{Group: "subresources.virtualization.deckhouse.io"}},
		}
		appendPolicyRuleWithDescription(policy, docs, AuditPolicySectionVirtualization, description_en, description_ru, rule)
	}
	{
		description_en := "Log create/update/patch/delete operations for Pod resources."
		description_ru := "Логировать операции create/update/patch/delete для ресурсов Pod."
		rule := audit.PolicyRule{
			Level:     audit.LevelMetadata,
			Verbs:     []string{"create", "update", "patch", "delete"},
			Resources: []audit.GroupResources{{Group: "", Resources: []string{"pods"}}},
		}
		appendPolicyRuleWithDescription(policy, docs, AuditPolicySectionVirtualization, description_en, description_ru, rule)
	}
	{
		description_en := "Log create/update/patch/delete operations in d8-virtualization namespace."
		description_ru := "Логировать операции create/update/patch/delete в пространстве имён d8-virtualization."
		rule := audit.PolicyRule{
			Level:      audit.LevelMetadata,
			Verbs:      []string{"create", "update", "patch", "delete"},
			Namespaces: []string{"d8-virtualization"},
		}
		appendPolicyRuleWithDescription(policy, docs, AuditPolicySectionVirtualization, description_en, description_ru, rule)
	}
	{
		description_en := "Log create/update/patch/delete operations for ModuleConfig resources."
		description_ru := "Логировать операции create/update/patch/delete для ресурсов ModuleConfig."
		rule := audit.PolicyRule{
			Level: audit.LevelMetadata,
			Verbs: []string{"create", "update", "patch", "delete"},
			Resources: []audit.GroupResources{{
				Group:     "deckhouse.io",
				Resources: []string{"moduleconfigs"},
			}},
		}
		appendPolicyRuleWithDescription(policy, docs, AuditPolicySectionVirtualization, description_en, description_ru, rule)
	}
}

func appendUnauthenticatedRules(policy *audit.Policy, docs *[]AuditPolicyRuleWithDescription) {
	{
		description_en := "Do not log requests from authenticated users."
		description_ru := "Не логировать запросы аутентифицированных пользователей."
		rule := audit.PolicyRule{
			Level:      audit.LevelNone,
			UserGroups: []string{"system:authenticated"},
		}
		appendPolicyRuleWithDescription(policy, docs, AuditPolicySectionUnauthenticated, description_en, description_ru, rule)
	}
	{
		description_en := "Log all remaining (unauthenticated) requests at metadata level."
		description_ru := "Логировать все оставшиеся (неаутентифицированные) запросы на уровне metadata."
		rule := audit.PolicyRule{
			Level: audit.LevelMetadata,
		}
		appendPolicyRuleWithDescription(policy, docs, AuditPolicySectionUnauthenticated, description_en, description_ru, rule)
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

const (
	AuditPolicySectionBasic           = "basic"
	AuditPolicySectionVirtualization  = "virtualization"
	AuditPolicySectionUnauthenticated = "unauthenticated"
)

type AuditRuleDescription struct {
	EN string
	RU string
}

type AuditPolicyRuleWithDescription struct {
	Rule        audit.PolicyRule
	Section     string
	Description AuditRuleDescription
}

func BuiltInAuditPolicyRulesForDocumentation() ([]AuditPolicyRuleWithDescription, error) {
	policy := &audit.Policy{}
	result := make([]AuditPolicyRuleWithDescription, 0)

	appendBasicPolicyRules(policy, nil, &result)
	appendVirtualizationPolicyRules(policy, &result)
	appendUnauthenticatedRules(policy, &result)

	if len(result) == 0 {
		return nil, errors.New("no built-in audit policy rules generated")
	}
	if len(policy.Rules) != len(result) {
		return nil, fmt.Errorf("mismatch between built-in rules and documented rules: %d vs %d", len(policy.Rules), len(result))
	}

	return result, nil
}

func appendPolicyRuleWithDescription(policy *audit.Policy, docs *[]AuditPolicyRuleWithDescription, section, descriptionEN, descriptionRU string, rule audit.PolicyRule) {
	policy.Rules = append(policy.Rules, rule)
	if docs == nil {
		return
	}

	*docs = append(*docs, AuditPolicyRuleWithDescription{
		Rule:    rule,
		Section: section,
		Description: AuditRuleDescription{
			EN: descriptionEN,
			RU: descriptionRU,
		},
	})
}

type ConfigMapInfo struct {
	ServiceAccounts []string
}
