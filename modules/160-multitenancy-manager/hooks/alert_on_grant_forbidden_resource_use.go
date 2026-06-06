package hooks

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/theory/jsonpath"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/restmapper"
)

const (
	grantViolationMetricName = "d8_cluster_objects_grant_violated"
	// A single shared metric group is expired once per run and then fully
	// repopulated. This ensures metrics of deleted grants (which are no longer
	// iterated) disappear instead of lingering as phantom firing alerts.
	grantViolationMetricGroup = "cluster_objects_grant_violations"
)

// systemNamespacePrefixes mirrors the in-controller namespaces.IsSystem check: such
// namespaces are never project namespaces and must be ignored.
var systemNamespacePrefixes = []string{"d8-", "kube-", "upmeter-probe-namespace-"}

func isSystemNamespace(name string) bool {
	for _, p := range systemNamespacePrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

type grant struct {
	ObjectMeta v1.ObjectMeta `json:"metadata"`
	Spec       struct {
		ProjectSelector *v1.LabelSelector `json:"projectSelector"`
		Policies        []policyReference `json:"clusterObjectGrantPolicies"`
	} `json:"spec"`
}

type policyReference struct {
	Name            string            `json:"name"`
	Default         string            `json:"default"`
	Allowed         []string          `json:"allowed"`
	AllowedSelector *v1.LabelSelector `json:"allowedSelector"`
}

type violation struct {
	GVR                schema.GroupVersionResource
	Project            string
	Name               string
	ViolatingFieldPath string
}

type usageReference struct {
	APIVersion string `json:"apiVersion"`
	Resource   string `json:"resource"`
	FieldPath  string `json:"fieldPath"`
}

type clusterObjectGrantPolicy struct {
	Spec struct {
		GrantedResource struct {
			APIVersion string `json:"apiVersion"`
			Kind       string `json:"kind"`
		} `json:"grantedResource"`
		UsageReferences []usageReference `json:"usageReferences"`
	} `json:"spec"`
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/160-multitenancy-manager",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "grants",
			ApiVersion: "multitenancy.deckhouse.io/v1alpha1",
			Kind:       "ClusterObjectGrant",
			FilterFunc: filterGrants,
		},
	},
}, dependency.WithExternalDependencies(checkIfGrantRulesAreViolated))

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/160-multitenancy-manager",
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "grants",
			Crontab: "*/2 * * * *",
		},
	},
}, dependency.WithExternalDependencies(scanClusterObjectGrantRulesViolations))

func filterGrants(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	p := &grant{}
	err := sdk.FromUnstructured(obj, p)
	if err != nil {
		return nil, err
	}

	return p, nil
}

func checkIfGrantRulesAreViolated(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	kubeClient := dc.MustGetK8sClient()
	log := input.Logger

	input.MetricsCollector.Expire(grantViolationMetricGroup)

	for _, snap := range input.Snapshots.Get("grants") {
		g := &grant{}
		if err := snap.UnmarshalTo(g); err != nil {
			return fmt.Errorf("unmarshal grant snapshot: %w", err)
		}

		violations, err := validateGrantNotViolated(ctx, g, kubeClient, log)
		if err != nil {
			return fmt.Errorf("scan grant %s for violations: %w", g.ObjectMeta.Name, err)
		}

		log.InfoContext(ctx, "Completed violations scan for ClusterObjectGrant",
			"grant", g.ObjectMeta.Name,
			"violations_count", len(violations),
		)

		setGrantViolationMetrics(input, g.ObjectMeta.Name, violations)
	}

	return nil
}

// setGrantViolationMetrics emits the violation metrics for a single grant into the
// shared metric group. It is used by both the event-driven and the scheduled hook so
// the two paths cannot drift in label set or semantics.
func setGrantViolationMetrics(input *go_hook.HookInput, grantName string, violations []violation) {
	metricOpts := metrics.WithGroup(grantViolationMetricGroup)

	for _, v := range violations {
		metricLabels := map[string]string{
			"grant":                 grantName,
			"project":               v.Project,
			"violating_object_name": v.Name,
			"violating_field":       v.ViolatingFieldPath,
			"violating_resource":    v.GVR.Resource,
		}
		if v.GVR.Group != "" {
			metricLabels["violating_resource"] = fmt.Sprintf("%s.%s", v.GVR.Resource, v.GVR.Group)
		}

		input.MetricsCollector.Set(grantViolationMetricName, 1, metricLabels, metricOpts)
	}
}

func scanClusterObjectGrantRulesViolations(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	log := input.Logger
	kube := dc.MustGetK8sClient()

	log.InfoContext(ctx, "Starting periodic ClusterObjectGrant violations scan")

	grantList, err := kube.Dynamic().Resource(schema.GroupVersionResource{
		Group:    "multitenancy.deckhouse.io",
		Version:  "v1alpha1",
		Resource: "clusterobjectgrants",
	}).List(ctx, v1.ListOptions{})
	if err != nil {
		return fmt.Errorf("fetch grants: %w", err)
	}

	input.MetricsCollector.Expire(grantViolationMetricGroup)

	for _, obj := range grantList.Items {
		g := &grant{}
		if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, g); err != nil {
			return err
		}

		violations, err := validateGrantNotViolated(ctx, g, kube, log)
		if err != nil {
			return fmt.Errorf("scan grant %s for violations: %w", g.ObjectMeta.Name, err)
		}

		log.InfoContext(ctx, "Completed violations scan for ClusterObjectGrant",
			"grant", g.ObjectMeta.Name,
			"violations_count", len(violations),
		)

		setGrantViolationMetrics(input, g.ObjectMeta.Name, violations)
	}

	log.InfoContext(ctx, "Finished periodic ClusterObjectGrant violations scan")
	return nil
}

// matchingNamespaces returns the non-system project namespaces whose labels match the
// grant's projectSelector. A nil selector matches nothing.
func matchingNamespaces(ctx context.Context, kube k8s.Client, sel *v1.LabelSelector) ([]string, error) {
	if sel == nil {
		return nil, nil
	}

	selector, err := v1.LabelSelectorAsSelector(sel)
	if err != nil {
		return nil, fmt.Errorf("invalid projectSelector: %w", err)
	}

	nsList, err := kube.Dynamic().
		Resource(schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}).
		List(ctx, v1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list namespaces: %w", err)
	}

	names := make([]string, 0, len(nsList.Items))
	for _, ns := range nsList.Items {
		name := ns.GetName()
		if isSystemNamespace(name) {
			continue
		}
		if selector.Matches(labels.Set(ns.GetLabels())) {
			names = append(names, name)
		}
	}
	return names, nil
}

// grantPolicyWhitelist mirrors the webhook's whitelist construction: explicit Allowed
// names, the Default, plus the names of the policy's granted resource objects matching
// AllowedSelector (union).
func grantPolicyWhitelist(ctx context.Context, kube k8s.Client, policyRef policyReference, policy *clusterObjectGrantPolicy) ([]string, error) {
	whitelist := slices.Clone(policyRef.Allowed)
	if policyRef.Default != "" && !slices.Contains(whitelist, policyRef.Default) {
		whitelist = append(whitelist, policyRef.Default)
	}

	if policyRef.AllowedSelector == nil {
		return whitelist, nil
	}

	selector, err := v1.LabelSelectorAsSelector(policyRef.AllowedSelector)
	if err != nil {
		return nil, fmt.Errorf("invalid allowedSelector: %w", err)
	}

	gvr, err := grantedResourceGVR(kube, policy.Spec.GrantedResource.APIVersion, policy.Spec.GrantedResource.Kind)
	if err != nil {
		return nil, err
	}

	list, err := kube.Dynamic().Resource(gvr).List(ctx, v1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, fmt.Errorf("list granted resource %s: %w", policy.Spec.GrantedResource.Kind, err)
	}
	for i := range list.Items {
		name := list.Items[i].GetName()
		if !slices.Contains(whitelist, name) {
			whitelist = append(whitelist, name)
		}
	}

	return whitelist, nil
}

// grantedResourceGVR resolves the GroupVersionResource of a granted resource (given by
// apiVersion + kind) using the cluster's discovery information.
func grantedResourceGVR(kube k8s.Client, apiVersion, kind string) (schema.GroupVersionResource, error) {
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("parse grantedResource apiVersion %q: %w", apiVersion, err)
	}

	groupResources, err := restmapper.GetAPIGroupResources(kube.Discovery())
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("discover api resources: %w", err)
	}

	mapper := restmapper.NewDiscoveryRESTMapper(groupResources)
	mapping, err := mapper.RESTMapping(schema.GroupKind{Group: gv.Group, Kind: kind}, gv.Version)
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("map %s/%s to resource: %w", apiVersion, kind, err)
	}

	return mapping.Resource, nil
}

func validateGrantNotViolated(ctx context.Context, g *grant, kubeClient k8s.Client, log go_hook.Logger) ([]violation, error) {
	projects, err := matchingNamespaces(ctx, kubeClient, g.Spec.ProjectSelector)
	if err != nil {
		return nil, err
	}
	if len(projects) == 0 {
		return nil, nil
	}

	policyGVR := schema.GroupVersionResource{
		Group:    "multitenancy.deckhouse.io",
		Version:  "v1alpha1",
		Resource: "clusterobjectgrantpolicies",
	}

	var violations []violation

	for _, policyRef := range g.Spec.Policies {
		policyObj, err := kubeClient.Dynamic().Resource(policyGVR).Get(ctx, policyRef.Name, v1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("get ClusterObjectGrantPolicy %s: %w", policyRef.Name, err)
		}

		policy := &clusterObjectGrantPolicy{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(policyObj.Object, policy); err != nil {
			return nil, fmt.Errorf("convert ClusterObjectGrantPolicy %s: %w", policyRef.Name, err)
		}

		// Match the admission webhook semantics: the default value and any objects
		// matching allowedSelector are considered allowed, otherwise objects admitted
		// by the webhook would be reported here as false-positive violations.
		whitelist, err := grantPolicyWhitelist(ctx, kubeClient, policyRef, policy)
		if err != nil {
			return nil, fmt.Errorf("build whitelist for policy %s: %w", policyRef.Name, err)
		}

		for _, ref := range policy.Spec.UsageReferences {
			gv, err := schema.ParseGroupVersion(ref.APIVersion)
			if err != nil {
				return nil, fmt.Errorf("parse apiVersion %q in policy %s: %w", ref.APIVersion, policyRef.Name, err)
			}
			gvr := gv.WithResource(ref.Resource)

			// Compile the JSONPath once per reference instead of once per object.
			jsonPath, err := jsonpath.Parse(ref.FieldPath)
			if err != nil {
				log.Error("Invalid JSONPath expression", "expr", ref.FieldPath, "policy", policyRef.Name)
				continue
			}

			for _, project := range projects {
				list, err := kubeClient.Dynamic().Resource(gvr).Namespace(project).List(ctx, v1.ListOptions{})
				if err != nil {
					return nil, fmt.Errorf("list %q in namespace %s: %w", gvr, project, err)
				}

				for _, item := range list.Items {
					values := jsonPath.Select(item.Object)
					if len(values) == 0 {
						continue
					}

					// Check every matched value so multi-match expressions
					// (e.g. $.spec.containers[*].image) are not silently skipped.
					for _, raw := range values {
						s, ok := raw.(string)
						if !ok {
							// Non-string values cannot match a string whitelist; skip
							// to avoid false positives (semantics tracked for redesign).
							continue
						}
						if !slices.Contains(whitelist, s) {
							violations = append(violations, violation{
								GVR:                gvr,
								Project:            project,
								Name:               item.GetName(),
								ViolatingFieldPath: ref.FieldPath,
							})
							break
						}
					}
				}
			}
		}
	}

	return violations, nil
}
