/*
Copyright 2021 Flant CJSC

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
