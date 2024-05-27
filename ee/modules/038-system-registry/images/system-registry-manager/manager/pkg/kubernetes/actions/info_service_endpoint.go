/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package actions

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pkg_cfg "system-registry-manager/pkg/cfg"
)

func GetEndpointInfo(namespace, epName string) (*corev1.Endpoints, error) {
	cfg := pkg_cfg.GetConfig()

	ep, err := cfg.K8sClient.CoreV1().Endpoints(namespace).Get(context.TODO(), epName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting Endpoint: %s", err)
	}

	return ep, nil
}

func WaitEndpointInfo(namespace, epName string, cmpFunc func(ep *corev1.Endpoints) bool) (*corev1.Endpoints, bool, error) {
	for i := 0; i < pkg_cfg.MaxRetries; i++ {
		epInfo, err := GetEndpointInfo(namespace, epName)
		if err != nil {
			return nil, false, err
		}
		if cmpFunc(epInfo) {
			return epInfo, true, nil
		}
		time.Sleep(1 * time.Second)
	}
	return nil, false, nil
}

func EndpointCmpNotReadyAddressesEmpty(ep *corev1.Endpoints) bool {
	if ep == nil {
		return false
	}
	for _, subset := range ep.Subsets {
		if len(subset.NotReadyAddresses) != 0 {
			return false
		}
	}
	return true
}
