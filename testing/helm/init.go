/*
Copyright 2021 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package helm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	addonutils "github.com/flant/addon-operator/pkg/utils"
	"github.com/flant/addon-operator/pkg/values/validation"
	"github.com/flant/kube-client/manifest/releaseutil"
	"github.com/iancoleman/strcase"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/testing/library"
	"github.com/deckhouse/deckhouse/testing/library/helm"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
	"github.com/deckhouse/deckhouse/testing/library/values_store"
	"github.com/deckhouse/deckhouse/testing/library/values_validation"
)

type Config struct {
	moduleName      string
	modulePath      string
	objectStore     object_store.ObjectStore
	values          *values_store.ValuesStore
	RenderError     error
	ValuesValidator *validation.ValuesValidator
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
	wd, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	modulePath := filepath.Dir(wd)

	moduleName, err := library.GetModuleNameByPath(modulePath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	moduleName = strcase.ToLowerCamel(moduleName)
	moduleValuesKey := addonutils.ModuleNameToValuesKey(moduleName)

	defaultConfigValues := addonutils.Values{
		addonutils.GlobalValuesKey: map[string]interface{}{},
		moduleValuesKey:            map[string]interface{}{},
	}
	initialValues, err := library.InitValues(modulePath, []byte(values))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	mergedConfigValues := addonutils.MergeValues(defaultConfigValues, initialValues)

	config := new(Config)
	config.modulePath = modulePath
	config.moduleName = moduleName

	initialValuesJSON, err := json.Marshal(mergedConfigValues)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	config.ValuesValidator = validation.NewValuesValidator()

	if err := values_validation.LoadOpenAPISchemas(config.ValuesValidator, moduleName, modulePath); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	BeforeEach(func() {
		config.values = values_store.NewStoreFromRawJSON(initialValuesJSON)

		// set some common values
		config.values.SetByPath("global.discovery.kubernetesVersion", "1.22.0")
		config.values.SetByPath("global.modulesImages.registry", "registry.example.com")
		config.values.SetByPathFromYAML("global.modules.placement", []byte("{}"))
	})

	return config
}

func GetModulesImages() map[string]interface{} {
	tags, err := library.GetModulesImagesTags("")
	if err != nil {
		panic(err)
	}
	return map[string]interface{}{
		"registry":          "registry.example.com",
		"registryDockercfg": "Y2ZnCg==",
		"tags":              tags,
	}
}

func (hec *Config) HelmRender() {
	// Validate Helm values
	err := values_validation.ValidateHelmValues(hec.ValuesValidator, hec.moduleName, string(hec.values.JSONRepr))
	Expect(err).To(Not(HaveOccurred()), "Helm values should conform to the contract in openapi/values.yaml")

	hec.objectStore = make(object_store.ObjectStore)

	yamlValuesBytes := hec.values.GetAsYaml()

	renderer := helm.Renderer{LintMode: true}
	files, err := renderer.RenderChartFromDir(hec.modulePath, string(yamlValuesBytes))

	hec.RenderError = err

	if files == nil {
		return
	}

	for _, manifests := range files {
		for _, doc := range releaseutil.SplitManifests(manifests) {
			var t interface{}
			err = yaml.Unmarshal([]byte(doc), &t)

			if err != nil {
				By("Doc\n:" + doc)
			}
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
