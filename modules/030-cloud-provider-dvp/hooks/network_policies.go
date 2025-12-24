package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"dvp-common/config"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	templatesConfigMapName = "dvp-project-networkpolicies-templates"

	projectAPIVersion = "deckhouse.io/v1alpha2"
	projectKind       = "Project"

	npTypeIsolated      = "Isolated"
	npTypeNotRestricted = "NotRestricted"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/cloud-provider-dvp/ensure_project_network_policies_external",
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "sync-project-network-policies",
			Crontab: "* * * * *",
		},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "np_templates_cm",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-provider-dvp"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{templatesConfigMapName},
			},
			FilterFunc: filterTemplatesConfigMap,
		},
	},
}, ensureProjectNetworkPoliciesExternal)

func filterTemplatesConfigMap(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	cm := &corev1.ConfigMap{}
	if err := sdk.FromUnstructured(obj, cm); err != nil {
		return nil, fmt.Errorf("failed to convert ConfigMap: %w", err)
	}
	return cm, nil
}

func ensureProjectNetworkPoliciesExternal(ctx context.Context, input *go_hook.HookInput) error {
	cm, ok, err := getTemplatesConfigMap(input)
	if err != nil {
		return err
	}
	if !ok {
		input.Logger.Warn("networkpolicy templates ConfigMap not found, skipping", "configMap", templatesConfigMapName)
		return nil
	}

	cloudCfg, err := config.NewCloudConfig()
	if err != nil {
		return fmt.Errorf("failed to init dvp cloud config: %w", err)
	}
	projectName := cloudCfg.Namespace
	projectNamespace := cloudCfg.Namespace
	if projectName == "" {
		return fmt.Errorf("DVP_NAMESPACE is empty")
	}

	extClient, err := newExternalClient(cloudCfg)
	if err != nil {
		return err
	}

	npType, err := getProjectNetworkPolicyType(ctx, extClient, projectName)
	if err != nil {
		return err
	}
	if npType == "" {
		input.Logger.Debug("Project has empty spec.parameters.networkPolicy, skipping", "project", projectName)
		return nil
	}

	if err := ensureNamespaceExists(ctx, extClient, projectNamespace); err != nil {
		if err == client.IgnoreNotFound(err) {
			input.Logger.Debug("external namespace not found yet, skipping", "namespace", projectNamespace)
			return nil
		}
		return err
	}

	templateNP, err := getTemplateNetworkPolicy(cm, npType, projectNamespace)
	if err != nil {
		return err
	}

	currentNP, err := getCurrentNetworkPolicy(ctx, extClient, projectNamespace, templateNP.Name)
	if err != nil {
		if client.IgnoreNotFound(err) == nil {
			input.Logger.Debug("target NetworkPolicy not found yet, skipping", "namespace", projectNamespace, "name", templateNP.Name)
			return nil
		}
		return err
	}

	mergedIngress, mergedEgress, addedIngress, addedEgress := mergeMissingRulesAddOnly(currentNP, templateNP)
	if addedIngress == 0 && addedEgress == 0 {
		input.Logger.Debug("no missing rules, nothing to patch",
			"project", projectName,
			"namespace", projectNamespace,
			"policyType", npType,
			"networkPolicy", templateNP.Name,
		)
		return nil
	}

	if err := patchNetworkPolicy(ctx, extClient, projectNamespace, templateNP.Name, currentNP, mergedIngress, mergedEgress); err != nil {
		return err
	}

	input.Logger.Info("NetworkPolicy patched with missing rules from template",
		"project", projectName,
		"namespace", projectNamespace,
		"policyType", npType,
		"networkPolicy", templateNP.Name,
		"addedIngressRules", addedIngress,
		"addedEgressRules", addedEgress,
	)

	return nil
}

func getTemplatesConfigMap(input *go_hook.HookInput) (*corev1.ConfigMap, bool, error) {
	snaps := input.Snapshots.Get("np_templates_cm")
	if len(snaps) == 0 {
		return nil, false, nil
	}
	cm := &corev1.ConfigMap{}
	if err := snaps[0].UnmarshalTo(cm); err != nil {
		return nil, false, fmt.Errorf("failed to unmarshal templates ConfigMap snapshot: %w", err)
	}
	return cm, true, nil
}

func newExternalClient(cloudCfg *config.CloudConfig) (client.Client, error) {
	restCfg, err := cloudCfg.GetKubernetesClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get external kube client config: %w", err)
	}

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("failed to add corev1 scheme: %w", err)
	}
	if err := networkingv1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("failed to add networkingv1 scheme: %w", err)
	}

	extClient, err := client.New(restCfg, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("failed to create external kube client: %w", err)
	}
	return extClient, nil
}

func getProjectNetworkPolicyType(ctx context.Context, extClient client.Client, projectName string) (string, error) {
	projectObj := &unstructured.Unstructured{}
	projectObj.SetAPIVersion(projectAPIVersion)
	projectObj.SetKind(projectKind)

	if err := extClient.Get(ctx, client.ObjectKey{Name: projectName}, projectObj); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return "", nil
		}
		return "", fmt.Errorf("failed to get external Project %q: %w", projectName, err)
	}

	npType, _, err := unstructured.NestedString(projectObj.Object, "spec", "parameters", "networkPolicy")
	if err != nil {
		return "", fmt.Errorf("failed to read Project.spec.parameters.networkPolicy: %w", err)
	}

	if npType != "" && npType != npTypeIsolated && npType != npTypeNotRestricted {
		return "", fmt.Errorf("unsupported Project.spec.parameters.networkPolicy %q (expected %q or %q)", npType, npTypeIsolated, npTypeNotRestricted)
	}

	return npType, nil
}

func ensureNamespaceExists(ctx context.Context, extClient client.Client, namespace string) error {
	ns := &corev1.Namespace{}
	return extClient.Get(ctx, client.ObjectKey{Name: namespace}, ns)
}

func getTemplateNetworkPolicy(cm *corev1.ConfigMap, npType, projectNamespace string) (*networkingv1.NetworkPolicy, error) {
	templateKey := ""
	switch npType {
	case npTypeIsolated:
		templateKey = "isolated.yaml"
	case npTypeNotRestricted:
		templateKey = "notrestricted.yaml"
	default:
		return nil, fmt.Errorf("unsupported network policy type %q", npType)
	}

	templateRaw, ok := cm.Data[templateKey]
	if !ok {
		return nil, fmt.Errorf("template %q not found in ConfigMap %q", templateKey, templatesConfigMapName)
	}
	templateRaw = strings.ReplaceAll(templateRaw, "__PROJECT_NAMESPACE__", projectNamespace)

	templateNP := &networkingv1.NetworkPolicy{}
	decoder := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(templateRaw)), 4096)
	if err := decoder.Decode(templateNP); err != nil {
		return nil, fmt.Errorf("failed to decode template %q: %w", templateKey, err)
	}
	if templateNP.Name == "" {
		return nil, fmt.Errorf("template %q: metadata.name is required", templateKey)
	}
	return templateNP, nil
}

func getCurrentNetworkPolicy(ctx context.Context, extClient client.Client, namespace, name string) (*networkingv1.NetworkPolicy, error) {
	current := &networkingv1.NetworkPolicy{}
	if err := extClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, current); err != nil {
		return nil, fmt.Errorf("failed to get NetworkPolicy %q/%q: %w", namespace, name, err)
	}
	return current, nil
}

func mergeMissingRulesAddOnly(
	current *networkingv1.NetworkPolicy,
	template *networkingv1.NetworkPolicy,
) ([]networkingv1.NetworkPolicyIngressRule, []networkingv1.NetworkPolicyEgressRule, int, int) {
	existingIngress := make(map[string]struct{}, len(current.Spec.Ingress))
	existingEgress := make(map[string]struct{}, len(current.Spec.Egress))

	for i := range current.Spec.Ingress {
		r := current.Spec.Ingress[i]
		k := ingressRuleKey(r)
		existingIngress[k] = struct{}{}
	}

	for i := range current.Spec.Egress {
		r := current.Spec.Egress[i]
		k := egressRuleKey(r)
		existingEgress[k] = struct{}{}
	}

	mergedIngress := make([]networkingv1.NetworkPolicyIngressRule, 0, len(current.Spec.Ingress)+len(template.Spec.Ingress))
	mergedIngress = append(mergedIngress, current.Spec.Ingress...)
	addedIngress := 0

	for i := range template.Spec.Ingress {
		r := template.Spec.Ingress[i]
		k := ingressRuleKey(r)
		if _, ok := existingIngress[k]; ok {
			continue
		}
		mergedIngress = append(mergedIngress, normalizeIngressRule(r))
		existingIngress[k] = struct{}{}
		addedIngress++
	}

	mergedEgress := make([]networkingv1.NetworkPolicyEgressRule, 0, len(current.Spec.Egress)+len(template.Spec.Egress))
	mergedEgress = append(mergedEgress, current.Spec.Egress...)
	addedEgress := 0

	for i := range template.Spec.Egress {
		r := template.Spec.Egress[i]
		k := egressRuleKey(r)
		if _, ok := existingEgress[k]; ok {
			continue
		}
		mergedEgress = append(mergedEgress, normalizeEgressRule(r))
		existingEgress[k] = struct{}{}
		addedEgress++
	}

	return mergedIngress, mergedEgress, addedIngress, addedEgress
}

func patchNetworkPolicy(
	ctx context.Context,
	extClient client.Client,
	namespace string,
	name string,
	current *networkingv1.NetworkPolicy,
	ingress []networkingv1.NetworkPolicyIngressRule,
	egress []networkingv1.NetworkPolicyEgressRule,
) error {
	target := &networkingv1.NetworkPolicy{}
	target.Namespace = namespace
	target.Name = name

	_, err := controllerutil.CreateOrUpdate(ctx, extClient, target, func() error {
		target.Spec.PodSelector = current.Spec.PodSelector
		target.Spec.PolicyTypes = current.Spec.PolicyTypes
		target.Spec.Ingress = ingress
		target.Spec.Egress = egress
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to patch NetworkPolicy %q/%q: %w", namespace, name, err)
	}
	return nil
}

func normalizeIngressRule(r networkingv1.NetworkPolicyIngressRule) networkingv1.NetworkPolicyIngressRule {
	cp := r.DeepCopy()
	if len(cp.From) > 1 {
		sort.SliceStable(cp.From, func(i, j int) bool { return peerKey(cp.From[i]) < peerKey(cp.From[j]) })
	}
	if len(cp.Ports) > 1 {
		sort.SliceStable(cp.Ports, func(i, j int) bool { return portKey(cp.Ports[i]) < portKey(cp.Ports[j]) })
	}
	return *cp
}

func normalizeEgressRule(r networkingv1.NetworkPolicyEgressRule) networkingv1.NetworkPolicyEgressRule {
	cp := r.DeepCopy()
	if len(cp.To) > 1 {
		sort.SliceStable(cp.To, func(i, j int) bool { return peerKey(cp.To[i]) < peerKey(cp.To[j]) })
	}
	if len(cp.Ports) > 1 {
		sort.SliceStable(cp.Ports, func(i, j int) bool { return portKey(cp.Ports[i]) < portKey(cp.Ports[j]) })
	}
	return *cp
}

func ingressRuleKey(r networkingv1.NetworkPolicyIngressRule) string {
	n := normalizeIngressRule(r)
	b, _ := json.Marshal(n)
	return string(b)
}

func egressRuleKey(r networkingv1.NetworkPolicyEgressRule) string {
	n := normalizeEgressRule(r)
	b, _ := json.Marshal(n)
	return string(b)
}

func peerKey(p networkingv1.NetworkPolicyPeer) string {
	cp := p.DeepCopy()
	if cp.IPBlock != nil && len(cp.IPBlock.Except) > 1 {
		sort.Strings(cp.IPBlock.Except)
	}
	b, _ := json.Marshal(cp)
	return string(b)
}

func portKey(p networkingv1.NetworkPolicyPort) string {
	cp := p.DeepCopy()
	b, _ := json.Marshal(cp)
	return string(b)
}
