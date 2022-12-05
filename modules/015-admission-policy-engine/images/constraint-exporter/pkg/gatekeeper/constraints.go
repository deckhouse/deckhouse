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

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	controllerClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// ConstraintMeta represents meta information of a constraint
type ConstraintMeta struct {
	Kind string
	Name string
}

// Violation represents each constraintViolation under status
type Violation struct {
	Kind              string `json:"kind"`
	Name              string `json:"name"`
	Namespace         string `json:"namespace,omitempty"`
	Message           string `json:"message"`
	EnforcementAction string `json:"enforcementAction"`
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

// ConstraintSpec collect general information about the overall constraints applied to the cluster
type ConstraintSpec struct {
	EnforcementAction string          `json:"enforcementAction"`
	Match             ConstraintMatch `json:"match"`
}

type ConstraintMatch struct {
	Kinds []MatchKind `json:"kinds"`
}

type MatchKind struct {
	APIGroups []string `json:"apiGroups"`
	Kinds     []string `json:"kinds"`
}

const (
	constraintsGV           = "constraints.gatekeeper.sh/v1beta1"
	constraintsGroup        = "constraints.gatekeeper.sh"
	constraintsGroupVersion = "v1beta1"
)

func createKubeClientGroupVersion(config *rest.Config) (controllerClient.Client, error) {
	client, err := controllerClient.New(config, controllerClient.Options{})
	if err != nil {
		return nil, err
	}

	return client, nil
}

// GetConstraints returns a list of all OPA constraints
func GetConstraints(config *rest.Config, client *kubernetes.Clientset) ([]Constraint, error) {
	cClient, err := createKubeClientGroupVersion(config)
	if err != nil {
		return nil, err
	}

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
					klog.Error(err)
					continue
				}

				constraint.Meta.Kind = item.GetKind()
				constraint.Meta.Name = item.GetName()

				constraints = append(constraints, constraint)
			}
		}

	}
	return constraints, nil
}
