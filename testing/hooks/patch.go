package hooks

import (
	klient "github.com/flant/kube-client/client"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
)

type KubernetesPatch struct {
	*object_patch.ObjectPatcher
}

func NewKubernetesPatch(kubeClient klient.Client) *KubernetesPatch {
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
