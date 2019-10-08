package antiopa

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/apps/v1beta1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	client "github.com/flant/shell-operator/pkg/kube"

	addon_operator_app "github.com/flant/addon-operator/pkg/app"

	"github.com/deckhouse/deckhouse/antiopa/pkg/app"
)

func GetCurrentPod() (pod *v1.Pod, err error) {
	pod, err = client.Kubernetes.CoreV1().Pods(addon_operator_app.Namespace).Get(app.PodName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return pod, nil
}

func GetDeploymentOfCurrentPod() (deployment *v1beta1.Deployment, err error) {
	pod, err := GetCurrentPod()
	if err != nil {
		return nil, fmt.Errorf("get current pod: %v", err)
	}

	if len(pod.OwnerReferences) == 0 {
		return nil, fmt.Errorf("current pod has no owner")
	}

	var rs *appsv1.ReplicaSet

	for _, ownerRef := range pod.OwnerReferences {
		if ownerRef.Kind == "ReplicaSet" {
			rs, err = client.Kubernetes.AppsV1().ReplicaSets(addon_operator_app.Namespace).Get(ownerRef.Name, metav1.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("get ReplicaSet of current pod: %v", err)
			}
			break
		}
	}

	if rs == nil {
		return nil, fmt.Errorf("no ReplicaSet found for current pod")
	}

	if len(rs.OwnerReferences) == 0 {
		return nil, fmt.Errorf("ReplicaSet/%s of current pod has no owner", rs.Name)
	}

	for _, ownerRef := range rs.OwnerReferences {
		if ownerRef.Kind == "Deployment" {
			deployment, err = client.Kubernetes.AppsV1beta1().Deployments(addon_operator_app.Namespace).Get(ownerRef.Name, metav1.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("get Deployment of current pod: %v", err)
			}
			break
		}
	}

	if deployment == nil {
		return nil, fmt.Errorf("no Deployment found for current pod")
	}

	return deployment, nil
}

func UpdateDeployment(deployment *v1beta1.Deployment) error {
	_, err := client.Kubernetes.AppsV1beta1().Deployments(addon_operator_app.Namespace).Update(deployment)
	switch {
	case errors.IsConflict(err):
		// Deployment is modified in the meanwhile, query the latest version
		// and modify the retrieved object.
		return fmt.Errorf("Deployment/%s manifest changed during update: %v", deployment.Name, err)
	case err != nil:
		return err
	default:
		return nil
	}
}
