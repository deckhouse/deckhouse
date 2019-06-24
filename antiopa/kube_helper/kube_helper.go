package kube_helper

import (
	"fmt"
	"os"
	"regexp"

	"github.com/romana/rlog"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kube_client "github.com/flant/addon-operator/pkg/kube"
)

const (
	AntiopaDefaultNamespace = "antiopa"
	AntiopaDeploymentName   = "antiopa"
	AntiopaContainerName    = "antiopa"
)

var (
	KubernetesClient kubernetes.Interface
	AntiopaNamespace string
)

func Init() {

	AntiopaNamespace = os.Getenv("ANTIOPA_NAMESPACE")
	if AntiopaNamespace == "" {
		if kube_client.AddonOperatorNamespace != "" {
			AntiopaNamespace = kube_client.AddonOperatorNamespace
		} else {
			AntiopaNamespace = AntiopaDefaultNamespace
		}
	}

	KubernetesClient = kube_client.Kubernetes

	rlog.Infof("ANTIOPA: Use namespace '%s'", AntiopaNamespace)
}

// Возвращает image — имя образа (адрес регистри/репо:тэг) и imageID.
// imageID может быть такого вида:
//  "imageID": "docker-pullable://registry.flant.com/sys/antiopa/dev@sha256:05f5cc14dff4fcc3ff3eb554de0e550050e65c968dc8bbc2d7f4506edfcdc5b6"
//  "imageID": "docker://sha256:e537460dd124f6db6656c1728a42cf8e268923ff52575504a471fa485c2a884a"
func KubeGetPodImageInfo(podName string) (imageName string, imageId string) {
	res, err := KubernetesClient.CoreV1().Pods(AntiopaNamespace).Get(podName, metav1.GetOptions{})

	if err != nil {
		rlog.Debugf("KUBE Cannot get info for pod %s! %v", podName, err)
		return "", ""
	}

	// Get image name from container spec. ContainerStatus contains bad name
	// if multiple tags has one digest!
	// https://github.com/kubernetes/kubernetes/issues/51017
	for _, spec := range res.Spec.Containers {
		if spec.Name == AntiopaContainerName {
			imageName = spec.Image
			break
		}
	}

	for _, status := range res.Status.ContainerStatuses {
		if status.Name == AntiopaContainerName {
			imageId = status.ImageID
			break
		}
	}

	return
}

// KubeUpdateDeployment - меняет лейбл antiopaImageName на новый id образа antiopa
// тем самым заставляя kubernetes обновить Pod.
func KubeUpdateDeployment(imageId string) error {
	deploymentsClient := KubernetesClient.AppsV1beta1().Deployments(AntiopaNamespace)

	res, err := deploymentsClient.Get(AntiopaDeploymentName, metav1.GetOptions{})

	if err != nil {
		return fmt.Errorf("Cannot get antiopa deployment! %v", err)
	}

	res.Spec.Template.Labels["antiopaImageId"] = NormalizeLabelValue(imageId)

	if _, err := deploymentsClient.Update(res); errors.IsConflict(err) {
		// Deployment is modified in the meanwhile, query the latest version
		// and modify the retrieved object.
		return fmt.Errorf("Manifest changed during update: %v", err)
	} else if err != nil {
		return err
	}

	return nil
}

var NonSafeCharsRegexp = regexp.MustCompile(`[^a-zA-Z0-9]`)

func NormalizeLabelValue(value string) string {
	newVal := NonSafeCharsRegexp.ReplaceAllLiteralString(value, "_")
	labelLen := len(newVal)
	if labelLen > 63 {
		labelLen = 63
	}
	return newVal[:labelLen]
}
