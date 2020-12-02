package helm

import (
	"encoding/json"
	"path/filepath"
	"runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/flant/shell-operator/pkg/utils/manifest/releaseutil"

	"github.com/deckhouse/deckhouse/testing/library"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
	"github.com/deckhouse/deckhouse/testing/library/values_store"
	"github.com/deckhouse/deckhouse/testing/util/helm"
)

type Config struct {
	modulePath  string
	objectStore object_store.ObjectStore
	values      *values_store.ValuesStore
	RenderError error
}

func (hec Config) ValuesGet(path string) library.KubeResult {
	return hec.values.Get(path)
}

func (hec *Config) ValuesSet(path string, value interface{}) {
	hec.values.SetByPath(path, value)
}

func (hec *Config) ValuesSetFromYaml(path, value string) {
	hec.values.SetByPathFromYAML(path, []byte(value))
}

func (hec *Config) KubernetesGlobalResource(kind, name string) object_store.KubeObject {
	return hec.objectStore.KubernetesGlobalResource(kind, name)
}

func (hec *Config) KubernetesResource(kind, namespace, name string) object_store.KubeObject {
	return hec.objectStore.KubernetesResource(kind, namespace, name)
}

func SetupHelmConfig(values string) *Config {
	_, path, _, ok := runtime.Caller(1)
	if !ok {
		panic("can't execute runtime.Caller")
	}

	modulePath := filepath.Dir(filepath.Dir(path))

	initialValues, err := library.InitValues(modulePath, []byte(values))
	if err != nil {
		panic(err)
	}

	config := new(Config)
	config.modulePath = modulePath

	initialValuesJSON, err := json.Marshal(initialValues)
	if err != nil {
		panic(err)
	}

	BeforeEach(func() {
		config.values = values_store.NewStoreFromRawJSON(initialValuesJSON)

		// set some common values
		config.values.SetByPath("global.discovery.kubernetesVersion", "1.17.0")
		config.values.SetByPath("global.modulesImages.registry", "registry.example.com")
	})

	return config
}

func (hec *Config) HelmRender() {
	hec.objectStore = make(object_store.ObjectStore)

	yamlValuesBytes := hec.values.GetAsYaml()

	var renderer helm.Renderer
	files, err := renderer.RenderChartFromDir(hec.modulePath, string(yamlValuesBytes))

	hec.RenderError = err

	if files == nil {
		return
	}

	for _, manifests := range files {
		for _, doc := range releaseutil.SplitManifests(manifests) {
			var t interface{}
			err = yaml.Unmarshal([]byte(doc), &t)

			Expect(err).To(Not(HaveOccurred()))
			if t == nil {
				continue
			}
			Expect(t).To(BeAssignableToTypeOf(map[string]interface{}{}))

			var unstructuredObj unstructured.Unstructured
			unstructuredObj.SetUnstructuredContent(t.(map[string]interface{}))

			hec.objectStore.PutObject(unstructuredObj.Object, object_store.NewMetaIndex(
				unstructuredObj.GetKind(),
				unstructuredObj.GetNamespace(),
				unstructuredObj.GetName(),
			))
		}
	}
}
