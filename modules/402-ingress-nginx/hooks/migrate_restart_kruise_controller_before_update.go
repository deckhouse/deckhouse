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
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

// TODO: remove this hook after Deckhouse 1.50

// Scale in Kruise Controller manager to zero before update Ingress-Nginx module
// so that it doesn't update ingress controllers before a new version of Kruise Controller is deployed.

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/ingress-nginx/restart_kruise_controller",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
}, dependency.WithExternalDependencies(restartKruiseControllerDeployment))

const (
	kruisePatchAnnotation = "ingress.deckhouse.io/force-max-unavailable"
	restartAnnotation     = "ingress.deckhouse.io/restartedAt"
	targetNamespace       = "d8-ingress-nginx"
	targetDeployment      = "kruise-controller-manager"
)

func restartKruiseControllerDeployment(_ *go_hook.HookInput, dc dependency.Container) error {
	kubeCl, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("cannot init Kubernetes client: %v", err)
	}
	deployment, err := kubeCl.AppsV1().Deployments(targetNamespace).Get(context.TODO(), targetDeployment, metav1.GetOptions{})

	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	annotations := deployment.ObjectMeta.GetAnnotations()

	if _, exists := annotations[kruisePatchAnnotation]; exists {
		return nil
	}

	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[kruisePatchAnnotation] = ""

	templateAnnotations := deployment.Spec.Template.ObjectMeta.GetAnnotations()
	if templateAnnotations == nil {
		templateAnnotations = make(map[string]string)
	}
	templateAnnotations[restartAnnotation] = time.Now().Format(time.RFC3339)

	deployment.ObjectMeta.SetAnnotations(annotations)
	deployment.Spec.Template.ObjectMeta.SetAnnotations(templateAnnotations)

	_, err = kubeCl.AppsV1().Deployments(targetNamespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}
