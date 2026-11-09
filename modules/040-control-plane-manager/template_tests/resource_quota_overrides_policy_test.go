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

package template_tests

import (
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/apiserver/pkg/cel/environment"
)

// These tests cover the two ValidatingAdmissionPolicies that keep the
// resource-quota-overrides.deckhouse.io/* labels, which the patched kube-apiserver uses to leave
// pods and PVCs out of ResourceQuota, to Deckhouse components. They compile the expressions straight
// out of the chart template and evaluate them the way the API server does: matchConditions first
// (any false one skips the policy, an error rejects the request because failurePolicy is Fail),
// then the variables, then the validations.

const (
	quotaOverridesPolicy          = "resource-quota-overrides.deckhouse.io"
	quotaOverridesTemplatesPolicy = "resource-quota-overrides-templates.deckhouse.io"
	quotaIgnoreLabel              = "resource-quota-overrides.deckhouse.io/ignore"
)

const (
	quotaTenant               = "alice@example.com"
	quotaTenantServiceAccount = "system:serviceaccount:team-a:ci"
	quotaClusterAdmin         = "kubernetes-admin"
	quotaDeckhouseComponent   = "system:serviceaccount:d8-virtualization:virtualization-controller"
	quotaDeploymentController = "system:serviceaccount:kube-system:deployment-controller"
	quotaReplicaSetController = "system:serviceaccount:kube-system:replicaset-controller"
	quotaCronJobController    = "system:serviceaccount:kube-system:cronjob-controller"
	quotaStatefulSetCtrl      = "system:serviceaccount:kube-system:statefulset-controller"
	quotaDaemonSetCtrl        = "system:serviceaccount:kube-system:daemon-set-controller"
	quotaEphemeralVolumeCtrl  = "system:serviceaccount:kube-system:ephemeral-volume-controller"
	quotaKubeScheduler        = "system:kube-scheduler"
)

type quotaObject = map[string]any

type quotaDecision string

const (
	quotaAllowed quotaDecision = "allowed"
	quotaDenied  quotaDecision = "denied"
	quotaSkipped quotaDecision = "skipped"
)

// quotaTemplateResources are the resources the controllers of kube-controller-manager create pods
// and PVCs out of.
var quotaTemplateResources = []struct{ resource, group, kind string }{
	{"deployments", "apps", "Deployment"},
	{"replicasets", "apps", "ReplicaSet"},
	{"daemonsets", "apps", "DaemonSet"},
	{"statefulsets", "apps", "StatefulSet"},
	{"jobs", "batch", "Job"},
	{"cronjobs", "batch", "CronJob"},
	{"replicationcontrollers", "", "ReplicationController"},
}

func TestResourceQuotaOverridesPolicyRules(t *testing.T) {
	documents := loadQuotaDocuments(t)

	tests := []struct {
		policy string
		rules  []quotaResourceRule
	}{
		{
			policy: quotaOverridesPolicy,
			rules: []quotaResourceRule{
				{APIGroups: []string{""}, APIVersions: []string{"v1"}, Operations: []string{"CREATE", "UPDATE"}, Resources: []string{"pods", "pods/resize", "persistentvolumeclaims"}},
			},
		},
		{
			policy: quotaOverridesTemplatesPolicy,
			rules: []quotaResourceRule{
				{APIGroups: []string{"apps"}, APIVersions: []string{"*"}, Operations: []string{"CREATE", "UPDATE"}, Resources: []string{"deployments", "replicasets", "daemonsets", "statefulsets"}},
				{APIGroups: []string{"batch"}, APIVersions: []string{"*"}, Operations: []string{"CREATE", "UPDATE"}, Resources: []string{"jobs", "cronjobs"}},
				{APIGroups: []string{""}, APIVersions: []string{"v1"}, Operations: []string{"CREATE", "UPDATE"}, Resources: []string{"replicationcontrollers"}},
				{APIGroups: []string{"apps"}, APIVersions: []string{"*"}, Operations: []string{"CREATE"}, Resources: []string{"controllerrevisions"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.policy, func(t *testing.T) {
			policy, ok := documents[tt.policy]["ValidatingAdmissionPolicy"]
			if !ok {
				t.Fatalf("no ValidatingAdmissionPolicy %s in templates/validation.yaml", tt.policy)
			}
			if policy.Spec.FailurePolicy != "Fail" {
				t.Errorf("failurePolicy is %q, want Fail", policy.Spec.FailurePolicy)
			}
			if !reflect.DeepEqual(policy.Spec.MatchConstraints.ResourceRules, tt.rules) {
				t.Errorf("resourceRules are %+v, want %+v", policy.Spec.MatchConstraints.ResourceRules, tt.rules)
			}

			binding, ok := documents[tt.policy]["ValidatingAdmissionPolicyBinding"]
			if !ok {
				t.Fatalf("no ValidatingAdmissionPolicyBinding %s in templates/validation.yaml", tt.policy)
			}
			if !reflect.DeepEqual(binding.Spec.ValidationActions, []string{"Deny"}) {
				t.Errorf("validationActions are %v, want [Deny]", binding.Spec.ValidationActions)
			}
		})
	}
}

func TestResourceQuotaOverridesTemplatesPolicy(t *testing.T) {
	policy := loadQuotaPolicy(t, quotaOverridesTemplatesPolicy)

	// Every resource a controller reads a pod template from.
	for _, r := range quotaTemplateResources {
		t.Run(r.kind, func(t *testing.T) {
			labeled := quotaWorkload(r.resource, quotaPodTemplate(quotaLabels(quotaIgnoreLabel, "true")))
			plain := quotaWorkload(r.resource, quotaPodTemplate(quotaLabels()))
			// The labels of the resource itself reach no pod or PVC, so they do not change the quota.
			labeledItself := quotaWorkload(r.resource, quotaPodTemplate(quotaLabels()))
			labeledItself["metadata"].(quotaObject)["labels"] = quotaLabels(quotaIgnoreLabel, "true")

			cases := []struct {
				name    string
				request quotaObject
				object  quotaObject
				want    quotaDecision
			}{
				{"a user may not label the pod template", quotaRequest("CREATE", r.resource, quotaTenant), labeled, quotaDenied},
				{"a Deckhouse component may label the pod template", quotaRequest("CREATE", r.resource, quotaDeckhouseComponent), labeled, quotaAllowed},
				{"a pod template without the labels is not restricted", quotaRequest("CREATE", r.resource, quotaTenant), plain, quotaAllowed},
				{"the labels of the resource itself are not restricted", quotaRequest("CREATE", r.resource, quotaTenant), labeledItself, quotaAllowed},
			}
			for _, c := range cases {
				t.Run(c.name, func(t *testing.T) {
					expectQuotaDecision(t, policy, c.request, nil, c.object, c.want)
				})
			}
		})
	}

	tests := []struct {
		name      string
		request   quotaObject
		oldObject quotaObject
		object    quotaObject
		want      quotaDecision
	}{
		// -- who may set the labels --------------------------------------------------------
		{
			name:    "a service account of a tenant may not label a pod template",
			request: quotaRequest("CREATE", "deployments", quotaTenantServiceAccount),
			object:  quotaWorkload("deployments", quotaPodTemplate(quotaLabels(quotaIgnoreLabel, "true"))),
			want:    quotaDenied,
		},
		{
			name:    "a cluster admin may not label a pod template",
			request: quotaRequest("CREATE", "deployments", quotaClusterAdmin, "system:masters"),
			object:  quotaWorkload("deployments", quotaPodTemplate(quotaLabels(quotaIgnoreLabel, "true"))),
			want:    quotaDenied,
		},
		{
			name:    "a kube-system service account is not trusted with templates",
			request: quotaRequest("CREATE", "deployments", quotaReplicaSetController),
			object:  quotaWorkload("deployments", quotaPodTemplate(quotaLabels(quotaIgnoreLabel, "true"))),
			want:    quotaDenied,
		},

		// -- controllers that copy a template checked by this policy ------------------
		{
			name:    "the deployment controller may create a labeled ReplicaSet",
			request: quotaRequest("CREATE", "replicasets", quotaDeploymentController),
			object:  quotaWorkload("replicasets", quotaPodTemplate(quotaLabels(quotaIgnoreLabel, "true"))),
			want:    quotaAllowed,
		},
		{
			name:    "the deployment controller may not create a labeled Deployment",
			request: quotaRequest("CREATE", "deployments", quotaDeploymentController),
			object:  quotaWorkload("deployments", quotaPodTemplate(quotaLabels(quotaIgnoreLabel, "true"))),
			want:    quotaDenied,
		},
		{
			name:    "the cronjob controller may create a labeled Job",
			request: quotaRequest("CREATE", "jobs", quotaCronJobController),
			object:  quotaWorkload("jobs", quotaPodTemplate(quotaLabels(quotaIgnoreLabel, "true"))),
			want:    quotaAllowed,
		},
		{
			name:    "the cronjob controller may not create a labeled CronJob",
			request: quotaRequest("CREATE", "cronjobs", quotaCronJobController),
			object:  quotaWorkload("cronjobs", quotaPodTemplate(quotaLabels(quotaIgnoreLabel, "true"))),
			want:    quotaDenied,
		},

		// -- controller revisions -----------------------------------------------------------
		// The StatefulSet controller creates the pods below the partition out of the revision named
		// in status.currentRevision, so a user who replaces that revision chooses their template.
		{
			name:    "a user may not create a revision with a labeled pod template",
			request: quotaRequest("CREATE", "controllerrevisions", quotaTenant),
			object:  quotaControllerRevision(quotaRevisionData(quotaPodTemplate(quotaLabels(quotaIgnoreLabel, "true")))),
			want:    quotaDenied,
		},
		{
			name:    "a user may not create a revision with a labeled volume claim template",
			request: quotaRequest("CREATE", "controllerrevisions", quotaTenant),
			object:  quotaControllerRevision(quotaRevisionData(quotaPodTemplate(quotaLabels()), quotaVolumeClaim(quotaLabels(quotaIgnoreLabel, "true")))),
			want:    quotaDenied,
		},
		{
			name:    "the statefulset controller may create a labeled revision",
			request: quotaRequest("CREATE", "controllerrevisions", quotaStatefulSetCtrl),
			object:  quotaControllerRevision(quotaRevisionData(quotaPodTemplate(quotaLabels(quotaIgnoreLabel, "true")))),
			want:    quotaAllowed,
		},
		{
			name:    "the daemonset controller may create a labeled revision",
			request: quotaRequest("CREATE", "controllerrevisions", quotaDaemonSetCtrl),
			object:  quotaControllerRevision(quotaRevisionData(quotaPodTemplate(quotaLabels(quotaIgnoreLabel, "true")))),
			want:    quotaAllowed,
		},
		{
			name:    "other kube-system service accounts may not create a labeled revision",
			request: quotaRequest("CREATE", "controllerrevisions", quotaDeploymentController),
			object:  quotaControllerRevision(quotaRevisionData(quotaPodTemplate(quotaLabels(quotaIgnoreLabel, "true")))),
			want:    quotaDenied,
		},
		{
			name:    "a revision without the labels is not restricted",
			request: quotaRequest("CREATE", "controllerrevisions", quotaTenant),
			object:  quotaControllerRevision(quotaRevisionData(quotaPodTemplate(quotaLabels()))),
			want:    quotaAllowed,
		},
		{
			name:    "the labels of the revision itself are not restricted",
			request: quotaRequest("CREATE", "controllerrevisions", quotaTenant),
			object: func() quotaObject {
				revision := quotaControllerRevision(quotaRevisionData(quotaPodTemplate(quotaLabels())))
				revision["metadata"].(quotaObject)["labels"] = quotaLabels(quotaIgnoreLabel, "true")
				return revision
			}(),
			want: quotaAllowed,
		},
		{
			name:    "a revision without data is not restricted",
			request: quotaRequest("CREATE", "controllerrevisions", quotaTenant),
			object:  quotaControllerRevision(nil),
			want:    quotaAllowed,
		},

		// -- where the labels are -------------------------------------------------------
		{
			name:    "the labels of the Job a CronJob creates do not reach its pods",
			request: quotaRequest("CREATE", "cronjobs", quotaTenant),
			object:  quotaCronJobWithJobLabels(quotaLabels(quotaIgnoreLabel, "true")),
			want:    quotaAllowed,
		},
		{
			name:    "a user may not label a volume claim template of a StatefulSet",
			request: quotaRequest("CREATE", "statefulsets", quotaTenant),
			object:  quotaStatefulSet(quotaLabels(quotaIgnoreLabel, "true")),
			want:    quotaDenied,
		},
		{
			name:    "a user may not label an ephemeral volume claim template in a pod template",
			request: quotaRequest("CREATE", "deployments", quotaTenant),
			object:  quotaWorkload("deployments", quotaPodTemplate(quotaLabels(), quotaEphemeralVolume(quotaLabels(quotaIgnoreLabel, "true")))),
			want:    quotaDenied,
		},
		{
			name:    "a pod template with unlabeled and non-ephemeral volumes is not restricted",
			request: quotaRequest("CREATE", "deployments", quotaTenant),
			object: quotaWorkload("deployments", quotaPodTemplate(quotaLabels(),
				quotaEphemeralVolume(nil), quotaObject{"name": "cache", "emptyDir": quotaObject{}})),
			want: quotaAllowed,
		},
		{
			name:    "the ignore label is restricted whatever its value",
			request: quotaRequest("CREATE", "deployments", quotaTenant),
			object:  quotaWorkload("deployments", quotaPodTemplate(quotaLabels(quotaIgnoreLabel, "false"))),
			want:    quotaDenied,
		},
		{
			name:    "every label with the prefix is restricted",
			request: quotaRequest("CREATE", "deployments", quotaTenant),
			object:  quotaWorkload("deployments", quotaPodTemplate(quotaLabels("resource-quota-overrides.deckhouse.io/future-label", "1"))),
			want:    quotaDenied,
		},
		{
			name:    "a label that only starts like the prefix is not restricted",
			request: quotaRequest("CREATE", "deployments", quotaTenant),
			object:  quotaWorkload("deployments", quotaPodTemplate(quotaLabels("resource-quota-overrides.deckhouse.io.example.com/ignore", "true"))),
			want:    quotaAllowed,
		},

		// -- updates ----------------------------------------------------------------------
		{
			name:      "a user may restart a labeled Deployment",
			request:   quotaRequest("UPDATE", "deployments", quotaTenant),
			oldObject: quotaWorkload("deployments", quotaPodTemplate(quotaLabels(quotaIgnoreLabel, "true"))),
			object:    quotaRestarted(quotaWorkload("deployments", quotaPodTemplate(quotaLabels(quotaIgnoreLabel, "true")))),
			want:      quotaAllowed,
		},
		{
			name:      "a user may update a StatefulSet with labeled volume claim templates",
			request:   quotaRequest("UPDATE", "statefulsets", quotaTenant),
			oldObject: quotaStatefulSet(quotaLabels(quotaIgnoreLabel, "true")),
			object:    quotaRestarted(quotaStatefulSet(quotaLabels(quotaIgnoreLabel, "true"))),
			want:      quotaAllowed,
		},
		{
			name:      "a user may not add the labels to a pod template",
			request:   quotaRequest("UPDATE", "deployments", quotaTenant),
			oldObject: quotaWorkload("deployments", quotaPodTemplate(quotaLabels())),
			object:    quotaWorkload("deployments", quotaPodTemplate(quotaLabels(quotaIgnoreLabel, "true"))),
			want:      quotaDenied,
		},
		{
			name:      "a user may not change the value of a label",
			request:   quotaRequest("UPDATE", "deployments", quotaTenant),
			oldObject: quotaWorkload("deployments", quotaPodTemplate(quotaLabels(quotaIgnoreLabel, "false"))),
			object:    quotaWorkload("deployments", quotaPodTemplate(quotaLabels(quotaIgnoreLabel, "true"))),
			want:      quotaDenied,
		},
		{
			name:      "a user may not add a volume to a labeled template, even without the labels",
			request:   quotaRequest("UPDATE", "deployments", quotaTenant),
			oldObject: quotaWorkload("deployments", quotaPodTemplate(quotaLabels(quotaIgnoreLabel, "true"))),
			object: quotaWorkload("deployments", quotaPodTemplate(quotaLabels(quotaIgnoreLabel, "true"),
				quotaObject{"name": "cache", "emptyDir": quotaObject{}})),
			want: quotaDenied,
		},
		{
			name:      "a user may not change the resources of a labeled template",
			request:   quotaRequest("UPDATE", "deployments", quotaTenant),
			oldObject: quotaWorkload("deployments", quotaPodTemplate(quotaLabels(quotaIgnoreLabel, "true"))),
			object: quotaWithContainer(quotaWorkload("deployments", quotaPodTemplate(quotaLabels(quotaIgnoreLabel, "true"))),
				quotaObject{"name": "web", "image": "registry.example.com/web:1.0",
					"resources": quotaObject{"requests": quotaObject{"cpu": "64", "memory": "256Gi"}}}),
			want: quotaDenied,
		},
		{
			name:      "a user may not change the volume claim templates of a labeled StatefulSet",
			request:   quotaRequest("UPDATE", "statefulsets", quotaTenant),
			oldObject: quotaStatefulSet(quotaLabels(quotaIgnoreLabel, "true")),
			object: func() quotaObject {
				statefulSet := quotaStatefulSet(quotaLabels(quotaIgnoreLabel, "true"))
				claim := statefulSet["spec"].(quotaObject)["volumeClaimTemplates"].([]any)[0].(quotaObject)
				claim["spec"].(quotaObject)["resources"] = quotaObject{"requests": quotaObject{"storage": "100Ti"}}
				return statefulSet
			}(),
			want: quotaDenied,
		},
		{
			// The scale subresource changes replicas without the template, so the policy cannot see it either.
			name:      "a user may scale a labeled Deployment",
			request:   quotaRequest("UPDATE", "deployments", quotaTenant),
			oldObject: quotaWorkload("deployments", quotaPodTemplate(quotaLabels(quotaIgnoreLabel, "true"))),
			object: func() quotaObject {
				deployment := quotaWorkload("deployments", quotaPodTemplate(quotaLabels(quotaIgnoreLabel, "true")))
				deployment["spec"].(quotaObject)["replicas"] = int64(3)
				return deployment
			}(),
			want: quotaAllowed,
		},
		{
			name:      "a user may change the other labels of a labeled template",
			request:   quotaRequest("UPDATE", "deployments", quotaTenant),
			oldObject: quotaWorkload("deployments", quotaPodTemplate(quotaLabels(quotaIgnoreLabel, "true"))),
			object:    quotaWorkload("deployments", quotaPodTemplate(quotaLabels(quotaIgnoreLabel, "true", "version", "2"))),
			want:      quotaAllowed,
		},
		{
			name:      "a user may change a template without the labels",
			request:   quotaRequest("UPDATE", "deployments", quotaTenant),
			oldObject: quotaWorkload("deployments", quotaPodTemplate(quotaLabels())),
			object: quotaWithContainer(quotaWorkload("deployments", quotaPodTemplate(quotaLabels())),
				quotaObject{"name": "web", "image": "registry.example.com/web:2.0"}),
			want: quotaAllowed,
		},
		{
			name:      "a user may change a template while removing the labels",
			request:   quotaRequest("UPDATE", "deployments", quotaTenant),
			oldObject: quotaWorkload("deployments", quotaPodTemplate(quotaLabels(quotaIgnoreLabel, "true"))),
			object: quotaWithContainer(quotaWorkload("deployments", quotaPodTemplate(quotaLabels())),
				quotaObject{"name": "web", "image": "registry.example.com/web:2.0"}),
			want: quotaAllowed,
		},
		{
			name:      "a Deckhouse component may change the labels",
			request:   quotaRequest("UPDATE", "deployments", quotaDeckhouseComponent),
			oldObject: quotaWorkload("deployments", quotaPodTemplate(quotaLabels(quotaIgnoreLabel, "false"))),
			object:    quotaWorkload("deployments", quotaPodTemplate(quotaLabels(quotaIgnoreLabel, "true"))),
			want:      quotaAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectQuotaDecision(t, policy, tt.request, tt.oldObject, tt.object, tt.want)
		})
	}

	t.Run("the denial names the requester", func(t *testing.T) {
		labeled := quotaWorkload("deployments", quotaPodTemplate(quotaLabels(quotaIgnoreLabel, "true")))
		decision, message := policy.evaluate(quotaRequest("CREATE", "deployments", quotaTenant), nil, labeled)
		if decision != quotaDenied || !strings.Contains(message, quotaTenant) {
			t.Errorf("got %s (%s), want a denial naming %s", decision, message, quotaTenant)
		}
	})
}

func TestResourceQuotaOverridesPodsPolicy(t *testing.T) {
	policy := loadQuotaPolicy(t, quotaOverridesPolicy)

	labeledPod := func() quotaObject { return quotaPod(quotaLabels(quotaIgnoreLabel, "true")) }
	podWithLabeledClaim := func() quotaObject {
		return quotaPod(quotaLabels(), quotaEphemeralVolume(quotaLabels(quotaIgnoreLabel, "true")))
	}

	tests := []struct {
		name      string
		request   quotaObject
		oldObject quotaObject
		object    quotaObject
		want      quotaDecision
	}{
		// -- pod labels ---------------------------------------------------------------------
		{"a user may not label a pod", quotaRequest("CREATE", "pods", quotaTenant), nil, labeledPod(), quotaDenied},
		{"a cluster admin may not label a pod", quotaRequest("CREATE", "pods", quotaClusterAdmin, "system:masters"), nil, labeledPod(), quotaDenied},
		{"a Deckhouse component may label a pod", quotaRequest("CREATE", "pods", quotaDeckhouseComponent), nil, labeledPod(), quotaAllowed},
		{"a controller may create a pod out of a checked template", quotaRequest("CREATE", "pods", quotaReplicaSetController), nil, labeledPod(), quotaAllowed},
		{"the scheduler may update a labeled pod", quotaRequest("UPDATE", "pods", quotaKubeScheduler), labeledPod(), labeledPod(), quotaAllowed},
		{"a user may not update a labeled pod even leaving the labels as they are", quotaRequest("UPDATE", "pods", quotaTenant), labeledPod(), labeledPod(), quotaDenied},
		{"a pod without the labels is out of scope", quotaRequest("CREATE", "pods", quotaTenant), nil, quotaPod(nil), quotaSkipped},

		// -- ephemeral volume claim templates -------------------------------------------
		{"a user may not label an ephemeral volume claim template", quotaRequest("CREATE", "pods", quotaTenant), nil, podWithLabeledClaim(), quotaDenied},
		{"a controller may create a pod with a labeled ephemeral volume claim template", quotaRequest("CREATE", "pods", quotaReplicaSetController), nil, podWithLabeledClaim(), quotaAllowed},
		{
			name:    "a pod with unlabeled and non-ephemeral volumes is out of scope",
			request: quotaRequest("CREATE", "pods", quotaTenant),
			object:  quotaPod(nil, quotaEphemeralVolume(nil), quotaObject{"name": "cache", "emptyDir": quotaObject{}}),
			want:    quotaSkipped,
		},

		// -- PVC labels ---------------------------------------------------------------------
		{"a user may not label a PVC", quotaRequest("CREATE", "persistentvolumeclaims", quotaTenant), nil, quotaPVC(quotaLabels(quotaIgnoreLabel, "true")), quotaDenied},
		{"the statefulset controller may create a labeled PVC", quotaRequest("CREATE", "persistentvolumeclaims", quotaStatefulSetCtrl), nil, quotaPVC(quotaLabels(quotaIgnoreLabel, "true")), quotaAllowed},
		{"the ephemeral volume controller may create a labeled PVC", quotaRequest("CREATE", "persistentvolumeclaims", quotaEphemeralVolumeCtrl), nil, quotaPVC(quotaLabels(quotaIgnoreLabel, "true")), quotaAllowed},
		{"a PVC without the labels is out of scope", quotaRequest("CREATE", "persistentvolumeclaims", quotaTenant), nil, quotaPVC(nil), quotaSkipped},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectQuotaDecision(t, policy, tt.request, tt.oldObject, tt.object, tt.want)
		})
	}
}

func expectQuotaDecision(t *testing.T, policy quotaPolicy, request, oldObject, object quotaObject, want quotaDecision) {
	t.Helper()

	decision, detail := policy.evaluate(request, oldObject, object)
	if decision != want {
		t.Errorf("got %s (%s), want %s", decision, detail, want)
	}
}

// -- objects ---------------------------------------------------------------------------------

// quotaLabels returns the labels of an application plus the given key-value pairs.
func quotaLabels(pairs ...string) quotaObject {
	labels := quotaObject{"app": "web"}
	for i := 0; i+1 < len(pairs); i += 2 {
		labels[pairs[i]] = pairs[i+1]
	}
	return labels
}

// quotaPodTemplate leaves metadata.labels out when labels is nil, as the API server does.
func quotaPodTemplate(labels quotaObject, volumes ...any) quotaObject {
	metadata := quotaObject{}
	if labels != nil {
		metadata["labels"] = labels
	}
	spec := quotaObject{"containers": []any{quotaObject{"name": "web", "image": "registry.example.com/web:1.0"}}}
	if len(volumes) > 0 {
		spec["volumes"] = volumes
	}
	return quotaObject{"metadata": metadata, "spec": spec}
}

// quotaWorkload puts the pod template where the controller of the resource reads it from.
func quotaWorkload(resource string, template quotaObject) quotaObject {
	spec := quotaObject{"template": template}
	if resource == "cronjobs" {
		spec = quotaObject{
			"schedule":    "*/5 * * * *",
			"jobTemplate": quotaObject{"spec": quotaObject{"template": template}},
		}
	}
	for _, r := range quotaTemplateResources {
		if r.resource == resource {
			return quotaObject{
				"apiVersion": strings.TrimPrefix(r.group+"/v1", "/"),
				"kind":       r.kind,
				"metadata":   quotaObject{"name": "web", "namespace": "team-a"},
				"spec":       spec,
			}
		}
	}
	panic("unknown resource " + resource)
}

func quotaCronJobWithJobLabels(labels quotaObject) quotaObject {
	cronJob := quotaWorkload("cronjobs", quotaPodTemplate(quotaLabels()))
	cronJob["spec"].(quotaObject)["jobTemplate"].(quotaObject)["metadata"] = quotaObject{"labels": labels}
	return cronJob
}

func quotaStatefulSet(claimLabels quotaObject) quotaObject {
	claim := quotaVolumeClaim(claimLabels)
	claim["metadata"].(quotaObject)["name"] = "data"
	statefulSet := quotaWorkload("statefulsets", quotaPodTemplate(quotaLabels()))
	statefulSet["spec"].(quotaObject)["volumeClaimTemplates"] = []any{claim}
	return statefulSet
}

// quotaRevisionData is the data the StatefulSet controller writes into a ControllerRevision: a patch
// that replaces the pod template of the StatefulSet.
func quotaRevisionData(template quotaObject, claims ...any) quotaObject {
	template["$patch"] = "replace"
	spec := quotaObject{"template": template}
	if len(claims) > 0 {
		spec["volumeClaimTemplates"] = claims
	}
	return quotaObject{"spec": spec}
}

// quotaControllerRevision leaves data out when it is nil.
func quotaControllerRevision(data quotaObject) quotaObject {
	revision := quotaObject{
		"apiVersion": "apps/v1",
		"kind":       "ControllerRevision",
		"metadata":   quotaObject{"name": "web-665c9c676c", "namespace": "team-a"},
		"revision":   int64(1),
	}
	if data != nil {
		revision["data"] = data
	}
	return revision
}

func quotaEphemeralVolume(labels quotaObject) quotaObject {
	return quotaObject{"name": "scratch", "ephemeral": quotaObject{"volumeClaimTemplate": quotaVolumeClaim(labels)}}
}

func quotaVolumeClaim(labels quotaObject) quotaObject {
	metadata := quotaObject{}
	if labels != nil {
		metadata["labels"] = labels
	}
	return quotaObject{
		"metadata": metadata,
		"spec": quotaObject{
			"accessModes": []any{"ReadWriteOnce"},
			"resources":   quotaObject{"requests": quotaObject{"storage": "1Ti"}},
		},
	}
}

func quotaPod(labels quotaObject, volumes ...any) quotaObject {
	pod := quotaPodTemplate(labels, volumes...)
	pod["apiVersion"] = "v1"
	pod["kind"] = "Pod"
	pod["metadata"].(quotaObject)["name"] = "web-0"
	pod["metadata"].(quotaObject)["namespace"] = "team-a"
	return pod
}

func quotaPVC(labels quotaObject) quotaObject {
	pvc := quotaVolumeClaim(labels)
	pvc["apiVersion"] = "v1"
	pvc["kind"] = "PersistentVolumeClaim"
	pvc["metadata"].(quotaObject)["name"] = "data-web-0"
	pvc["metadata"].(quotaObject)["namespace"] = "team-a"
	return pvc
}

// quotaWithContainer replaces the containers of the pod template of a workload.
func quotaWithContainer(workload quotaObject, container quotaObject) quotaObject {
	template := workload["spec"].(quotaObject)["template"].(quotaObject)
	template["spec"].(quotaObject)["containers"] = []any{container}
	return workload
}

// quotaRestarted does what kubectl rollout restart does to a workload.
func quotaRestarted(workload quotaObject) quotaObject {
	template := workload["spec"].(quotaObject)["template"].(quotaObject)
	template["metadata"].(quotaObject)["annotations"] = quotaObject{"kubectl.kubernetes.io/restartedAt": "2026-11-03T10:00:00Z"}
	return workload
}

func quotaRequest(operation, resource, username string, groups ...string) quotaObject {
	group := ""
	if resource == "controllerrevisions" {
		group = "apps"
	}
	for _, r := range quotaTemplateResources {
		if r.resource == resource {
			group = r.group
		}
	}
	return quotaObject{
		"operation": operation,
		"resource":  quotaObject{"group": group, "version": "v1", "resource": resource},
		"userInfo": quotaObject{
			"username": username,
			"groups":   append([]string{"system:authenticated"}, groups...),
		},
	}
}

// -- policy evaluation ---------------------------------------------------------------------

type quotaProgram struct {
	name    string
	program cel.Program
	// message is the messageExpression of a validation, staticMessage its message.
	message       cel.Program
	staticMessage string
}

type quotaPolicy struct {
	matchConditions []quotaProgram
	variables       []quotaProgram
	validations     []quotaProgram
}

// evaluate returns the decision and, depending on it, the match condition that skipped the policy
// or the denial message.
func (p quotaPolicy) evaluate(request, oldObject, object quotaObject) (quotaDecision, string) {
	variables := map[string]any{}
	activation := map[string]any{
		"request":   request,
		"object":    quotaCELValue(object),
		"oldObject": quotaCELValue(oldObject),
		"variables": variables,
	}

	for _, condition := range p.matchConditions {
		out, _, err := condition.program.Eval(activation)
		if err != nil {
			return quotaDenied, fmt.Sprintf("the %s match condition errored: %v", condition.name, err)
		}
		if out != types.True {
			return quotaSkipped, condition.name
		}
	}

	// The API server evaluates a variable when an expression first uses it. Evaluating all of them
	// in order is stricter: an error in any of them fails the request.
	for _, variable := range p.variables {
		out, _, err := variable.program.Eval(activation)
		if err != nil {
			return quotaDenied, fmt.Sprintf("the %s variable errored: %v", variable.name, err)
		}
		variables[variable.name] = out
	}

	for _, validation := range p.validations {
		out, _, err := validation.program.Eval(activation)
		if err != nil {
			return quotaDenied, fmt.Sprintf("a validation errored: %v", err)
		}
		if out == types.True {
			continue
		}
		if validation.message == nil {
			return quotaDenied, validation.staticMessage
		}
		message, _, err := validation.message.Eval(activation)
		if err != nil {
			return quotaDenied, fmt.Sprintf("the message expression errored: %v", err)
		}
		return quotaDenied, fmt.Sprint(message.Value())
	}

	return quotaAllowed, ""
}

func quotaCELValue(o quotaObject) ref.Val {
	if o == nil {
		return types.NullValue
	}
	return types.DefaultTypeAdapter.NativeToValue(o)
}

func loadQuotaPolicy(t *testing.T, name string) quotaPolicy {
	t.Helper()

	document, ok := loadQuotaDocuments(t)[name]["ValidatingAdmissionPolicy"]
	if !ok {
		t.Fatalf("no ValidatingAdmissionPolicy %s in templates/validation.yaml", name)
	}

	env := quotaCELEnvironment(t)
	policy := quotaPolicy{}
	for _, condition := range document.Spec.MatchConditions {
		policy.matchConditions = append(policy.matchConditions, quotaProgram{
			name:    condition.Name,
			program: compileQuotaExpression(t, env, condition.Name, condition.Expression, cel.BoolType),
		})
	}
	for _, variable := range document.Spec.Variables {
		policy.variables = append(policy.variables, quotaProgram{
			name:    variable.Name,
			program: compileQuotaExpression(t, env, variable.Name, variable.Expression, nil),
		})
	}
	for i, validation := range document.Spec.Validations {
		compiled := quotaProgram{
			name:          fmt.Sprintf("validation %d", i),
			program:       compileQuotaExpression(t, env, fmt.Sprintf("validation %d", i), validation.Expression, cel.BoolType),
			staticMessage: validation.Message,
		}
		if validation.MessageExpression != "" {
			compiled.message = compileQuotaExpression(t, env, fmt.Sprintf("message %d", i), validation.MessageExpression, cel.StringType)
		}
		policy.validations = append(policy.validations, compiled)
	}
	if len(policy.validations) == 0 {
		t.Fatalf("the %s policy has no validations", name)
	}
	return policy
}

// quotaCELEnvironment declares what a ValidatingAdmissionPolicy exposes to its expressions with the
// CEL libraries of the API server. The API server compiles a new expression with the libraries of
// the previous minor version, so 1.32 is what the oldest supported Kubernetes, 1.33, offers.
func quotaCELEnvironment(t *testing.T) *cel.Env {
	t.Helper()

	envSet, err := environment.MustBaseEnvSet(version.MajorMinor(1, 32)).Extend(environment.VersionedOptions{
		IntroducedVersion: version.MajorMinor(1, 0),
		EnvOptions: []cel.EnvOption{
			cel.Variable("object", cel.DynType),
			cel.Variable("oldObject", cel.DynType),
			cel.Variable("request", cel.DynType),
			cel.Variable("variables", cel.MapType(cel.StringType, cel.DynType)),
		},
	})
	if err != nil {
		t.Fatalf("extending the CEL environment: %v", err)
	}
	env, err := envSet.Env(environment.NewExpressions)
	if err != nil {
		t.Fatalf("building the CEL environment: %v", err)
	}
	return env
}

// compileQuotaExpression checks the output type unless want is nil, as for variables.
func compileQuotaExpression(t *testing.T, env *cel.Env, name, expression string, want *cel.Type) cel.Program {
	t.Helper()

	ast, issues := env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		t.Fatalf("compiling %s: %v", name, issues.Err())
	}
	if want != nil && !ast.OutputType().IsExactType(want) {
		t.Fatalf("%s evaluates to %s, want %s", name, ast.OutputType(), want)
	}
	program, err := env.Program(ast)
	if err != nil {
		t.Fatalf("instantiating %s: %v", name, err)
	}
	return program
}

// -- chart template ------------------------------------------------------------------------

type quotaResourceRule struct {
	APIGroups   []string `yaml:"apiGroups"`
	APIVersions []string `yaml:"apiVersions"`
	Operations  []string `yaml:"operations"`
	Resources   []string `yaml:"resources"`
}

type quotaExpression struct {
	Name              string `yaml:"name"`
	Expression        string `yaml:"expression"`
	Message           string `yaml:"message"`
	MessageExpression string `yaml:"messageExpression"`
}

// quotaDocument holds the fields of a policy and of a binding the tests look at.
type quotaDocument struct {
	Kind string `yaml:"kind"`
	Spec struct {
		FailurePolicy    string `yaml:"failurePolicy"`
		MatchConstraints struct {
			ResourceRules []quotaResourceRule `yaml:"resourceRules"`
		} `yaml:"matchConstraints"`
		MatchConditions   []quotaExpression `yaml:"matchConditions"`
		Variables         []quotaExpression `yaml:"variables"`
		Validations       []quotaExpression `yaml:"validations"`
		ValidationActions []string          `yaml:"validationActions"`
	} `yaml:"spec"`
}

var (
	quotaPolicyNameDeclaration = regexp.MustCompile(`\$policyName := "([^"]+)"`)
	quotaHelmAction            = regexp.MustCompile(`{{.*?}}`)
)

// loadQuotaDocuments returns the documents of templates/validation.yaml by policy name and kind.
// Every policy takes its name from a $policyName declaration, so a document belongs to the last
// declaration before it.
func loadQuotaDocuments(t *testing.T) map[string]map[string]quotaDocument {
	t.Helper()

	content, err := os.ReadFile("../templates/validation.yaml")
	if err != nil {
		t.Fatalf("reading templates/validation.yaml: %v", err)
	}

	documents := map[string]map[string]quotaDocument{}
	policyName := ""
	for _, chunk := range strings.Split(string(content), "\n---") {
		if match := quotaPolicyNameDeclaration.FindStringSubmatch(chunk); match != nil {
			policyName = match[1]
		}
		var document quotaDocument
		if err := yaml.Unmarshal([]byte(quotaWithoutHelmActions(chunk)), &document); err != nil {
			t.Fatalf("parsing a document of the %s policy: %v", policyName, err)
		}
		if document.Kind == "" {
			continue
		}
		if documents[policyName] == nil {
			documents[policyName] = map[string]quotaDocument{}
		}
		documents[policyName][document.Kind] = document
	}
	return documents
}

// quotaWithoutHelmActions turns a chart template into parseable YAML: lines that hold nothing but
// a Helm action are dropped, inline actions become a placeholder.
func quotaWithoutHelmActions(template string) string {
	var kept []string
	for _, line := range strings.Split(template, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "{{") && strings.HasSuffix(trimmed, "}}") {
			continue
		}
		kept = append(kept, quotaHelmAction.ReplaceAllString(line, "helm-value"))
	}
	return strings.Join(kept, "\n")
}
