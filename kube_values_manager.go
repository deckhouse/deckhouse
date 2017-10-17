package main

import (
	_ "crypto/md5"
	_ "encoding/hex"
	_ "encoding/json"
	"fmt"
	_ "time"

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
	<module-name>-checksum: <checksum-of-values-yaml> // устанавливается самой antiopa
*/

var (
	KubeValuesUpdated       chan map[string]interface{}
	KubeModuleValuesUpdated chan KubeModuleValuesUpdate

	valuesChecksum       string
	moduleValuesChecksum map[string]string
)

type KubeModuleValuesUpdate struct {
	ModuleName string
	Values     map[string]interface{}
}

func getConfigMap2() (*v1.ConfigMap, error) {
	configMap, err := KubernetesClient.CoreV1().ConfigMaps(KubernetesAntiopaNamespace).Get("antiopa", meta_v1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("ConfigMap '%s' is not found in namespace '%s'", "antiopa", KubernetesAntiopaNamespace)
	}

	return configMap, nil
}

func SetModuleKubeValues(ModuleName string, Values map[string]interface{}) {
	/*
	* Читаем текущий ConfigMap
	* Обновляем <module-name>-values + <module-name>-checksum (md5 от yaml-values)
	 */
}

func InitKubeValuesManager() (struct {
	Values        map[string]interface{}
	ModulesValues map[string]map[string]interface{}
}, error) {
	rlog.Info("Init kube values manager")

	/*
		* Достать через kubernetes-api текущую версию ConfigMap по api
		* запоминаем в глобальную переменную resourceVersion
		* читаем values из этого ConfigMap
			* Прочесть текущий values
			* Запомнить в переменной (глобальной) valuesChecksum md5 от yaml-строки, которую достали из ConfigMap
		* читаем все module values из этого же ConfigMap
			* Прочесть все остальные <module-name>-values (которые найдутся в configmap'е)
			* Запомнить в переменной (глобальной) moduleValuesChecksum[module-name] md5 от yaml-строки, которую достали из ConfigMap
		* Метод возвращает текущие значения values и modules-values разом. Любая возникающая ошибка тоже сразу возвращается.
	*/

	return struct {
		Values        map[string]interface{}
		ModulesValues map[string]map[string]interface{}
	}{make(map[string]interface{}), make(map[string]map[string]interface{})}, nil
}

func RunKubeValuesManager() {
	rlog.Info("Run config manager")

	/*
		Это горутина, поэтому в цикле.
		Long-polling через kubernetes-api через watch-запрос (* https://v1-6.docs.kubernetes.io/docs/api-reference/v1.6/#watch-199)
		* Делаем watch-запрос на ресурс ConfigMap, указывая в параметре resourceVersion известную нам версию из глобальной переменной
		* Указыаем в watch-запрос timeout в 15сек. Т.к. любой http-запрос имеет timeout, то это просто означает что надо повторить запрос по http.
		* Если resource-version поменялся, то kubernetes возвращает какой-то ответ с новым ресурсом
			* запоминаем в глобальную переменную новый resourceVersion
			* читаем values из этого ConfigMap
				* Считаем md5 от yaml-строки, если поменялась, то обновляем глобальную переменную и генерим сигнал в KubeValuesUpdated
			* читаем все module values из этого же ConfigMap, для каждого
				* Считаем md5 от yaml-строки -> фактический хэш
				* Если фактический хэш совпадает с <module-name>-checksum => не делаем ничего
				* Если фактический хэш не совпадает с <module-name>-checksum
					* Если фактический хэш не совпадает с moduleValuesChecksum[module-name]
						* Обновляем moduleValuesChecksum[module-name], генерим сигнал KubeModuleValuesUpdated
				* Считаем md5 от yaml-строки, если поменялась, то обновляем глобальную переменную и генерим сигнал в ModuleValuesUpdate
	*/
}
