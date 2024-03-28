/*
Copyright 2024 Flant JSC

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
	"github.com/deckhouse/deckhouse/go_lib/dependency"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"golang.org/x/mod/semver"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

const clusterAdminsGroupAndClusterRoleBinding = "kubeadm:cluster-admins"

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 15},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "crb",
			ApiVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"kubeadm:cluster-admins"},
			},
			ExecuteHookOnSynchronization: pointer.Bool(false),
			ExecuteHookOnEvents:          pointer.Bool(false),
			FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
				return obj.GetName(), nil
			},
		},
	},
}, dependency.WithExternalDependencies(k8sPostUpgrade))

func k8sPostUpgrade(input *go_hook.HookInput, dc dependency.Container) error {
	if len(input.Snapshots["crb"]) > 0 {
		// We need this hook to run only once
		return nil
	}

	kubernetesVersion := fmt.Sprintf("v%s", input.Values.Get("global.discovery.kubernetesVersion"))

	// if kubernetesVersion < v1.29.0
	if semver.Compare("v1.29.0", kubernetesVersion) == 1 {
		return nil
	}

	kubeCl, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("cannot init Kubernetes client: %v", err)
	}

	clusterRoleBinding := &rbac.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterAdminsGroupAndClusterRoleBinding,
		},
		RoleRef: rbac.RoleRef{
			APIGroup: rbac.GroupName,
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
		},
		Subjects: []rbac.Subject{
			{
				Kind: rbac.GroupKind,
				Name: clusterAdminsGroupAndClusterRoleBinding,
			},
		},
	}

	input.LogEntry.Printf("create clusterrolebinding: %s", clusterAdminsGroupAndClusterRoleBinding)

	_, err = kubeCl.RbacV1().ClusterRoleBindings().Create(context.TODO(), clusterRoleBinding, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("error create clusterrolebinding: %s: %v", clusterAdminsGroupAndClusterRoleBinding, err)
	}

	return nil
}
