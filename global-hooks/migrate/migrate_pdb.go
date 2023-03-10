/*
Copyright 2023 Flant JSC

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
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	policyv1 "k8s.io/api/policy/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
)

// TODO remove after 1.46 release

// remove helm.sh/hook annotations from PDB to make them compatible with normal helm release

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 10},
}, dependency.WithExternalDependencies(pdbMigration))

func pdbMigration(_ *go_hook.HookInput, dc dependency.Container) error {
	kubeCl, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("cannot init Kubernetes client: %v", err)
	}

	pdbResp, err := kubeCl.PolicyV1().PodDisruptionBudgets("").List(context.TODO(), v1.ListOptions{LabelSelector: "heritage=deckhouse"})
	if err != nil {
		return err
	}

	for _, pdb := range pdbResp.Items {
		if len(pdb.Annotations["helm.sh/hook"]) == 0 && len(pdb.Annotations["helm.sh/hook-delete-policy"]) == 0 {
			continue
		}

		err = patchPDB(kubeCl, pdb)
		if err != nil {
			return err
		}
	}

	return nil
}

func patchPDB(client k8s.Client, pdb policyv1.PodDisruptionBudget) error {
	delete(pdb.Annotations, "helm.sh/hook")
	delete(pdb.Annotations, "helm.sh/hook-delete-policy")

	// resources installed with hook don't have Helm labels, set them
	pdb.Labels["app.kubernetes.io/managed-by"] = "Helm"
	pdb.Annotations["meta.helm.sh/release-name"] = pdb.Labels["module"]
	pdb.Annotations["meta.helm.sh/release-namespace"] = "d8-system"

	_, err := client.PolicyV1().PodDisruptionBudgets(pdb.Namespace).Update(context.TODO(), &pdb, v1.UpdateOptions{})

	return err
}
