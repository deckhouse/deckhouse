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

package grantableclusterresourcereference

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	grantsv1alpha1 "controller/api/v1alpha1"
	"controller/internal/jsonpath"
	"controller/internal/testutil"
)

func newValidator() *validator { return &validator{factory: jsonpath.NewWithCache()} }

func raw(t *testing.T, reference *grantsv1alpha1.GrantableClusterResourceReference) []byte {
	t.Helper()
	b, err := json.Marshal(reference)
	require.NoError(t, err)
	return b
}

func request(t *testing.T, operation admissionv1.Operation, reference *grantsv1alpha1.GrantableClusterResourceReference) admission.Request {
	t.Helper()
	return admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Operation: operation,
		Name:      reference.Name,
		Object:    runtime.RawExtension{Raw: raw(t, reference)},
	}}
}

func update(t *testing.T, old, reference *grantsv1alpha1.GrantableClusterResourceReference) admission.Request {
	t.Helper()
	req := request(t, admissionv1.Update, reference)
	req.OldObject = runtime.RawExtension{Raw: raw(t, old)}
	return req
}

func reference(name string, fieldPaths ...grantsv1alpha1.FieldPath) *grantsv1alpha1.GrantableClusterResourceReference {
	return &grantsv1alpha1.GrantableClusterResourceReference{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: grantsv1alpha1.GrantableClusterResourceReferenceSpec{
			GrantableClusterResourceName: "storageclasses",
			Rule: grantsv1alpha1.UsageRule{
				APIGroups:   []string{""},
				APIVersions: []string{"v1"},
				Resources:   []string{"persistentvolumeclaims"},
			},
			FieldPaths: fieldPaths,
		},
	}
}

func fieldPath(path string, defaulting grantsv1alpha1.DefaultingMode) grantsv1alpha1.FieldPath {
	return grantsv1alpha1.FieldPath{Path: path, Defaulting: defaulting}
}

func guarded(fp grantsv1alpha1.FieldPath, matchPath string) grantsv1alpha1.FieldPath {
	fp.Match = &grantsv1alpha1.MatchPredicate{FieldPath: matchPath, Equals: "ClusterIssuer"}
	return fp
}

func TestHandle_DefaultingPath(t *testing.T) {
	ctx := context.Background()
	v := newValidator()

	t.Run("denied: FillEmpty with a wildcard path", func(t *testing.T) {
		resp := v.Handle(ctx, request(t, admissionv1.Create,
			reference("destinations", fieldPath("$.spec.clusterDestinationRefs[*]", grantsv1alpha1.DefaultingFillEmpty))))
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "spec.fieldPaths[0]")
		assert.Contains(t, resp.Result.Message, `"$.spec.clusterDestinationRefs[*]"`)
		assert.Contains(t, resp.Result.Message, "FillEmpty")
		assert.Contains(t, resp.Result.Message, "simple member paths only")
	})

	t.Run("denied: Coerce with an indexed path", func(t *testing.T) {
		resp := v.Handle(ctx, request(t, admissionv1.Create,
			reference("destinations", fieldPath("$.spec.clusterDestinationRefs[0]", grantsv1alpha1.DefaultingCoerce))))
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "Coerce")
		assert.Contains(t, resp.Result.Message, "$.spec.clusterDestinationRefs[0]")
	})

	t.Run("denied: FillEmpty with a path that only the old ad-hoc parser accepted", func(t *testing.T) {
		resp := v.Handle(ctx, request(t, admissionv1.Create,
			reference("storageclasses-pvc", fieldPath("$['']", grantsv1alpha1.DefaultingFillEmpty))))
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "simple member paths only")
	})

	t.Run("allowed: None with the same unparsable path", func(t *testing.T) {
		// A legal configuration: /is-granted evaluates the full JSONPath, only defaulting is limited.
		resp := v.Handle(ctx, request(t, admissionv1.Create,
			reference("destinations", fieldPath("$.spec.clusterDestinationRefs[*]", grantsv1alpha1.DefaultingNone))))
		assert.True(t, resp.Allowed)
	})

	t.Run("allowed: an empty defaulting mode with an unparsable path", func(t *testing.T) {
		// The CRD defaults the field to None; an object built in code may leave it empty, and the
		// mutator treats both the same way. The validator must not be stricter than the mutator.
		resp := v.Handle(ctx, request(t, admissionv1.Create,
			reference("destinations", fieldPath("$.spec.clusterDestinationRefs[*]", ""))))
		assert.True(t, resp.Allowed)
	})

	t.Run("allowed: FillEmpty with a simple member path", func(t *testing.T) {
		resp := v.Handle(ctx, request(t, admissionv1.Create,
			reference("storageclasses-pvc", fieldPath("$.spec.storageClassName", grantsv1alpha1.DefaultingFillEmpty))))
		assert.True(t, resp.Allowed)
	})

	t.Run("allowed: FillEmpty with an annotation path", func(t *testing.T) {
		resp := v.Handle(ctx, request(t, admissionv1.Create,
			reference("clusterissuers-ingress", fieldPath("$.metadata.annotations['cert-manager.io/cluster-issuer']", grantsv1alpha1.DefaultingFillEmpty))))
		assert.True(t, resp.Allowed)
	})

	t.Run("denied: the message names every broken entry and only those", func(t *testing.T) {
		resp := v.Handle(ctx, request(t, admissionv1.Create, reference("mixed",
			fieldPath("$.spec.storageClassName", grantsv1alpha1.DefaultingCoerce),
			fieldPath("$.spec.clusterDestinationRefs[*]", grantsv1alpha1.DefaultingFillEmpty),
			fieldPath("$.metadata.annotations['cert-manager.io/cluster-issuer']", grantsv1alpha1.DefaultingNone),
			fieldPath("$.spec.clusterDestinationRefs[0]", grantsv1alpha1.DefaultingCoerce))))
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "spec.fieldPaths[1]")
		assert.Contains(t, resp.Result.Message, "spec.fieldPaths[3]")
		assert.NotContains(t, resp.Result.Message, "spec.fieldPaths[0]")
		assert.NotContains(t, resp.Result.Message, "spec.fieldPaths[2]")
		assert.Contains(t, resp.Result.Message, "'mixed'")
	})
}

func TestHandle_PathsCompile(t *testing.T) {
	ctx := context.Background()
	v := newValidator()

	// Each of these is refused by the RFC 9535 parser /is-granted evaluates with; a stored one would
	// make /is-granted fail on every admission of the rule's resources. defaulting: None on purpose:
	// the check does not depend on the defaulting mode.
	for _, path := range []string{"$.spec.foo-bar", "$.metadata.annotations.cert-manager.io/cluster-issuer", "$.spec.1abc", "$.a b", "spec.storageClassName"} {
		t.Run("denied path: "+path, func(t *testing.T) {
			resp := v.Handle(ctx, request(t, admissionv1.Create,
				reference("broken", fieldPath(path, grantsv1alpha1.DefaultingNone))))
			assert.False(t, resp.Allowed)
			assert.Contains(t, resp.Result.Message, "'spec.fieldPaths[0].path'")
			assert.Contains(t, resp.Result.Message, "not a valid RFC 9535 JSONPath")
		})
	}

	t.Run("denied: a match.fieldPath that does not compile", func(t *testing.T) {
		resp := v.Handle(ctx, request(t, admissionv1.Create, reference("broken",
			guarded(fieldPath("$.spec.issuerRef.name", grantsv1alpha1.DefaultingFillEmpty), "$.spec.issuerRef.kind-x"))))
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "'spec.fieldPaths[0].match.fieldPath'")
		assert.Contains(t, resp.Result.Message, `"$.spec.issuerRef.kind-x"`)
	})

	t.Run("allowed: a valid match.fieldPath", func(t *testing.T) {
		resp := v.Handle(ctx, request(t, admissionv1.Create, reference("ok",
			guarded(fieldPath("$.spec.issuerRef.name", grantsv1alpha1.DefaultingFillEmpty), "$.spec.issuerRef.kind"))))
		assert.True(t, resp.Allowed)
	})

	t.Run("denied: an uncompilable path is reported once, not also as undefaultable", func(t *testing.T) {
		resp := v.Handle(ctx, request(t, admissionv1.Create,
			reference("broken", fieldPath("$.spec.foo-bar", grantsv1alpha1.DefaultingFillEmpty))))
		assert.False(t, resp.Allowed)
		assert.NotContains(t, resp.Result.Message, "simple member paths only")
	})
}

func TestHandle_Ratcheting(t *testing.T) {
	ctx := context.Background()
	v := newValidator()
	good := fieldPath("$.spec.storageClassName", grantsv1alpha1.DefaultingCoerce)
	undefaultable := fieldPath("$.spec.clusterDestinationRefs[*]", grantsv1alpha1.DefaultingFillEmpty)
	uncompilable := fieldPath("$.spec.foo-bar", grantsv1alpha1.DefaultingNone)
	stored := reference("stored", good, undefaultable, uncompilable)

	t.Run("allowed: metadata-only update of an object with stored invalid entries", func(t *testing.T) {
		updated := reference("stored", good, undefaultable, uncompilable)
		updated.Labels = map[string]string{"edited": "true"}
		assert.True(t, v.Handle(ctx, update(t, stored, updated)).Allowed)
	})

	t.Run("allowed: unchanged invalid entries at shifted indexes", func(t *testing.T) {
		updated := reference("stored", uncompilable, fieldPath("$.spec.volumeName", grantsv1alpha1.DefaultingNone), undefaultable)
		assert.True(t, v.Handle(ctx, update(t, stored, updated)).Allowed)
	})

	t.Run("denied: a new invalid entry next to stored ones", func(t *testing.T) {
		added := fieldPath("$.spec.clusterDestinationRefs[0]", grantsv1alpha1.DefaultingCoerce)
		resp := v.Handle(ctx, update(t, stored, reference("stored", good, undefaultable, uncompilable, added)))
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "spec.fieldPaths[3]")
		assert.NotContains(t, resp.Result.Message, "spec.fieldPaths[1]")
		assert.NotContains(t, resp.Result.Message, "spec.fieldPaths[2]")
	})

	t.Run("denied: a stored invalid entry changed but still invalid", func(t *testing.T) {
		changed := undefaultable
		changed.Defaulting = grantsv1alpha1.DefaultingCoerce
		resp := v.Handle(ctx, update(t, stored, reference("stored", good, changed, uncompilable)))
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "spec.fieldPaths[1]")
	})

	t.Run("allowed: a stored invalid entry fixed", func(t *testing.T) {
		fixed := undefaultable
		fixed.Defaulting = grantsv1alpha1.DefaultingNone
		assert.True(t, v.Handle(ctx, update(t, stored, reference("stored", good, fixed, uncompilable))).Allowed)
	})

	t.Run("denied: an invalid entry on CREATE", func(t *testing.T) {
		assert.False(t, v.Handle(ctx, request(t, admissionv1.Create, stored)).Allowed)
	})

	t.Run("allowed: an object being deleted, even with a new invalid entry", func(t *testing.T) {
		deleting := reference("stored", good, undefaultable, uncompilable, fieldPath("$.a-b", grantsv1alpha1.DefaultingFillEmpty))
		now := metav1.Now()
		deleting.DeletionTimestamp = &now
		deleting.Finalizers = nil
		assert.True(t, v.Handle(ctx, update(t, stored, deleting)).Allowed)
	})
}

// ruled is a reference with the given rule; its entries are given by the caller.
func ruled(rule grantsv1alpha1.UsageRule, fieldPaths ...grantsv1alpha1.FieldPath) *grantsv1alpha1.GrantableClusterResourceReference {
	ref := reference("scoped", fieldPaths...)
	ref.Spec.Rule = rule
	return ref
}

func rule(groups, versions, resources []string) grantsv1alpha1.UsageRule {
	return grantsv1alpha1.UsageRule{APIGroups: groups, APIVersions: versions, Resources: resources}
}

func scope(groups, versions, resources []string, path string) grantsv1alpha1.FieldPath {
	return grantsv1alpha1.FieldPath{APIGroups: groups, APIVersions: versions, Resources: resources, Path: path}
}

var (
	batch    = []string{"batch"}
	v1       = []string{"v1"}
	jobsCron = []string{"jobs", "cronjobs"}
	jobsOnly = []string{"jobs"}
	cronjobs = []string{"cronjobs"}
)

func unscopedFor(path string) grantsv1alpha1.FieldPath { return scope(nil, nil, nil, path) }

func TestHandle_ScopeWithinRule(t *testing.T) {
	ctx := context.Background()
	v := newValidator()
	r := rule(batch, v1, jobsCron)

	t.Run("denied: a resource the rule does not list", func(t *testing.T) {
		resp := v.Handle(ctx, request(t, admissionv1.Create, ruled(r,
			scope(nil, nil, []string{"cronjob"}, "$.a"), unscopedFor("$.b"))))
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, `'spec.fieldPaths[0].resources' lists "cronjob", which 'spec.rule.resources' does not include (allowed: "jobs", "cronjobs")`)
	})

	t.Run("denied: a group the rule does not list", func(t *testing.T) {
		resp := v.Handle(ctx, request(t, admissionv1.Create, ruled(r,
			unscopedFor("$.b"), scope([]string{"apps"}, nil, nil, "$.a"))))
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, `'spec.fieldPaths[1].apiGroups' lists "apps"`)
		assert.Contains(t, resp.Result.Message, `(allowed: "batch")`)
	})

	t.Run("denied: a version the rule does not list", func(t *testing.T) {
		resp := v.Handle(ctx, request(t, admissionv1.Create, ruled(r,
			scope(nil, []string{"v1beta1"}, nil, "$.a"), unscopedFor("$.b"))))
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, `'spec.fieldPaths[0].apiVersions' lists "v1beta1"`)
	})

	t.Run("allowed: any version when the rule lists '*'", func(t *testing.T) {
		resp := v.Handle(ctx, request(t, admissionv1.Create, ruled(rule(batch, []string{"*"}, jobsCron),
			scope(nil, []string{"v1beta1"}, nil, "$.a"), unscopedFor("$.b"))))
		assert.True(t, resp.Allowed, resp.Result.Message)
	})

	t.Run("allowed: any resource when the rule lists '*'", func(t *testing.T) {
		resp := v.Handle(ctx, request(t, admissionv1.Create, ruled(rule(batch, v1, []string{"*"}),
			scope(nil, nil, []string{"anything"}, "$.a"), unscopedFor("$.b"))))
		assert.True(t, resp.Allowed, resp.Result.Message)
	})

	t.Run("allowed: '*' in the entry's apiGroups and apiVersions restricts nothing", func(t *testing.T) {
		resp := v.Handle(ctx, request(t, admissionv1.Create, ruled(r,
			scope([]string{"*"}, []string{"*"}, nil, "$.a"))))
		assert.True(t, resp.Allowed, resp.Result.Message)
	})
}

func TestHandle_Coverage(t *testing.T) {
	ctx := context.Background()
	v := newValidator()

	t.Run("denied: a rule resource no entry applies to", func(t *testing.T) {
		resp := v.Handle(ctx, request(t, admissionv1.Create, ruled(rule(batch, v1, jobsCron),
			scope(nil, nil, jobsOnly, "$.spec.template.spec.priorityClassName"))))
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, `no 'spec.fieldPaths' entry applies to (apiGroup "batch", apiVersion "v1", resource "cronjobs")`)
		assert.NotContains(t, resp.Result.Message, `resource "jobs"`)
	})

	t.Run("allowed: scoped entries cover every rule resource without a fallback", func(t *testing.T) {
		resp := v.Handle(ctx, request(t, admissionv1.Create, ruled(rule(batch, v1, jobsCron),
			scope(nil, nil, jobsOnly, "$.spec.template.spec.priorityClassName"),
			scope(nil, nil, cronjobs, "$.spec.jobTemplate.spec.template.spec.priorityClassName"))))
		assert.True(t, resp.Allowed, resp.Result.Message)
	})

	t.Run("allowed: the fallback covers what the scoped entries leave", func(t *testing.T) {
		resp := v.Handle(ctx, request(t, admissionv1.Create, ruled(rule(batch, v1, jobsCron),
			scope(nil, nil, cronjobs, "$.spec.jobTemplate.spec.template.spec.priorityClassName"),
			unscopedFor("$.spec.template.spec.priorityClassName"))))
		assert.True(t, resp.Allowed, resp.Result.Message)
	})

	t.Run("denied: '*' in rule.resources without an entry unrestricted by resource", func(t *testing.T) {
		resp := v.Handle(ctx, request(t, admissionv1.Create, ruled(rule(batch, v1, []string{"*"}),
			scope(nil, nil, jobsCron, "$.a"))))
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, `(apiGroup "batch", apiVersion "v1", resource <any other>)`)
	})

	t.Run("denied: '*' in rule.apiVersions and entries scoped to listed versions only", func(t *testing.T) {
		resp := v.Handle(ctx, request(t, admissionv1.Create, ruled(rule(batch, []string{"*"}, jobsOnly),
			scope(nil, v1, nil, "$.a"))))
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "apiVersion <any other>")
	})

	t.Run("denied: every uncovered combination is listed", func(t *testing.T) {
		resp := v.Handle(ctx, request(t, admissionv1.Create, ruled(rule([]string{"", "apps"}, v1, []string{"pods", "deployments"}),
			scope([]string{""}, nil, []string{"pods"}, "$.a"))))
		assert.False(t, resp.Allowed)
		for _, missing := range []string{
			`(apiGroup "", apiVersion "v1", resource "deployments")`,
			`(apiGroup "apps", apiVersion "v1", resource "pods")`,
			`(apiGroup "apps", apiVersion "v1", resource "deployments")`,
		} {
			assert.Contains(t, resp.Result.Message, missing)
		}
		assert.NotContains(t, resp.Result.Message, `(apiGroup "", apiVersion "v1", resource "pods")`)
	})
}

func TestHandle_CoverageRatcheting(t *testing.T) {
	ctx := context.Background()
	v := newValidator()
	jobs := scope(nil, nil, jobsOnly, "$.spec.template.spec.priorityClassName")
	stored := ruled(rule(batch, v1, jobsCron), jobs)

	t.Run("allowed: metadata-only update of a stored reference with a coverage hole", func(t *testing.T) {
		updated := ruled(rule(batch, v1, jobsCron), jobs)
		updated.Labels = map[string]string{"edited": "true"}
		assert.True(t, v.Handle(ctx, update(t, stored, updated)).Allowed)
	})

	t.Run("denied: a rule change that leaves a hole", func(t *testing.T) {
		covered := ruled(rule(batch, v1, jobsOnly), jobs)
		resp := v.Handle(ctx, update(t, covered, ruled(rule(batch, v1, jobsCron), jobs)))
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, `resource "cronjobs"`)
	})

	t.Run("denied: a rule change that drops a resource an unchanged entry is scoped to", func(t *testing.T) {
		old := ruled(rule(batch, v1, jobsCron), jobs, unscopedFor("$.b"))
		resp := v.Handle(ctx, update(t, old, ruled(rule(batch, v1, cronjobs), jobs, unscopedFor("$.b"))))
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, `'spec.fieldPaths[0].resources' lists "jobs"`)
	})

	t.Run("allowed: an entry change that closes the hole", func(t *testing.T) {
		assert.True(t, v.Handle(ctx, update(t, stored, ruled(rule(batch, v1, jobsCron), jobs, unscopedFor("$.b")))).Allowed)
	})
}

// TestHandle_ShippedReferences runs every GrantableClusterResourceReference the module ships in
// templates/cluster-objects-controller/grantable-resources.yaml through this webhook: the module must
// not be able to block its own Helm release.
func TestHandle_ShippedReferences(t *testing.T) {
	refs := testutil.RenderShipped[grantsv1alpha1.GrantableClusterResourceReference](t, "GrantableClusterResourceReference")

	names := make([]string, 0, len(refs))
	for _, ref := range refs {
		names = append(names, ref.Name)
	}
	// Guard the rendering itself: a template that renders to nothing must not pass as "nothing to check".
	require.ElementsMatch(t, []string{
		"storageclasses-pvc",
		"loadbalancerclasses-service",
		"clusterissuers-certificate",
		"clusterissuers-ingress",
		"clusterroles-rolebinding",
	}, names)

	v := newValidator()
	for _, ref := range refs {
		t.Run(ref.Name, func(t *testing.T) {
			resp := v.Handle(context.Background(), request(t, admissionv1.Create, ref))
			assert.True(t, resp.Allowed, "shipped reference %q must pass validation: %s", ref.Name, resp.Result.Message)
		})
	}
}
