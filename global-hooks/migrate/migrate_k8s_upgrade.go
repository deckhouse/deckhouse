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

/*
This migration creates a ClusterRoleBinding kubeadm:cluster-admins after upgrading to kubernetes 1.29.
*/

package hooks

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/modules/040-control-plane-manager/hooks"
)

const clusterAdminsGroupAndClusterRoleBinding = "kubeadm:cluster-admins"

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:              "clusterConfiguration",
			ApiVersion:        "v1",
			Kind:              "Secret",
			NamespaceSelector: &types.NamespaceSelector{NameSelector: &types.NameSelector{MatchNames: []string{"kube-system"}}},
			NameSelector:      &types.NameSelector{MatchNames: []string{"d8-cluster-configuration"}},
			FilterFunc:        k8sUpgradeHookTriggerFilter,
		},
	},
}, dependency.WithExternalDependencies(k8sPostUpgrade))

// Required to run the hook when the k8s version has been changed
func k8sUpgradeHookTriggerFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, err
	}
	version, ok := secret.Data["maxUsedControlPlaneKubernetesVersion"]
	if !ok {
		return nil, nil
	}
	return version, nil
}

type clusterConfig struct {
	KubernetesVersion string `yaml:"kubernetesVersion"`
}

func k8sPostUpgrade(input *go_hook.HookInput, dc dependency.Container) error {
	kubeCl, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("cannot init Kubernetes client: %v", err)
	}

	_, err = kubeCl.RbacV1().ClusterRoleBindings().Get(context.TODO(), clusterAdminsGroupAndClusterRoleBinding, metav1.GetOptions{})
	// if kubeadm:cluster-admins ClusterRoleBinding exists
	if err == nil {
		return nil
	}

	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("error: %v", err)
	}

	secret, err := kubeCl.CoreV1().Secrets("kube-system").Get(context.TODO(), "d8-cluster-configuration", metav1.GetOptions{})
	// In managed clusters we do not have d8-cluster-configuration secret
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("error get d8-cluster-configuration secret: %s", err.Error())
	}

	var config clusterConfig
	err = yaml.Unmarshal(secret.Data["cluster-configuration.yaml"], &config)
	if err != nil {
		return fmt.Errorf("unmarshal 'cluster-configuration.yaml' failed: %w", err)
	}

	var kubernetesVersion = hooks.DefaultKubernetesVersion

	if config.KubernetesVersion != "Automatic" {
		kubernetesVersion = config.KubernetesVersion
	}

	input.Logger.Info("detected Kubernetes version", slog.String("version", kubernetesVersion))

	c, err := semver.NewConstraint("< 1.29")
	if err != nil {
		return fmt.Errorf("constraint not being parsable: %s", err.Error())
	}
	v, err := semver.NewVersion(kubernetesVersion)
	if err != nil {
		return fmt.Errorf("version %q not being parsable: %s", kubernetesVersion, err.Error())
	}
	// if kubernetesVersion < v1.29.0
	if c.Check(v) {
		return nil
	}

	clusterRoleBinding := &rbac.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterAdminsGroupAndClusterRoleBinding,
			Labels: map[string]string{
				"heritage": "deckhouse",
			},
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

	input.Logger.Info("creating clusterrolebinding", slog.String("name", clusterAdminsGroupAndClusterRoleBinding))

	_, err = kubeCl.RbacV1().ClusterRoleBindings().Create(context.TODO(), clusterRoleBinding, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("error create clusterrolebinding: %s: %v", clusterAdminsGroupAndClusterRoleBinding, err)
	}

	return nil
}
