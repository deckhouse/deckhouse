package hooks

import (
	"context"
	"fmt"
	"slices"

	"github.com/PaesslerAG/jsonpath"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	grantViolationMetricName = "d8_cluster_objects_grant_violated"
)

type grant struct {
	ObjectMeta v1.ObjectMeta `json:"metadata"`
	Spec       struct {
		Policies []policyReference `json:"clusterObjectGrantPolicies"`
	} `json:"spec"`
}

type policyReference struct {
	Name       string   `json:"name"`
	Default    string   `json:"default"`
	Allowed    []string `json:"allowed"`
	Violations []violation
}

type violation struct {
	GVR  schema.GroupVersionResource
	Name string
}

type usageReference struct {
	APIVersion string `json:"apiVersion"`
	Resource   string `json:"resource"`
	FieldPath  string `json:"fieldPath"`
}

type clusterObjectGrantPolicy struct {
	Spec struct {
		UsageReferences []usageReference `json:"usageReferences"`
	} `json:"spec"`
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/160-multitenancy-manager",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "grants",
			ApiVersion: "projects.deckhouse.io/v1alpha1",
			Kind:       "ClusterObjectsGrant",
			FilterFunc: filterGrants,
		},
	},
}, dependency.WithExternalDependencies(checkIfGrantRulesAreViolated))

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

	for _, snap := range input.Snapshots.Get("grants") {
		g := &grant{}
		if err := snap.UnmarshalTo(g); err != nil {
			return fmt.Errorf("unmarshal grant snapshot: %w", err)
		}

		metricLabels := map[string]string{
			"project":               g.ObjectMeta.Name,
			"violating_object_name": "",
			"violating_resource":    "",
		}

		log.InfoContext(ctx, "Scanning grant violations", "grant", g)

		violations, err := validateGrantNotViolated(ctx, g, kubeClient, log)
		if err != nil {
			return fmt.Errorf("scan of project %s for violations: %w", g.ObjectMeta.Name, err)
		}

		log.InfoContext(ctx, "Violations scan completed",
			"grant", g.ObjectMeta.Name,
			"violations_count", len(violations),
			"violations", violations,
		)

		if len(violations) == 0 {
			input.MetricsCollector.Set(grantViolationMetricName, 0, metricLabels)
			continue
		}

		for _, v := range violations {
			metricLabels["violating_object_name"] = v.Name
			metricLabels["violating_resource"] = v.GVR.Resource
			if v.GVR.Group != "" {
				metricLabels["violating_resource"] = fmt.Sprintf("%s.%s", v.GVR.Resource, v.GVR.Group)
			}

			input.MetricsCollector.Set(grantViolationMetricName, 1,metricLabels)

		}

		return nil
	}

	return nil
}

func validateGrantNotViolated(ctx context.Context, g *grant, kubeClient k8s.Client, log go_hook.Logger) ([]violation, error) {
	policyGVR := schema.GroupVersionResource{
		Group:    "projects.deckhouse.io",
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

		log.InfoContext(ctx,
			"Loaded policy",
			"policy", policyObj.GetName(),
			"usage_references", policy.Spec.UsageReferences,
			"usage_references_count", len(policy.Spec.UsageReferences),
		)

		for _, ref := range policy.Spec.UsageReferences {
			log.InfoContext(ctx,
				"Processing policy usage reference",
				"ref", ref,
				"policy", policyObj.GetName(),
			)
			gv, err := schema.ParseGroupVersion(ref.APIVersion)
			if err != nil {
				return nil, fmt.Errorf("parse apiVersion %q in policy %s: %w", ref.APIVersion, policyRef.Name, err)
			}
			gvr := gv.WithResource(ref.Resource)

			list, err := kubeClient.Dynamic().
				Resource(gvr).
				Namespace(g.ObjectMeta.Name).
				List(ctx, v1.ListOptions{})
			if err != nil {
				return nil, fmt.Errorf("list %q in namespace %s: %w", gvr, g.ObjectMeta.Name, err)
			}

			log.InfoContext(ctx, "Got resources for ref",
				"policy", policyObj.GetName(),
				"list", list.Items,
				"list_len", len(list.Items),
			)

			for _, item := range list.Items {
				value, err := jsonpath.Get(ref.FieldPath, item.Object)
				if err != nil {
					continue
				}

				if !slices.Contains(policyRef.Allowed, value.(string)) {
					violations = append(violations, violation{
						GVR:  gvr,
						Name: item.GetName(),
					})
				}
			}
		}
	}

	return violations, nil
}
