package actions

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pkg_cfg "system-registry-manager/pkg/cfg"
)

func GetEndpointInfo(namespace, endpointName string) (*corev1.Endpoints, error) {
	cfg := pkg_cfg.GetConfig()

	endpoint, err := cfg.K8sClient.CoreV1().Endpoints(namespace).Get(context.TODO(), endpointName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting Endpoint: %s", err)
	}

	return endpoint, nil
}

func WaitEndpointInfo(namespace, endpointName string, cmpFunc func(endpoint *corev1.Endpoints) bool) (*corev1.Endpoints, bool, error) {
	for i := 0; i < pkg_cfg.MaxRetries; i++ {
		endpointInfo, err := GetEndpointInfo(namespace, endpointName)
		if err != nil {
			return nil, false, err
		}
		if cmpFunc(endpointInfo) {
			return endpointInfo, true, nil
		}
		time.Sleep(1 * time.Second)
	}
	return nil, false, nil
}

func EndpointCmpFuncEqualDesiredAndReady(endpoint *corev1.Endpoints, addressesCount int) bool {
	if endpoint == nil {
		return false
	}

	if len(endpoint.Subsets) == 0 {
		return false
	}
	return len(endpoint.Subsets[0].Addresses) == addressesCount
}
