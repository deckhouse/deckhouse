package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/romana/rlog"

	v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

/* Формат values:
data:
	values: |
		<values-yaml>
	<module-name>-values: |
		<values-yaml>
	...
  	<module-name>-values: |
		<values-yaml>
*/

var (
	ModulesUpdated chan []map[string]string
)

func getConfigMap() (*v1.ConfigMap, error) {
	configMap, err := KubernetesClient.CoreV1().ConfigMaps(KubernetesAntiopaNamespace).Get("antiopa", meta_v1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("ConfigMap '%s' is not found in namespace '%s'", "antiopa", KubernetesAntiopaNamespace)
	}

	return configMap, nil
}

func InitConfigManager() {
	rlog.Info("Init config manager")
}

func RunConfigManager() {
	rlog.Info("Run config manager")
}
