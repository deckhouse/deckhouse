package deckhouse

import (
	"context"
	"fmt"

	addon_operator_app "github.com/flant/addon-operator/pkg/app"
	klient "github.com/flant/kube-client/client"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/app"
)

func GetCurrentPod(client klient.Client) (pod *v1.Pod, err error) {
	pod, err = client.CoreV1().Pods(addon_operator_app.Namespace).Get(context.TODO(), app.PodName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return pod, nil
}

func GetDeploymentOfCurrentPod(client klient.Client) (deployment *appsv1.Deployment, err error) {
	pod, err := GetCurrentPod(client)
	if err != nil {
		return nil, fmt.Errorf("get current pod: %v", err)
	}

	if len(pod.OwnerReferences) == 0 {
		return nil, fmt.Errorf("current pod has no owner")
	}

	var rs *appsv1.ReplicaSet

	for _, ownerRef := range pod.OwnerReferences {
		if ownerRef.Kind == "ReplicaSet" {
			rs, err = client.AppsV1().ReplicaSets(addon_operator_app.Namespace).Get(context.TODO(), ownerRef.Name, metav1.GetOptions{})
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
		return nil, fmt.Errorf("a ReplicaSet/%s of current pod has no owner", rs.Name)
	}

	for _, ownerRef := range rs.OwnerReferences {
		if ownerRef.Kind == "Deployment" {
			deployment, err = client.AppsV1().Deployments(addon_operator_app.Namespace).Get(context.TODO(), ownerRef.Name, metav1.GetOptions{})
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

func UpdateDeployment(client klient.Client, deployment *appsv1.Deployment) error {
	_, err := client.AppsV1().Deployments(addon_operator_app.Namespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
	switch {
	case errors.IsConflict(err):
		// Deployment is modified in the meanwhile, query the latest version
		// and modify the retrieved object.
		return fmt.Errorf("manifest for Deployment/%s is changed during update: %v", deployment.Name, err)
	case err != nil:
		return err
	default:
		return nil
	}
}
