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

// Package quota computes object-quota usage and limits. Usage is recounted from the live usage
// objects across the project's namespaces (authoritative, no drift); the reconciler mirrors the
// result into GrantQuota.status for visibility.
package quota

import (
	"context"
	"fmt"

	"controller/api/v1alpha1"
	"controller/internal/engine"
	"controller/internal/jsonpath"
	"controller/internal/naming"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Pool returns the project's object-quota pool GrantQuota (control namespace == project name), or
// nil when none exists (no object quota configured).
func Pool(ctx context.Context, cl client.Client, project string) (*v1alpha1.GrantQuota, error) {
	gq := &v1alpha1.GrantQuota{}
	if err := cl.Get(ctx, client.ObjectKey{Namespace: project, Name: naming.GrantQuotaName}, gq); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get pool GrantQuota %s/%s: %w", project, naming.GrantQuotaName, err)
	}
	return gq, nil
}

// Usage maps a granted name to its per-measure used amounts.
type Usage map[string]map[string]resource.Quantity

func (u Usage) add(name, measure string, q resource.Quantity) {
	m := u[name]
	if m == nil {
		m = map[string]resource.Quantity{}
		u[name] = m
	}
	cur := m[measure]
	cur.Add(q)
	m[measure] = cur
}

// Add merges another usage into this one.
func (u Usage) Add(other Usage) {
	for name, ms := range other {
		for measure, q := range ms {
			u.add(name, measure, q)
		}
	}
}

// Get returns the used amount for (name, measure), zero if absent.
func (u Usage) Get(name, measure string) resource.Quantity {
	if m, ok := u[name]; ok {
		return m[measure]
	}
	return resource.Quantity{}
}

// ContributionUsage converts an object's engine contributions into a Usage.
func ContributionUsage(contribs []engine.Contribution) Usage {
	u := Usage{}
	for _, c := range contribs {
		for measure, q := range c.Increments {
			u.add(c.Name, measure, q)
		}
	}
	return u
}

// ProjectUsage recounts object-quota usage for a registration's incoming resource across the project
// namespaces, optionally skipping one object (by namespace/name) so an UPDATE does not double-count
// the object being admitted. gvk is the usage object's kind; plural is its resource.
func ProjectUsage(
	ctx context.Context,
	cl client.Client,
	factory jsonpath.Factory,
	reg *v1alpha1.ClusterGrantableResource,
	gvk schema.GroupVersionKind,
	plural string,
	namespaces []string,
	skipNamespace, skipName string,
) (Usage, error) {
	total := Usage{}
	for _, ns := range namespaces {
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(schema.GroupVersionKind{Group: gvk.Group, Version: gvk.Version, Kind: gvk.Kind + "List"})
		if err := cl.List(ctx, list, client.InNamespace(ns)); err != nil {
			return nil, fmt.Errorf("list %s in %s: %w", gvk.Kind, ns, err)
		}
		for i := range list.Items {
			item := &list.Items[i]
			if item.GetNamespace() == skipNamespace && item.GetName() == skipName {
				continue
			}
			contribs, err := engine.Contributions(factory, reg, item.Object, gvk.Group, gvk.Version, plural)
			if err != nil {
				return nil, err
			}
			total.Add(ContributionUsage(contribs))
		}
	}
	return total, nil
}

// Violation describes a measure that would exceed its limit.
type Violation struct {
	Name    string
	Measure string
	Used    resource.Quantity
	Adding  resource.Quantity
	Limit   resource.Quantity
}

func (v Violation) String() string {
	return fmt.Sprintf("%s/%s: used %s + %s exceeds limit %s",
		v.Name, v.Measure, v.Used.String(), v.Adding.String(), v.Limit.String())
}

// Check reports the first limit violation when adding `adding` on top of `used`, against the pool's
// limits for the given registration (resourceName). Unlimited (negative) limits and unset measures
// never violate. Returns nil when within quota.
func Check(pool *v1alpha1.GrantQuota, resourceName string, used, adding Usage) *Violation {
	if pool == nil {
		return nil
	}
	objects := pool.Spec.Objects[resourceName]
	if objects == nil {
		return nil
	}
	for name, ms := range adding {
		for measure, incr := range ms {
			limit, ok := engine.LimitFor(objects, name, measure)
			if !ok || engine.IsUnlimited(limit) {
				continue
			}
			projected := used.Get(name, measure)
			projected.Add(incr)
			if projected.Cmp(limit) > 0 {
				return &Violation{Name: name, Measure: measure, Used: used.Get(name, measure), Adding: incr, Limit: limit}
			}
		}
	}
	return nil
}
