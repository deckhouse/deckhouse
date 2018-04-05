package kube_config_manager

import (
	"fmt"
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/deckhouse/deckhouse/antiopa/kube"
	"github.com/deckhouse/deckhouse/antiopa/utils"
)

var (
	mockSecretList *v1.SecretList
)

type MockSecret struct {
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

func (mockCoreV1 MockCoreV1) Secrets(namespace string) corev1.SecretInterface {
	return MockSecrets{}
}

type MockSecrets struct {
	corev1.SecretInterface
}

func (mockSecrets MockSecrets) List(options metav1.ListOptions) (*v1.SecretList, error) {
	return mockSecretList, nil
}

func (mockSecrets MockSecrets) Get(name string, options metav1.GetOptions) (*v1.Secret, error) {
	for _, v := range mockSecretList.Items {
		if v.Name == name {
			return &v, nil
		}
	}

	return nil, fmt.Errorf("no such secret '%s'", name)
}

func TestInit(t *testing.T) {
	mockSecretList = &v1.SecretList{
		Items: []v1.Secret{
			v1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: kube.AntiopaSecret},
				Data: map[string][]byte{
					GlobalValuesKeyName: []byte(`
project: someproject
clusterName: main
clusterHostname: kube.domain.my
settings:
  count: 2
  mysql:
    user: myuser`),
					"nginxIngress": []byte(`
config:
  hsts: true
  setRealIPFrom:
    - 1.1.1.1
    - 2.2.2.2`),
					"prometheus": []byte(`
adminPassword: qwerty
estimatedNumberOfMetrics: 480000
ingressHostname: prometheus.mysite.com
madisonAuthKey: 70cf58be013c93b5e7960716ea8538eb877808f88303c8a08f18f16582c81b61
retentionDays: 20
userPassword: qwerty`),
					"kubeLego": []byte("false"),
				},
			},
		},
	}

	kube.KubernetesClient = &MockKubernetesClientset{}

	config, err := Init()
	if err != nil {
		t.Errorf("kube_config_manager initialization error: %s", err)
	}

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
