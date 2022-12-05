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

package values_store

import (
	"encoding/json"
	"fmt"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"gopkg.in/yaml.v3"

	"github.com/deckhouse/deckhouse/testing/library"
)

type ValuesStore struct {
	Values   map[string]interface{} // since we aren't operating on concrete types yet, this field remains unused
	JSONRepr []byte
}

func NewStoreFromRawYaml(rawYaml []byte) (*ValuesStore, error) {
	jsonRaw, err := ConvertYAMLToJSON(rawYaml)
	if err != nil {
		return nil, err
	}

	return &ValuesStore{
		JSONRepr: jsonRaw,
	}, nil
}

func NewStoreFromRawJSON(rawJSON []byte) *ValuesStore {
	return &ValuesStore{
		JSONRepr: rawJSON,
	}
}

func (store *ValuesStore) Get(path string) library.KubeResult {
	gjsonResult := gjson.GetBytes(store.JSONRepr, path)
	kubeResult := library.KubeResult{Result: gjsonResult}
	return kubeResult
}

func (store *ValuesStore) GetAsYaml() []byte {
	yamlRaw, err := ConvertJSONToYAML(store.JSONRepr)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	return yamlRaw
}

func (store *ValuesStore) SetByPath(path string, value interface{}) {
	newValues, err := sjson.SetBytes(store.JSONRepr, path, value)
	if err != nil {
		_, _ = fmt.Fprintf(ginkgo.GinkgoWriter, "failed to set values by path \"%s\": %s\n\nin JSON:\n%s", path, err, store.JSONRepr)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
	}

	store.JSONRepr = newValues
}

func (store *ValuesStore) SetByPathFromYAML(path string, yamlRaw []byte) {
	jsonRaw, err := ConvertYAMLToJSON(yamlRaw)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	newValues, err := sjson.SetRawBytes(store.JSONRepr, path, jsonRaw)
	if err != nil {
		_, _ = fmt.Fprintf(ginkgo.GinkgoWriter, "failed to set values by path \"%s\": %s\n\nin JSON:\n%s", path, err, store.JSONRepr)
	}

	store.JSONRepr = newValues
}

func (store *ValuesStore) SetByPathFromJSON(path string, jsonRaw []byte) {
	newValues, err := sjson.SetRawBytes(store.JSONRepr, path, jsonRaw)
	if err != nil {
		_, _ = fmt.Fprintf(ginkgo.GinkgoWriter, "failed to set values by path \"%s\": %s\n\nin JSON:\n%s", path, err, store.JSONRepr)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
	}

	store.JSONRepr = newValues
}

func (store *ValuesStore) DeleteByPath(path string) {
	newValues, err := sjson.DeleteBytes(store.JSONRepr, path)
	if err != nil {
		_, _ = fmt.Fprintf(ginkgo.GinkgoWriter, "failed to delete values by path \"%s\": %s\n\nin JSON:\n%s", path, err, store.JSONRepr)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
	}

	store.JSONRepr = newValues
}

func ConvertYAMLToJSON(yamlBytes []byte) ([]byte, error) {
	var obj interface{}

	err := yaml.Unmarshal(yamlBytes, &obj)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML:%s\n\n%s", err, yamlBytes)
	}

	jsonBytes, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON:%s\n\n%+v", err, obj)
	}

	return jsonBytes, nil
}

func ConvertJSONToYAML(jsonBytes []byte) ([]byte, error) {
	var obj interface{}

	err := json.Unmarshal(jsonBytes, &obj)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON:%s\n\n%s", err, jsonBytes)
	}

	yamlBytes, err := yaml.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to marshol YAML:%s\n\n%+v", err, obj)
	}

	return yamlBytes, nil
}
