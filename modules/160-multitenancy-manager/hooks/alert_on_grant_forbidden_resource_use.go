/*
Copyright 2026 Flant JSC

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
	"context"
	"fmt"
	"strings"

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

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
)

const (
	grantViolationMetricName = "d8_cluster_objects_grant_violated"
	// A single shared metric group is expired once per run and then fully
	// repopulated. This ensures metrics of deleted grants disappear instead of
	// lingering as phantom firing alerts.
	grantViolationMetricGroup = "cluster_objects_grant_violations"
)

// systemNamespacePrefixes mirrors the in-controller namespaces.IsSystem check.
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
		Resources       []grantResource   `json:"resources"`
	} `json:"spec"`
}

type grantResource struct {
	ResourceName        string            `json:"resourceName"`
	Allowed             []string          `json:"allowed"`
	AllowedSelector     *v1.LabelSelector `json:"allowedSelector"`
	Denied              []string          `json:"denied"`
	DeniedSelector      *v1.LabelSelector `json:"deniedSelector"`
	Default             string            `json:"default"`
	AvailabilityDefault string            `json:"availabilityDefault"`
}

type resourceFilter struct {
	Names            []string                      `json:"names"`
	MatchLabels      map[string]string             `json:"matchLabels"`
	MatchExpressions []v1.LabelSelectorRequirement `json:"matchExpressions"`
}

type usageRule struct {
	APIGroups   []string `json:"apiGroups"`
	APIVersions []string `json:"apiVersions"`
	Resources   []string `json:"resources"`
}

type pathOverride struct {
	APIGroups   []string `json:"apiGroups"`
	APIVersions []string `json:"apiVersions"`
	FieldPath   string   `json:"fieldPath"`
}

type usageReference struct {
	Rule      usageRule      `json:"rule"`
	FieldPath string         `json:"fieldPath"`
	Paths     []pathOverride `json:"paths"`
}

type clusterGrantableResource struct {
	Spec struct {
		GrantedResource *struct {
			APIVersion string `json:"apiVersion"`
			Kind       string `json:"kind"`
		} `json:"grantedResource"`
		Enforcement         string           `json:"enforcement"`
		DefaultAvailability string           `json:"defaultAvailability"`
		Excluded            []resourceFilter `json:"excluded"`
		UsageReferences     []usageReference `json:"usageReferences"`
	} `json:"spec"`
}

type violation struct {
	GVR                schema.GroupVersionResource
	Project            string
	Name               string
	ViolatingFieldPath string
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/160-multitenancy-manager",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "grants",
			ApiVersion: "multitenancy.deckhouse.io/v1alpha1",
			Kind:       "ClusterResourceGrantPolicy",
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
}, dependency.WithExternalDependencies(scanClusterResourceGrantPolicyRulesViolations))

func filterGrants(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	g := &grant{}
	if err := sdk.FromUnstructured(obj, g); err != nil {
		return nil, err
	}
	return g, nil
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
		setGrantViolationMetrics(input, g.ObjectMeta.Name, violations)
	}
	return nil
}

func scanClusterResourceGrantPolicyRulesViolations(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	log := input.Logger
	kube := dc.MustGetK8sClient()

	grantList, err := kube.Dynamic().Resource(schema.GroupVersionResource{
		Group:    "multitenancy.deckhouse.io",
		Version:  "v1alpha1",
		Resource: "clusterresourcegrantpolicies",
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
		setGrantViolationMetrics(input, g.ObjectMeta.Name, violations)
	}
	return nil
}

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

// matchingNamespaces returns the non-system project namespaces whose labels match the selector.
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

// decisionSets holds the resolved allow/deny/excluded names for a registration in one grant entry.
// It mirrors the controller's resolve.Resolved so the alert matches what the webhook enforces.
type decisionSets struct {
	allowed             map[string]struct{}
	denied              map[string]struct{}
	excluded            map[string]struct{}
	anyAll              bool
	anyNone             bool
	registrationDefault string
}

// violates is the negation of the controller's resolve.Resolved.Decide: precedence
// excluded → denied → allowed → grant baseline (anyAll/anyNone) → registration defaultAvailability.
func (d decisionSets) violates(name string) bool {
	if _, ok := d.excluded[name]; ok {
		return true
	}
	if _, ok := d.denied[name]; ok {
		return true
	}
	if _, ok := d.allowed[name]; ok {
		return false
	}
	if d.anyAll {
		return false
	}
	if d.anyNone {
		return true
	}
	return d.registrationDefault == "None"
}

func buildDecisionSets(ctx context.Context, kube k8s.Client, entry grantResource, reg *clusterGrantableResource) (decisionSets, error) {
	d := decisionSets{
		allowed:             map[string]struct{}{},
		denied:              map[string]struct{}{},
		excluded:            map[string]struct{}{},
		registrationDefault: reg.Spec.DefaultAvailability,
	}
	switch entry.AvailabilityDefault {
	case "All":
		d.anyAll = true
	case "None":
		d.anyNone = true
	default:
		// No explicit baseline: an allow-list (names or selector) means "restrict to it", so the
		// baseline for everything else is None — mirroring resolve.Resolve in the controller.
		if len(entry.Allowed) > 0 || entry.AllowedSelector != nil {
			d.anyNone = true
		}
	}
	for _, n := range entry.Allowed {
		d.allowed[n] = struct{}{}
	}
	if entry.Default != "" {
		d.allowed[entry.Default] = struct{}{}
	}
	for _, n := range entry.Denied {
		d.denied[n] = struct{}{}
	}
	// Registration excluded literal names (union across all excluded filters).
	for i := range reg.Spec.Excluded {
		for _, n := range reg.Spec.Excluded[i].Names {
			d.excluded[n] = struct{}{}
		}
	}
	// Object-backed: expand selectors against live granted objects.
	if reg.Spec.GrantedResource != nil && reg.Spec.GrantedResource.Kind != "" {
		gvr, err := grantedResourceGVR(kube, reg.Spec.GrantedResource.APIVersion, reg.Spec.GrantedResource.Kind)
		if err != nil {
			return d, err
		}
		list, err := kube.Dynamic().Resource(gvr).List(ctx, v1.ListOptions{})
		if err != nil {
			return d, fmt.Errorf("list granted resource %s: %w", reg.Spec.GrantedResource.Kind, err)
		}
		excludedSels := make([]labels.Selector, 0, len(reg.Spec.Excluded))
		for i := range reg.Spec.Excluded {
			if sel := filterToSelector(&reg.Spec.Excluded[i]); sel != nil {
				excludedSels = append(excludedSels, sel)
			}
		}
		allowedSel := labelSelector(entry.AllowedSelector)
		deniedSel := labelSelector(entry.DeniedSelector)
		for i := range list.Items {
			name := list.Items[i].GetName()
			set := labels.Set(list.Items[i].GetLabels())
			for _, excludedSel := range excludedSels {
				if excludedSel.Matches(set) {
					d.excluded[name] = struct{}{}
					break
				}
			}
			if deniedSel != nil && deniedSel.Matches(set) {
				d.denied[name] = struct{}{}
			}
			if allowedSel != nil && allowedSel.Matches(set) {
				d.allowed[name] = struct{}{}
			}
		}
	}
	return d, nil
}

func labelSelector(ls *v1.LabelSelector) labels.Selector {
	if ls == nil {
		return nil
	}
	sel, err := v1.LabelSelectorAsSelector(ls)
	if err != nil {
		return nil
	}
	return sel
}

func filterToSelector(f *resourceFilter) labels.Selector {
	if f == nil || (len(f.MatchLabels) == 0 && len(f.MatchExpressions) == 0) {
		return nil
	}
	sel, err := v1.LabelSelectorAsSelector(&v1.LabelSelector{MatchLabels: f.MatchLabels, MatchExpressions: f.MatchExpressions})
	if err != nil {
		return nil
	}
	return sel
}

// grantedResourceGVR resolves a granted resource (apiVersion + kind) to its GVR via discovery.
func grantedResourceGVR(kube k8s.Client, apiVersion, kind string) (schema.GroupVersionResource, error) {
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("parse apiVersion %q: %w", apiVersion, err)
	}
	groupResources, err := restmapper.GetAPIGroupResources(kube.Discovery())
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("discover api resources: %w", err)
	}
	mapper := restmapper.NewDiscoveryRESTMapper(groupResources)
	mapping, err := mapper.RESTMapping(schema.GroupKind{Group: gv.Group, Kind: kind}, gv.Version)
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("map %s/%s: %w", apiVersion, kind, err)
	}
	return mapping.Resource, nil
}

// usageGVRs resolves the concrete GVRs a usage rule targets (skipping wildcard groups). Concrete
// versions are used directly; a "*" version is resolved via discovery.
func usageGVRs(kube k8s.Client, rule usageRule) []schema.GroupVersionResource {
	seen := map[schema.GroupVersionResource]struct{}{}
	var out []schema.GroupVersionResource
	add := func(gvr schema.GroupVersionResource) {
		if _, dup := seen[gvr]; dup {
			return
		}
		seen[gvr] = struct{}{}
		out = append(out, gvr)
	}

	var mapper interface {
		ResourcesFor(schema.GroupVersionResource) ([]schema.GroupVersionResource, error)
	}
	needDiscovery := false
	for _, g := range rule.APIGroups {
		if g == "*" {
			continue
		}
		for _, v := range rule.APIVersions {
			if v == "*" {
				needDiscovery = true
			}
		}
	}
	if needDiscovery {
		groupResources, err := restmapper.GetAPIGroupResources(kube.Discovery())
		if err == nil {
			mapper = restmapper.NewDiscoveryRESTMapper(groupResources)
		}
	}

	for _, g := range rule.APIGroups {
		if g == "*" {
			continue
		}
		for _, res := range rule.Resources {
			for _, v := range rule.APIVersions {
				if v != "*" {
					add(schema.GroupVersionResource{Group: g, Version: v, Resource: res})
					continue
				}
				if mapper == nil {
					continue
				}
				gvrs, err := mapper.ResourcesFor(schema.GroupVersionResource{Group: g, Resource: res})
				if err != nil {
					continue
				}
				for _, gvr := range gvrs {
					add(gvr)
				}
			}
		}
	}
	return out
}

func versionMatches(versions []string, v string) bool {
	for _, x := range versions {
		if x == "*" || x == v {
			return true
		}
	}
	return false
}

// selectFieldPath returns the path for the given group/version (paths[] override else default).
func selectFieldPath(ref usageReference, group, version string) string {
	for _, p := range ref.Paths {
		groupOK := len(p.APIGroups) == 0 || versionMatches(p.APIGroups, group)
		versionOK := len(p.APIVersions) == 0 || versionMatches(p.APIVersions, version)
		if groupOK && versionOK {
			return p.FieldPath
		}
	}
	return ref.FieldPath
}

func validateGrantNotViolated(ctx context.Context, g *grant, kube k8s.Client, log go_hook.Logger) ([]violation, error) {
	projects, err := matchingNamespaces(ctx, kube, g.Spec.ProjectSelector)
	if err != nil {
		return nil, err
	}
	if len(projects) == 0 {
		return nil, nil
	}

	regGVR := schema.GroupVersionResource{Group: "multitenancy.deckhouse.io", Version: "v1alpha1", Resource: "grantableclusterresourcedefinitions"}

	var violations []violation
	for _, entry := range g.Spec.Resources {
		regObj, err := kube.Dynamic().Resource(regGVR).Get(ctx, entry.ResourceName, v1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("get GrantableClusterResourceDefinition %s: %w", entry.ResourceName, err)
		}
		reg := &clusterGrantableResource{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(regObj.Object, reg); err != nil {
			return nil, fmt.Errorf("convert GrantableClusterResourceDefinition %s: %w", entry.ResourceName, err)
		}
		if reg.Spec.Enforcement == "External" {
			continue
		}
		decision, err := buildDecisionSets(ctx, kube, entry, reg)
		if err != nil {
			return nil, fmt.Errorf("build decision for %s: %w", entry.ResourceName, err)
		}

		for _, ref := range reg.Spec.UsageReferences {
			for _, gvr := range usageGVRs(kube, ref.Rule) {
				path := selectFieldPath(ref, gvr.Group, gvr.Version)
				jsonPath, err := jsonpath.Parse(path)
				if err != nil {
					log.Error("Invalid JSONPath expression", "expr", path, "registration", entry.ResourceName)
					continue
				}
				for _, project := range projects {
					list, err := kube.Dynamic().Resource(gvr).Namespace(project).List(ctx, v1.ListOptions{})
					if err != nil {
						continue
					}
					for _, item := range list.Items {
						for _, rawVal := range jsonPath.Select(item.Object) {
							s, ok := rawVal.(string)
							if !ok || s == "" {
								continue
							}
							if decision.violates(s) {
								violations = append(violations, violation{
									GVR:                gvr,
									Project:            project,
									Name:               item.GetName(),
									ViolatingFieldPath: path,
								})
								break
							}
						}
					}
				}
			}
		}
	}
	return violations, nil
}
