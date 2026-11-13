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
	"context"
	"os"

	celgo "github.com/google/cel-go/cel"
	celtypes "github.com/google/cel-go/common/types"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/apiserver/pkg/admission"
	admissioncel "k8s.io/apiserver/pkg/admission/plugin/cel"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/cel/environment"
	"sigs.k8s.io/yaml"
)

// admissionPolicy evaluates a rendered ValidatingAdmissionPolicy with the CEL
// library of kube-apiserver. An expression error denies the request, because the
// failure policy is Fail.
type admissionPolicy struct {
	gvk         schema.GroupVersionKind
	gvr         schema.GroupVersionResource
	match       admissioncel.ConditionEvaluator
	validations admissioncel.ConditionEvaluator
}

type condition string

func (c condition) GetExpression() string    { return string(c) }
func (condition) ReturnTypes() []*celgo.Type { return []*celgo.Type{celgo.BoolType} }

func compileNodePolicy(rendered string) admissionPolicy {
	return compilePolicy(rendered, schema.GroupVersionKind{Version: "v1", Kind: "Node"}, "nodes")
}

func compileStatesPolicy(rendered string) admissionPolicy {
	gvk := schema.GroupVersionKind{Group: fencingGroup, Version: "v1alpha1", Kind: "FencingFailedNodeState"}
	return compilePolicy(rendered, gvk, "fencingfailednodestates")
}

func compilePolicy(rendered string, gvk schema.GroupVersionKind, resource string) admissionPolicy {
	var policy admissionregistrationv1.ValidatingAdmissionPolicy
	Expect(yaml.UnmarshalStrict([]byte(rendered), &policy)).To(Succeed())

	compile := func(expressions []string) admissioncel.ConditionEvaluator {
		// The environment of the oldest supported Kubernetes version: a newer CEL
		// feature would deny every request of the agent on an older cluster.
		compiler, err := admissioncel.NewCompositedCompiler(environment.MustBaseEnvSet(oldestKubernetes()))
		Expect(err).NotTo(HaveOccurred())

		var conditions []admissioncel.ExpressionAccessor
		for _, e := range expressions {
			Expect(compiler.CompileCELExpression(condition(e), admissioncel.OptionalVariableDeclarations{}, environment.NewExpressions).Error).To(BeNil(), e)
			conditions = append(conditions, condition(e))
		}

		return compiler.CompileCondition(conditions, admissioncel.OptionalVariableDeclarations{}, environment.NewExpressions)
	}

	var match, validations []string
	for _, c := range policy.Spec.MatchConditions {
		match = append(match, c.Expression)
	}
	for _, v := range policy.Spec.Validations {
		validations = append(validations, v.Expression)
	}

	return admissionPolicy{
		gvk:         gvk,
		gvr:         gvk.GroupVersion().WithResource(resource),
		match:       compile(match),
		validations: compile(validations),
	}
}

// admits tells whether the policy lets the user do the operation on an object.
// oldObject is nil for CREATE, object is nil for DELETE.
func (p admissionPolicy) admits(who user.Info, operation admission.Operation, subresource string, oldObject, object map[string]any) bool {
	holds := func(evaluator admissioncel.ConditionEvaluator) bool {
		results, err := p.evaluate(evaluator, who, operation, subresource, oldObject, object)
		if err != nil {
			return false
		}
		for _, r := range results {
			if r.Error != nil || r.EvalResult != celtypes.True {
				return false
			}
		}

		return true
	}

	return !holds(p.match) || holds(p.validations)
}

// matches tells whether the match conditions of the policy select the request. An error of an
// expression fails the test: with the failure policy Fail, the API server denies such a request.
func (p admissionPolicy) matches(who user.Info, operation admission.Operation, oldObject, object map[string]any) bool {
	results, err := p.evaluate(p.match, who, operation, "", oldObject, object)
	Expect(err).NotTo(HaveOccurred())
	for _, r := range results {
		Expect(r.Error).To(BeNil(), r.ExpressionAccessor.GetExpression())
		if r.EvalResult != celtypes.True {
			return false
		}
	}

	return true
}

func (p admissionPolicy) evaluate(evaluator admissioncel.ConditionEvaluator, who user.Info, operation admission.Operation, subresource string, oldObject, object map[string]any) ([]admissioncel.EvaluationResult, error) {
	gvk, gvr := p.gvk, p.gvr

	// A nil object must stay a nil interface, not a typed nil pointer.
	var obj, oldObj runtime.Object
	var name string
	if object != nil {
		u := &unstructured.Unstructured{Object: object}
		obj, name = u, u.GetName()
	}
	if oldObject != nil {
		u := &unstructured.Unstructured{Object: oldObject}
		oldObj, name = u, u.GetName()
	}

	attributes := admission.NewAttributesRecord(obj, oldObj, gvk, "", name, gvr, subresource, operation, nil, false, who)
	versioned, err := admission.NewVersionedAttributes(attributes, gvk, nil)
	Expect(err).NotTo(HaveOccurred())
	request := admissioncel.CreateAdmissionRequest(attributes, metav1.GroupVersionResource(gvr), metav1.GroupVersionKind(gvk))

	results, _, err := evaluator.ForInput(context.Background(), versioned, request, admissioncel.OptionalVariableBindings{}, nil, celconfig.RuntimeCELCostBudget)
	return results, err
}

// oldestKubernetes is the lowest Kubernetes version in candi/version_map.yml.
func oldestKubernetes() *version.Version {
	raw, err := os.ReadFile("../../../candi/version_map.yml")
	Expect(err).NotTo(HaveOccurred())

	var versionMap struct {
		K8s map[string]any `json:"k8s"`
	}
	Expect(yaml.Unmarshal(raw, &versionMap)).To(Succeed())

	var oldest *version.Version
	for v := range versionMap.K8s {
		parsed := version.MustParseGeneric(v)
		if oldest == nil || parsed.LessThan(oldest) {
			oldest = parsed
		}
	}
	Expect(oldest).NotTo(BeNil())

	return oldest
}

// agentOf is the user of the token of the agent on the node.
func agentOf(node string) user.Info {
	return &user.DefaultInfo{
		Name:  "system:serviceaccount:d8-cloud-instance-manager:fencing-agent",
		Extra: map[string][]string{"authentication.kubernetes.io/node-name": {node}},
	}
}

// testNode is a Node with the labels of fencing 1.0 after the changes.
func testNode(name string, changes ...func(n map[string]any)) map[string]any {
	n := map[string]any{
		"apiVersion": "v1",
		"kind":       "Node",
		"metadata": map[string]any{
			"name": name,
			"labels": map[string]any{
				"kubernetes.io/hostname":                    name,
				"node-manager.deckhouse.io/fencing-enabled": "",
				"node-manager.deckhouse.io/fencing-mode":    "Notify",
			},
		},
		"spec":   map[string]any{"podCIDR": "10.111.1.0/24"},
		"status": map[string]any{},
	}
	for _, change := range changes {
		change(n)
	}

	return n
}

func nodeMetadata(n map[string]any) map[string]any { return n["metadata"].(map[string]any) }

func setLabel(key, value string) func(n map[string]any) {
	return func(n map[string]any) { nodeMetadata(n)["labels"].(map[string]any)[key] = value }
}

func dropLabel(key string) func(n map[string]any) {
	return func(n map[string]any) { delete(nodeMetadata(n)["labels"].(map[string]any), key) }
}

func dropLabels(n map[string]any) { delete(nodeMetadata(n), "labels") }

func annotate(n map[string]any) {
	nodeMetadata(n)["annotations"] = map[string]any{"node-manager.deckhouse.io/fencing-profile": "critical"}
}

func setFencingCondition(n map[string]any) {
	n["status"] = map[string]any{"conditions": []any{map[string]any{"type": "Fencing", "status": "True", "reason": "Armed"}}}
}

func cordon(n map[string]any) { n["spec"].(map[string]any)["unschedulable"] = true }

// testState is a FencingFailedNodeState of the failed node after the changes.
// testNodeGroup is a static NodeGroup named worker; an empty mode leaves out spec.fencing.
func testNodeGroup(mode string) map[string]any {
	spec := map[string]any{"nodeType": "Static"}
	if mode != "" {
		spec["fencing"] = map[string]any{"mode": mode}
	}
	return map[string]any{
		"apiVersion": "deckhouse.io/v1",
		"kind":       "NodeGroup",
		"metadata":   map[string]any{"name": "worker"},
		"spec":       spec,
	}
}

func testState(name string, changes ...func(s map[string]any)) map[string]any {
	s := map[string]any{
		"apiVersion": "node-manager.deckhouse.io/v1alpha1",
		"kind":       "FencingFailedNodeState",
		"metadata":   map[string]any{"name": name},
		"spec": map[string]any{
			"source":     "MemberlistDead",
			"detectedBy": "node-w",
			"detectedAt": "2026-10-29T10:00:00.000000Z",
			"bootID":     "boot-1",
			"armed":      false,
		},
	}
	for _, change := range changes {
		change(s)
	}

	return s
}

func stateStatus(s map[string]any) map[string]any {
	if _, ok := s["status"]; !ok {
		s["status"] = map[string]any{}
	}
	return s["status"].(map[string]any)
}

func heartbeat(at string) func(s map[string]any) {
	return func(s map[string]any) {
		stateStatus(s)["fallback"] = map[string]any{"lastHeartbeatAt": at}
	}
}

func attestation(witness, at string) func(s map[string]any) {
	return func(s map[string]any) {
		stateStatus(s)["attestation"] = map[string]any{"witness": witness, "at": at}
	}
}

func contact(reportedBy, at string) func(s map[string]any) {
	return func(s map[string]any) {
		stateStatus(s)["contact"] = map[string]any{"reportedBy": reportedBy, "at": at}
	}
}

func setPhase(phase string) func(s map[string]any) {
	return func(s map[string]any) { stateStatus(s)["phase"] = phase }
}

func approve(value string) func(s map[string]any) {
	return func(s map[string]any) {
		s["metadata"].(map[string]any)["annotations"] = map[string]any{"node-manager.deckhouse.io/fencing-approve-eviction": value}
	}
}

func setConditions(reason string) func(s map[string]any) {
	return func(s map[string]any) {
		stateStatus(s)["conditions"] = []any{map[string]any{"type": "Queued", "status": "True", "reason": reason}}
	}
}

func setEpisode(episode string) func(s map[string]any) {
	return func(s map[string]any) { stateStatus(s)["correlation"] = map[string]any{"episode": episode} }
}

func episodeLabel(episode string) func(s map[string]any) {
	return func(s map[string]any) {
		s["metadata"].(map[string]any)["labels"] = map[string]any{"node-manager.deckhouse.io/fencing-episode": episode}
	}
}
