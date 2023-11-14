package controller

import (
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/models/modules"
	"github.com/flant/addon-operator/pkg/utils"
	"github.com/flant/addon-operator/pkg/values/validation"
)

type DeckhouseModule struct {
	basic *modules.BasicModule

	description string
	labels      map[string]string
}

func NewDeckhouseModule(def deckhouseModuleDefinition, staticValues utils.Values, vv *validation.ValuesValidator) *DeckhouseModule {
	basic := modules.NewBasicModule(def.Name, def.Path, def.Weight, staticValues, vv)

	labels := make(map[string]string, len(def.Tags))
	for _, tag := range def.Tags {
		labels["module.deckhouse.io/"+tag] = ""
	}

	if len(def.Tags) == 0 {
		labels = calculateLabels(def.Name)
	}

	return &DeckhouseModule{
		basic:       basic,
		labels:      labels,
		description: def.Description,
	}
}

func (dm DeckhouseModule) AsKubeObject() *v1alpha1.Module {
	return &v1alpha1.Module{
		TypeMeta: metav1.TypeMeta{
			Kind:       v1alpha1.ModuleGVK.Kind,
			APIVersion: v1alpha1.ModuleGVK.Group + "/" + v1alpha1.ModuleGVK.Version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   dm.basic.Name,
			Labels: dm.labels,
		},
		Properties: v1alpha1.ModuleProperties{
			Weight:      dm.basic.Order,
			Source:      "Embedded",
			Description: dm.description,
		},
	}
}

func calculateLabels(name string) map[string]string {
	// could be removed when we will ready properties from the module.yaml file
	labels := make(map[string]string, 0)

	if strings.HasPrefix(name, "cni-") {
		labels["module.deckhouse.io/cni"] = ""
	}

	if strings.HasPrefix(name, "cloud-provider-") {
		labels["module.deckhouse.io/cloud-provider"] = ""
	}

	if strings.HasSuffix(name, "-crd") {
		labels["module.deckhouse.io/crd"] = ""
	}

	return labels
}
