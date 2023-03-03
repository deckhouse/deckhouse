package gatekeeper

import (
	"context"

	controllerClient "sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

const (
	mutationsGroup        = "mutations.gatekeeper.sh"
	mutationsGroupVersion = "v1"
	mutationsGV           = mutationsGroup + "/" + mutationsGroupVersion
)

type MutationMeta struct {
	Kind string
	Name string
}

type MutationSpec struct {
	Match Match `json:"match"`
}

type Mutation struct {
	Meta MutationMeta
	Spec MutationSpec
}

func (m Mutation) GetMatchKinds() []MatchKind {
	return m.Spec.Match.Kinds
}

func GetMutations(cClient controllerClient.Client, client *kubernetes.Clientset) ([]Mutation, error) {
	c, err := client.ServerResourcesForGroupVersion(mutationsGV)
	if err != nil {
		return nil, err
	}

	var mutations []Mutation
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
			Group:   mutationsGroup,
			Kind:    r.Kind,
			Version: mutationsGroupVersion,
		})

		err = cClient.List(context.TODO(), actual)
		if err != nil {
			return nil, err
		}

		if len(actual.Items) > 0 {
			for _, item := range actual.Items {
				var mutation Mutation

				err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), &mutation)
				if err != nil {
					klog.Error(err)
					continue
				}

				mutation.Meta.Kind = item.GetKind()
				mutation.Meta.Name = item.GetName()

				mutations = append(mutations, mutation)
			}
		}

	}
	return mutations, nil
}
