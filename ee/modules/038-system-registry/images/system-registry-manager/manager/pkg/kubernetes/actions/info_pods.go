/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package actions

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pkg_cfg "system-registry-manager/pkg/cfg"
)

func GetPodsInfoByLabels(labelSelector []string) (*corev1.PodList, error) {
	cfg := pkg_cfg.GetConfig()

	pods, err := cfg.K8sClient.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{
		LabelSelector: strings.Join(labelSelector, ","),
	})
	if err != nil {
		return nil, fmt.Errorf("error getting PodList: %v", err)
	}

	for _, pod := range pods.Items {
		fmt.Println(pod)
	}
	return pods, nil
}

func WaitAppPodsInfo(labelsSelector []string, cmpFunc func(pods *corev1.PodList) bool) (*corev1.PodList, bool, error) {
	for i := 0; i < pkg_cfg.MaxRetries; i++ {

		podList, err := GetPodsInfoByLabels(labelsSelector)
		if err != nil {
			return nil, false, err
		}

		if cmpFunc(podList) {
			return podList, true, nil
		}

		time.Sleep(1 * time.Second)
	}
	return nil, false, nil
}
