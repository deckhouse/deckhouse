/*
Copyright 2022 Flant JSC

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

package gatekeeper

import (
	"context"
	"log/slog"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	controllerClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// ConstraintMeta represents meta information of a constraint
type ConstraintMeta struct {
	Kind string
	Name string
	// D8 source type for constaint. for example: PSS (pod security standard), OperationPolicy
	SourceType string
	// PolicyKind and PolicyName point at the policy resource whose status collects the violations
	// of this constraint. They come from the labels the module puts on every constraint it renders.
	// PolicyName is empty for a constraint that belongs to the module itself rather than to a
	// policy, and such a constraint contributes to no status.
	PolicyKind string
	PolicyName string
}

// Violation represents each constraintViolation under status
type Violation struct {
	Kind              string `json:"kind"`
	Name              string `json:"name"`
	Namespace         string `json:"namespace,omitempty"`
	Message           string `json:"message"`
	EnforcementAction string `json:"enforcementAction"`
}

// ConstraintSpec collect general information about the overall constraints applied to the cluster
type ConstraintSpec struct {
	EnforcementAction string `json:"enforcementAction"`
	Match             Match  `json:"match"`
}

type ConstraintStatus struct {
	TotalViolations float64 `json:"totalViolations"`
	Violations      []*Violation
}

type Constraint struct {
	Meta   ConstraintMeta
	Spec   ConstraintSpec
	Status ConstraintStatus
}

func (c Constraint) GetMatchKinds() []MatchKind {
	return c.Spec.Match.Kinds
}

type Match struct {
	Kinds []MatchKind `json:"kinds"`
}

type MatchKind struct {
	APIGroups []string `json:"apiGroups"`
	Kinds     []string `json:"kinds"`
}

const (
	constraintsGroup        = "constraints.gatekeeper.sh"
	constraintsGroupVersion = "v1beta1"
	constraintsGV           = constraintsGroup + "/" + constraintsGroupVersion

	podStandardLabel     = "security.deckhouse.io/pod-standard"
	operationPolicyLabel = "security.deckhouse.io/operation-policy"
	securityPolicyLabel  = "security.deckhouse.io/security-policy"

	// SecurityPolicyKind and OperationPolicyKind name the Deckhouse resources that own constraints.
	SecurityPolicyKind  = "SecurityPolicy"
	OperationPolicyKind = "OperationPolicy"
)

// pssPolicyName returns the name of the SecurityPolicy the module renders for a standard.
// It has to match the name in templates/policies/pod-security-standards/security-policy.yaml.
func pssPolicyName(standard string) string {
	if standard == "" {
		return ""
	}
	return "d8-pod-security-" + standard
}

// GetConstraints returns a list of all OPA constraints
func GetConstraints(cClient controllerClient.Client, client *kubernetes.Clientset) ([]Constraint, error) {
	c, err := client.ServerResourcesForGroupVersion(constraintsGV)
	if err != nil {
		return nil, err
	}

	var constraints []Constraint
	for _, r := range c.APIResources {
		canList := false
		for _, verb := range r.Verbs {
			if verb == "list" {
				canList = true
				break
			}
		}

		if !canList {
			continue
		}
		actual := &unstructured.UnstructuredList{}
		actual.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   constraintsGroup,
			Kind:    r.Kind,
			Version: constraintsGroupVersion,
		})

		err = cClient.List(context.TODO(), actual)
		if err != nil {
			return nil, err
		}

		if len(actual.Items) > 0 {
			for _, item := range actual.Items {
				var constraint Constraint

				err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), &constraint)
				if err != nil {
					slog.Error("decode constraint failed", "kind", item.GetKind(), "name", item.GetName(), "error", err)
					continue
				}

				constraint.Meta.Kind = item.GetKind()
				constraint.Meta.Name = item.GetName()

				labels := item.GetLabels()
				f := func(key string) bool { _, ok := labels[key]; return ok }

				switch {
				case f(podStandardLabel):
					constraint.Meta.SourceType = "PSS"
					constraint.Meta.PolicyKind = SecurityPolicyKind
					// The module renders one SecurityPolicy per standard to hold its violations.
					constraint.Meta.PolicyName = pssPolicyName(labels[podStandardLabel])

				case f(operationPolicyLabel):
					constraint.Meta.SourceType = "OperationPolicy"
					constraint.Meta.PolicyKind = OperationPolicyKind
					constraint.Meta.PolicyName = labels[operationPolicyLabel]

				case f(securityPolicyLabel):
					constraint.Meta.SourceType = "SecurityPolicy"
					constraint.Meta.PolicyKind = SecurityPolicyKind
					constraint.Meta.PolicyName = labels[securityPolicyLabel]
				}

				constraints = append(constraints, constraint)
			}
		}
	}
	return constraints, nil
}
