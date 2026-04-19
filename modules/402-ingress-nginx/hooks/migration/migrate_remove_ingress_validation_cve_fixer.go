/*
Copyright 2025 Flant JSC

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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

// This migration hooks deletes resources related to CVE-2025-1974 fix
// (https://github.com/deckhouse/deckhouse/blob/main/modules/402-ingress-nginx/docs/internal/VALIDATIONS_CVE_FIXUP_RU.md)
// TODO: Remove this migration hook in v1.70+

const (
	ingressNginxValidationCveAnnotation = "igress-nginx.deckhouse.io/cve-2025-1974-fixer-deleted"
	fixerRBACName                       = "d8:ingress-validation-cve-fixer"
	fixerName                           = "d8-ingress-validation-cve-fixer"

	d8SystemNs = "d8-system"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 0},
}, dependency.WithExternalDependencies(removeFixer))

func removeFixer(_ context.Context, _ *go_hook.HookInput, dc dependency.Container) error {
	kubeClient := dc.MustGetK8sClient()

	if err := kubeClient.CoreV1().ServiceAccounts(d8SystemNs).Delete(context.Background(), fixerName, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
		return err
	}

	if err := kubeClient.CoreV1().Secrets(d8SystemNs).Delete(context.Background(), fixerName, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
		return err
	}

	if err := kubeClient.CoreV1().Services(d8SystemNs).Delete(context.Background(), fixerName, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
		return err
	}

	if err := kubeClient.CoreV1().ConfigMaps(d8SystemNs).Delete(context.Background(), fixerName, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
		return err
	}

	if err := kubeClient.AppsV1().Deployments(d8SystemNs).Delete(context.Background(), fixerName, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
		return err
	}

	if err := kubeClient.RbacV1().ClusterRoles().Delete(context.Background(), fixerRBACName, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
		return err
	}

	if err := kubeClient.RbacV1().ClusterRoleBindings().Delete(context.Background(), fixerRBACName, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
		return err
	}

	if err := kubeClient.AdmissionregistrationV1().MutatingWebhookConfigurations().Delete(context.Background(), fixerName+"-hooks", metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
		return err
	}

	return nil
}
