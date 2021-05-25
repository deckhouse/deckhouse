package hooks

import (
	"github.com/flant/shell-operator/pkg/kube"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
)

type KubernetesPatch struct {
	*object_patch.ObjectPatcher
}

func NewKubernetesPatch(kubeClient kube.KubernetesClient) *KubernetesPatch {
	op := object_patch.NewObjectPatcher(kubeClient)
	return &KubernetesPatch{op}
}

func (f *KubernetesPatch) Apply(kpBytes []byte) error {
	parsedSpecs, err := object_patch.ParseSpecs(kpBytes)
	if err != nil {
		return err
	}

	return f.ObjectPatcher.GenerateFromJSONAndExecuteOperations(parsedSpecs)
}
