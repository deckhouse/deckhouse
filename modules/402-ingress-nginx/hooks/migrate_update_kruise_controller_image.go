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

// Update Kruise Controller manager image to a new version beforehand
// so that the old one doesn't update ingress controllers before a new version of Kruise Controller is deployed.

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/ingress-nginx/restart_kruise_controller",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
}, dependency.WithExternalDependencies(restartKruiseControllerDeployment))

const (
	kruisePatchAnnotation = "ingress.deckhouse.io/force-max-unavailable"
	targetNamespace       = "d8-ingress-nginx"
	targetDeployment      = "kruise-controller-manager"
)

func restartKruiseControllerDeployment(input *go_hook.HookInput, dc dependency.Container) error {
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

	// Annotations in place - we don't neet to patch kruise's image
	if _, exists := annotations[kruisePatchAnnotation]; exists {
		return nil
	}

	registry := input.Values.Get("global.modulesImages.registry.base").String()
	if len(registry) == 0 {
		return fmt.Errorf("Hook failed to get registry base")
	}
	kruiseImage := input.Values.Get("global.modulesImages.digests.ingressNginx.kruise").String()
	if len(kruiseImage) == 0 {
		return fmt.Errorf("Hook failed to get kruise image hash")
	}

	// Find kruise container
	for i, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == "kruise" {
			// Check if kruise image needs to be updated
			if container.Image == fmt.Sprintf("%s@%s", registry, kruiseImage) {
				return nil
			}
			deployment.Spec.Template.Spec.Containers[i].Image = fmt.Sprintf("%s@%s", registry, kruiseImage)
			break
		}
	}

	// Update deployment image
	_, err = kubeCl.AppsV1().Deployments(targetNamespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	// Wait up to 2 minutes if new deployment is up
	for i := 0; i < 24; i++ {
		deployment, err := kubeCl.AppsV1().Deployments(targetNamespace).Get(context.TODO(), targetDeployment, metav1.GetOptions{})
		if err != nil {
			input.LogEntry.Warnf("Hook failed to get %s/%s deployment", targetNamespace, targetDeployment)
			// If deployment is up, annotate it with kruisePatchAnnotation and exit
		} else if *deployment.Spec.Replicas == deployment.Status.ReadyReplicas {
			deployment, err := kubeCl.AppsV1().Deployments(targetNamespace).Get(context.TODO(), targetDeployment, metav1.GetOptions{})
			if err != nil {
				return err
			}

			if annotations == nil {
				annotations = make(map[string]string)
			}

			annotations[kruisePatchAnnotation] = time.Now().Format(time.RFC3339)
			deployment.ObjectMeta.SetAnnotations(annotations)
			_, err = kubeCl.AppsV1().Deployments(targetNamespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
			return nil
		}
		time.Sleep(5 * time.Second)
	}

	// Deployment failed to update - return error
	return fmt.Errorf("Hook waiting for %s/%s deployment timed out", targetNamespace, targetDeployment)
}
