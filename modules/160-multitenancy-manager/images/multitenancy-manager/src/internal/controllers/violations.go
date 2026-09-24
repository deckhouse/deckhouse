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

package controllers

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	"controller/api/v1alpha1"
	"controller/internal/engine"
	"controller/internal/jsonpath"
	"controller/internal/resolve"
)

// grantViolations is the series the ClusterResourceGrantPolicyViolation alert and the "Cluster Resource
// Grant Violations" dashboard read. It used to be produced by a module hook with its own copy of the
// availability rules, and the copy disagreed with the webhook on one point that matters: two
// allow-list policies on the same resource in one project are a union for the webhook, so an object
// using a name the second policy allows is admitted -- and the hook, evaluating one policy at a time,
// reported it as a violation. The controller computes it here through the same resolver the webhook
// uses, so the alert can only fire on what the webhook would refuse.
//
// The label set is the hook's: project is the namespace (the dashboard keys on it), grant names each
// applicable policy that holds an entry for the definition, so a violation shows up once per such
// policy exactly as before.
var grantViolations = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "d8_cluster_objects_grant_violated",
	Help: "An object in a project namespace references a cluster-wide resource that no applicable ClusterResourceGrantPolicy allows.",
}, []string{"project", "grant", "violating_resource", "violating_object_name", "violating_field"})

func init() {
	metrics.Registry.MustRegister(grantViolations)
}

// violation is one object field naming a cluster-wide resource the project may not use.
type violation struct {
	grant    string
	resource string
	object   string
	field    string
}

// clearViolations drops every series of the namespace. It runs before each recount, and when the
// namespace stops being a project namespace or disappears, so a series never outlives its cause.
func clearViolations(namespace string) {
	grantViolations.DeletePartialMatch(prometheus.Labels{"project": namespace})
}

// publishViolations replaces the series of the namespace with the current set.
func publishViolations(namespace string, violations []violation) {
	clearViolations(namespace)
	for _, v := range violations {
		grantViolations.With(prometheus.Labels{
			"project":               namespace,
			"grant":                 v.grant,
			"violating_resource":    v.resource,
			"violating_object_name": v.object,
			"violating_field":       v.field,
		}).Set(1)
	}
}

// scanViolations walks the usage objects of the namespace that a GrantableClusterResourceReference
// governs and returns the ones referencing a name the resolved availability refuses. It is the read
// side of /is-granted: the same references, the same field paths and guards, the same Resolve and
// Decide -- only without the grandfathering an UPDATE gets, because here the question is what is
// in use, not what is being written.
//
// usage is the uncached API reader: the objects are of arbitrary kinds (PVCs, Ingresses,
// RoleBindings, whatever a reference names) and a cached read would start an informer per kind
// for the sake of a two-minute resync.
func scanViolations(
	ctx context.Context,
	cl client.Reader,
	usage client.Reader,
	mapper meta.RESTMapper,
	factory jsonpath.Factory,
	namespace string,
	grants []*v1alpha1.ClusterResourceGrantPolicy,
	definitions []v1alpha1.GrantableClusterResourceDefinition,
) ([]violation, error) {
	refList := &v1alpha1.GrantableClusterResourceReferenceList{}
	if err := cl.List(ctx, refList); err != nil {
		return nil, fmt.Errorf("list GrantableClusterResourceReferences: %w", err)
	}
	defByName := make(map[string]*v1alpha1.GrantableClusterResourceDefinition, len(definitions))
	for i := range definitions {
		defByName[definitions[i].Name] = &definitions[i]
	}

	resolvedByDef := map[string]*resolve.Resolved{}
	var out []violation
	for i := range refList.Items {
		ref := &refList.Items[i]
		def := defByName[ref.Spec.GrantableClusterResourceName]
		if def == nil || def.Spec.Enforcement == v1alpha1.EnforcementExternal {
			continue
		}
		entries := resolve.EntriesFor(grants, def.Name)
		grantNames := grantsNaming(grants, def.Name)
		if len(grantNames) == 0 {
			// No applicable policy holds an entry for this definition: the hook never reported such
			// a namespace either, and the alert has no policy to point at.
			continue
		}

		for _, gvr := range usageGVRs(mapper, ref.Spec.Rule) {
			fp, ok := engine.SelectFieldPath(ref.Spec.FieldPaths, gvr.Group, gvr.Version, gvr.Resource)
			if !ok {
				continue
			}
			gvk, err := mapper.KindFor(gvr)
			if err != nil {
				// The CRD behind the rule is not installed: nothing to scan, like the hook.
				continue
			}
			list := &unstructured.UnstructuredList{}
			list.SetGroupVersionKind(schema.GroupVersionKind{Group: gvk.Group, Version: gvk.Version, Kind: gvk.Kind + "List"})
			if err := usage.List(ctx, list, client.InNamespace(namespace)); err != nil {
				return nil, fmt.Errorf("list %s in %s: %w", gvr.Resource, namespace, err)
			}
			if len(list.Items) == 0 {
				continue
			}

			resolved := resolvedByDef[def.Name]
			if resolved == nil {
				resolved, err = resolve.Resolve(ctx, cl, mapper, def, entries)
				if err != nil {
					return nil, fmt.Errorf("resolve %s: %w", def.Name, err)
				}
				resolvedByDef[def.Name] = resolved
			}

			resource := gvr.Resource
			if gvr.Group != "" {
				resource = gvr.Resource + "." + gvr.Group
			}
			for j := range list.Items {
				obj := list.Items[j].Object
				guardOK, err := engine.EvalMatch(factory, fp.Match, obj)
				if err != nil || !guardOK {
					continue
				}
				names, err := engine.StringValuesAt(factory, obj, fp.Path)
				if err != nil {
					continue
				}
				for _, name := range names {
					if name == "" || resolved.Decide(name) {
						continue
					}
					for _, grant := range grantNames {
						out = append(out, violation{grant: grant, resource: resource, object: list.Items[j].GetName(), field: fp.Path})
					}
					break
				}
			}
		}
	}
	return out, nil
}

// grantsNaming returns the names of the applicable policies that hold an entry for the definition.
func grantsNaming(grants []*v1alpha1.ClusterResourceGrantPolicy, definition string) []string {
	var names []string
	for _, g := range grants {
		for i := range g.Spec.Resources {
			if g.Spec.Resources[i].ResourceName == definition {
				names = append(names, g.Name)
				break
			}
		}
	}
	return names
}

// usageGVRs expands a reference rule into the concrete (group, version, resource) triples it covers.
// A wildcard group cannot be enumerated and is skipped, as the hook did; a wildcard version is
// resolved through the REST mapper to every served version of the resource.
func usageGVRs(mapper meta.RESTMapper, rule v1alpha1.UsageRule) []schema.GroupVersionResource {
	seen := map[schema.GroupVersionResource]struct{}{}
	var out []schema.GroupVersionResource
	add := func(gvr schema.GroupVersionResource) {
		if _, dup := seen[gvr]; dup {
			return
		}
		seen[gvr] = struct{}{}
		out = append(out, gvr)
	}
	for _, group := range rule.APIGroups {
		if group == "*" {
			continue
		}
		for _, resource := range rule.Resources {
			if resource == "*" {
				continue
			}
			for _, version := range rule.APIVersions {
				if version != "*" {
					add(schema.GroupVersionResource{Group: group, Version: version, Resource: resource})
					continue
				}
				gvrs, err := mapper.ResourcesFor(schema.GroupVersionResource{Group: group, Resource: resource})
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
