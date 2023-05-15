package apis

import (
	"github.com/flant/addon-operator/pkg/module_manager"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/modules/002-deckhouse/hooks/pkg/apis/v1alpha1"
)

type ModuleProducer struct {
}

func NewModuleProducer() *ModuleProducer {
	return &ModuleProducer{}
}

func (mp *ModuleProducer) GetGVK() schema.GroupVersionKind {
	return v1alpha1.ModuleGVK
}

func (mp *ModuleProducer) NewModule() module_manager.ModuleObject {
	return mp.newModule()
}

func (mp *ModuleProducer) newModule() *v1alpha1.Module {
	return &v1alpha1.Module{
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
}
