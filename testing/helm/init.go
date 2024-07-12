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
	"strings"

	addonutils "github.com/flant/addon-operator/pkg/utils"
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
	ValuesValidator *values_validation.ValuesValidator
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

	// Discover module path and name: bubble up to the module root, also guard againt filesystem root.
	modulePath := filepath.Dir(wd)
	for filepath.Base(filepath.Dir(modulePath)) != "modules" && filepath.Dir(modulePath) != "/" {
		modulePath = filepath.Dir(modulePath)
	}
	moduleName, err := library.GetModuleNameByPath(modulePath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	moduleName = strcase.ToLowerCamel(moduleName)
	moduleValuesKey := addonutils.ModuleNameToValuesKey(moduleName)

	// Create values structure
	initialValues, err := library.InitValues(modulePath, []byte(values))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defaultConfigValues := addonutils.Values{
		addonutils.GlobalValuesKey: map[string]interface{}{},
		moduleValuesKey:            map[string]interface{}{},
	}
	mergedConfigValues := addonutils.MergeValues(defaultConfigValues, initialValues)
	initialValuesJSON, err := json.Marshal(mergedConfigValues)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	valueValidator, err := values_validation.NewValuesValidator(moduleName, modulePath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Helm config
	config := new(Config)
	config.modulePath = modulePath
	config.moduleName = moduleName
	config.ValuesValidator = valueValidator

	BeforeEach(func() {
		config.values = values_store.NewStoreFromRawJSON(initialValuesJSON)
		config.values.SetByPath("global.discovery.kubernetesVersion", "1.29.1")
	})

	return config
}

func GetModulesImages() map[string]interface{} {
	return map[string]interface{}{
		"registry": map[string]interface{}{
			"base":      "registry.example.com",
			"dockercfg": "Y2ZnCg==",
			"address":   "registry.deckhouse.io",
			"path":      "/deckhouse/fe",
			"CA":        "CACACA",
			"scheme":    "https",
		},
		"digests": library.DefaultImagesDigests,
	}
}

func ManifestStringToUnstructed(doc string) *unstructured.Unstructured {
	var t interface{}
	err := yaml.Unmarshal([]byte(doc), &t)
	if err != nil {
		By("Failed file content:\n" + doc)
	}
	Expect(err).To(Not(HaveOccurred()))
	if t == nil {
		return nil
	}
	Expect(t).To(BeAssignableToTypeOf(map[string]interface{}{}))

	unstructuredObj := &unstructured.Unstructured{}
	unstructuredObj.SetUnstructuredContent(t.(map[string]interface{}))
	return unstructuredObj
}

func (hec *Config) HelmRender(options ...Option) {
	opts := &configOptions{}

	for _, opt := range options {
		opt(opts)
	}

	// set some common values
	hec.values.SetByPath("global.modulesImages.registry.base", "registry.example.com")
	hec.values.SetByPath("global.internal.modules.kubeRBACProxyCA.cert", "test")
	hec.values.SetByPath("global.internal.modules.kubeRBACProxyCA.key", "test")
	hec.values.SetByPathFromYAML("global.modules.placement", []byte("{}"))

	// Validate Helm values
	err := hec.ValuesValidator.ValidateHelmValues(hec.moduleName, string(hec.values.JSONRepr))
	Expect(err).To(Not(HaveOccurred()), "Helm values should conform to the contract in openapi/values.yaml")

	hec.objectStore = make(object_store.ObjectStore)

	yamlValuesBytes := hec.values.GetAsYaml()

	// disable LintMode, otherwise 'fail' function will not render any value
	renderer := helm.Renderer{LintMode: false}
	files, err := renderer.RenderChartFromDir(hec.modulePath, string(yamlValuesBytes))

	hec.RenderError = err

	if files == nil {
		return
	}

	for filePath, manifests := range files {
		if opts.renderedOutput != nil {
			if opts.filterPath != "" {
				if strings.Contains(filePath, opts.filterPath) {
					opts.renderedOutput[filePath] = manifests
				}
			} else {
				opts.renderedOutput[filePath] = manifests
			}
		}
		for _, doc := range releaseutil.SplitManifests(manifests) {
			unstructuredObj := ManifestStringToUnstructed(doc)
			if unstructuredObj == nil {
				continue
			}

			hec.objectStore.PutObject(unstructuredObj.Object, object_store.NewMetaIndex(
				unstructuredObj.GetKind(),
				unstructuredObj.GetNamespace(),
				unstructuredObj.GetName(),
			))
		}
	}
}

type configOptions struct {
	renderedOutput map[string]string
	filterPath     string
}

type Option func(options *configOptions)

// WithRenderOutput output rendered files in a format: $filename: $renderedTemplates (splitted with ---)
func WithRenderOutput(m map[string]string) Option {
	return func(options *configOptions) {
		options.renderedOutput = m
	}
}

// WithFilteredRenderOutput same as WithRenderOutput but filters files which contain `filter` pattern
func WithFilteredRenderOutput(m map[string]string, filter string) Option {
	return func(options *configOptions) {
		options.renderedOutput = m
		options.filterPath = filter
	}
}
