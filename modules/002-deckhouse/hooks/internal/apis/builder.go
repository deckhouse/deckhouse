package apis

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/modules/002-deckhouse/hooks/internal/apis/v1alpha1"
)

type ModuleBuilder struct {
}

func NewModuleBuilder() *ModuleBuilder {
	return &ModuleBuilder{}
}

func (mb *ModuleBuilder) GetGVK() schema.GroupVersionKind {
	return v1alpha1.ModuleGVK
}

func (mb *ModuleBuilder) NewModuleTemplate() *v1alpha1.Module {
	m := &v1alpha1.Module{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.ModuleGVK.GroupVersion().String(),
			Kind:       v1alpha1.ModuleGVK.Kind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   "",
			Labels: make(map[string]string),
		},
		Properties: v1alpha1.ModuleProperties{
			Weight: 0,
		},
		Status: v1alpha1.ModuleStatus{},
	}

	return m
}
