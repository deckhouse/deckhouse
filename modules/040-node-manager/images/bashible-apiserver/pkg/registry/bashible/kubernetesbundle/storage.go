package kubernetesbundle

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"bashible-apiserver/pkg/apis/bashible"
)

// NewStorage returns a RESTStorage object that will work against API services.
func NewEmptyStorage() (*EmptyStorage, error) {
	return &EmptyStorage{}, nil
}

type EmptyStorage struct{}

// Render empty list for KubernetesBundle api, because now we add k8s bundles to node groups
func (s *EmptyStorage) Render(name string) (runtime.Object, error) {
	obj := bashible.KubernetesBundle{}
	obj.ObjectMeta.Name = name
	obj.ObjectMeta.CreationTimestamp = metav1.NewTime(time.Now())
	obj.Data = map[string]string{}

	return &obj, nil
}

func (s *EmptyStorage) New() runtime.Object {
	return &bashible.KubernetesBundle{}
}

func (s *EmptyStorage) NewList() runtime.Object {
	return &bashible.KubernetesBundleList{}
}
