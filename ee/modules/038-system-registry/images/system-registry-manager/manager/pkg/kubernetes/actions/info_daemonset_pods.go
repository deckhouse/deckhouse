/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package actions

import (
	"context"
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pkg_cfg "system-registry-manager/pkg/cfg"
	"time"
)

func GetDaemonsetInfo(namespace, dsName string) (*appsv1.DaemonSet, error) {
	cfg := pkg_cfg.GetConfig()

	ds, err := cfg.K8sClient.AppsV1().DaemonSets(namespace).Get(context.TODO(), dsName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting DaemonSet: %s", err)
	}

	return ds, nil
}

func WaitDaemonsetInfo(namespace, dsName string, cmpFunc func(ds *appsv1.DaemonSet) bool) (*appsv1.DaemonSet, bool, error) {
	for i := 0; i < pkg_cfg.MaxRetries; i++ {
		dsInfo, err := GetDaemonsetInfo(namespace, dsName)
		if err != nil {
			return nil, false, err
		}
		if cmpResult := cmpFunc(dsInfo); cmpResult {
			return dsInfo, cmpResult, nil
		}
		time.Sleep(1 * time.Second)
	}
	return nil, false, nil
}

func DaemonsetCmpFuncEqualDesiredAndReady(ds *appsv1.DaemonSet) bool {
	if ds == nil {
		return false
	}
	return ds.Status.DesiredNumberScheduled == ds.Status.NumberAvailable
}
