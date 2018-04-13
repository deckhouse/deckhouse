package kube_config_manager

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes"
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/deckhouse/deckhouse/antiopa/kube"
	"github.com/deckhouse/deckhouse/antiopa/utils"
)

var (
	mockConfigMapList *v1.ConfigMapList
)

type MockConfigMap struct {
	ObjectMeta struct {
		Name string
	}
	Data map[string]string
}

type MockKubernetesClientset struct {
	kubernetes.Clientset
}

func (client *MockKubernetesClientset) CoreV1() corev1.CoreV1Interface {
	return MockCoreV1{}
}

type MockCoreV1 struct {
	corev1.CoreV1Interface
}

func (mockCoreV1 MockCoreV1) ConfigMaps(namespace string) corev1.ConfigMapInterface {
	return MockConfigMaps{}
}

type MockConfigMaps struct {
	corev1.ConfigMapInterface
}

func (mockConfigMaps MockConfigMaps) List(options metav1.ListOptions) (*v1.ConfigMapList, error) {
	return mockConfigMapList, nil
}

func (mockConfigMaps MockConfigMaps) Get(name string, options metav1.GetOptions) (*v1.ConfigMap, error) {
	for _, v := range mockConfigMapList.Items {
		if v.Name == name {
			return &v, nil
		}
	}

	return nil, fmt.Errorf("no such resource '%s'", name)
}

func (mockConfigMaps MockConfigMaps) Create(obj *v1.ConfigMap) (*v1.ConfigMap, error) {
	mockConfigMapList.Items = append(mockConfigMapList.Items, *obj)
	return obj, nil
}

func (mockConfigMaps MockConfigMaps) Update(obj *v1.ConfigMap) (*v1.ConfigMap, error) {
	for ind, v := range mockConfigMapList.Items {
		if v.Name == obj.Name {
			mockConfigMapList.Items[ind] = *obj
			return obj, nil
		}
	}

	return nil, fmt.Errorf("no such resource '%s'", obj.Name)
}

func TestInit(t *testing.T) {
	mockConfigMapList = &v1.ConfigMapList{
		Items: []v1.ConfigMap{
			v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: kube.AntiopaConfigMap},
				Data: map[string]string{
					GlobalValuesKeyName: `
project: someproject
clusterName: main
clusterHostname: kube.domain.my
settings:
  count: 2
  mysql:
    user: myuser`,
					"nginxIngress": `
config:
  hsts: true
  setRealIPFrom:
    - 1.1.1.1
    - 2.2.2.2`,
					"prometheus": `
adminPassword: qwerty
estimatedNumberOfMetrics: 480000
ingressHostname: prometheus.mysite.com
madisonAuthKey: 70cf58be013c93b5e7960716ea8538eb877808f88303c8a08f18f16582c81b61
retentionDays: 20
userPassword: qwerty`,
					"kubeLego": "false",
				},
			},
		},
	}

	kube.KubernetesClient = &MockKubernetesClientset{}

	kcm, err := Init()
	if err != nil {
		t.Errorf("kube_config_manager initialization error: %s", err)
	}
	config := kcm.InitialConfig()

	expectedData := map[string]utils.Values{
		"global": utils.Values{
			"project":         "someproject",
			"clusterName":     "main",
			"clusterHostname": "kube.domain.my",
			"settings": map[string]interface{}{
				"count": 2.0,
				"mysql": map[string]interface{}{
					"user": "myuser",
				},
			},
		},
		"nginxIngress": utils.Values{
			"config": map[string]interface{}{
				"hsts": true,
				"setRealIPFrom": []interface{}{
					"1.1.1.1",
					"2.2.2.2",
				},
			},
		},
		"prometheus": utils.Values{
			"adminPassword":            "qwerty",
			"estimatedNumberOfMetrics": 480000.0,
			"ingressHostname":          "prometheus.mysite.com",
			"madisonAuthKey":           "70cf58be013c93b5e7960716ea8538eb877808f88303c8a08f18f16582c81b61",
			"retentionDays":            20.0,
			"userPassword":             "qwerty",
		},
	}

	for key, data := range expectedData {
		if key == "global" {
			if !reflect.DeepEqual(data, config.Values) {
				t.Errorf("Bad global values: expected %v, got %v", data, config.Values)
			}
		} else {
			moduleName := key
			moduleConfig, hasKey := config.ModuleConfigs[moduleName]
			if !hasKey {
				t.Errorf("Expected module %s values to be existing in config", moduleName)
			}
			if moduleConfig.ModuleName != moduleName {
				t.Errorf("Expected %s module name, got %s", moduleName, moduleConfig.ModuleName)
			}
			if !moduleConfig.IsEnabled {
				t.Errorf("Expected %s module to be enabled", moduleConfig.ModuleName)
			}
			if !reflect.DeepEqual(data, moduleConfig.Values) {
				t.Errorf("Bad %s module values: expected %+v, got %+v", moduleConfig.ModuleName, data, moduleConfig.Values)
			}
		}
	}

	for moduleName, moduleConfig := range config.ModuleConfigs {
		if _, hasKey := expectedData[moduleName]; hasKey {
			continue
		}

		if moduleConfig.ModuleName != moduleName {
			t.Errorf("Expected %s module name in index, got %s", moduleName, moduleConfig.ModuleName)
		}

		if moduleConfig.IsEnabled {
			t.Errorf("Expected %s module to be disabled", moduleConfig.ModuleName)
		}
	}
}

func findCurrentConfigMap() *v1.ConfigMap {
	for _, cm := range mockConfigMapList.Items {
		if cm.Name == "antiopa" {
			return &cm
		}
	}

	return nil
}

func configRawDataShouldEqual(expectedData map[string]string) error {
	obj := findCurrentConfigMap()
	if obj == nil {
		return fmt.Errorf("expected ConfigMap 'antiopa' to be existing")
	}

	if !reflect.DeepEqual(obj.Data, expectedData) {
		return fmt.Errorf("expected %+v ConfigMap data, got %+v", expectedData, obj.Data)
	}

	return nil
}

func convertToConfigData(input map[string]utils.Values) (map[string]string, error) {
	res := make(map[string]string)
	for k, v := range input {
		yamlData, err := yaml.Marshal(v)
		if err != nil {
			return nil, err
		}
		res[k] = string(yamlData)
	}

	return res, nil
}

func configDataShouldEqual(expectedData map[string]utils.Values) error {
	expectedDataRaw, err := convertToConfigData(expectedData)
	if err != nil {
		return err
	}
	return configRawDataShouldEqual(expectedDataRaw)
}

func TestSetConfig(t *testing.T) {
	mockConfigMapList = &v1.ConfigMapList{}
	kube.KubernetesClient = &MockKubernetesClientset{}
	kcm := &MainKubeConfigManager{}

	var err error

	err = kcm.SetKubeValues(utils.Values{
		"mysql": map[string]interface{}{
			"username": "root",
			"password": "password",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = configDataShouldEqual(map[string]utils.Values{
		"global": utils.Values{
			"mysql": map[string]interface{}{
				"username": "root",
				"password": "password",
			},
		},
	})
	if err != nil {
		t.Error(err)
	}

	err = kcm.SetKubeValues(utils.Values{
		"mysql": map[string]interface{}{
			"username": "root",
			"password": "password",
		},
		"mongo": map[string]interface{}{
			"username": "root",
			"password": "password",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = configDataShouldEqual(map[string]utils.Values{
		"global": utils.Values{
			"mysql": map[string]interface{}{
				"username": "root",
				"password": "password",
			},
			"mongo": map[string]interface{}{
				"username": "root",
				"password": "password",
			},
		},
	})
	if err != nil {
		t.Error(err)
	}

	err = kcm.SetModuleKubeValues("mymodule", utils.Values{
		"one": 1,
		"two": 2,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = configDataShouldEqual(map[string]utils.Values{
		"global": utils.Values{
			"mysql": map[string]interface{}{
				"username": "root",
				"password": "password",
			},
			"mongo": map[string]interface{}{
				"username": "root",
				"password": "password",
			},
		},
		"mymodule": utils.Values{
			"one": 1,
			"two": 2,
		},
	})
	if err != nil {
		t.Error(err)
	}

}
