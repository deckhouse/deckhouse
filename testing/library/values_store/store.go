package values_store

import (
	"encoding/json"
	"fmt"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"

	"gopkg.in/yaml.v3"

	"github.com/tidwall/sjson"

	"github.com/tidwall/gjson"
)

type ValuesStore struct {
	Values   map[string]interface{} // since we aren't operating on concrete types yet, this field remains unused
	JsonRepr []byte
}

func NewStoreFromRawYaml(rawYaml []byte) (*ValuesStore, error) {
	jsonRaw, err := convertYamlToJson(rawYaml)
	if err != nil {
		return nil, err
	}

	return &ValuesStore{
		JsonRepr: jsonRaw,
	}, nil
}

func NewStoreFromRawJson(rawJson []byte) *ValuesStore {
	return &ValuesStore{
		JsonRepr: rawJson,
	}
}

func (store *ValuesStore) Get(path string) gjson.Result {
	return gjson.GetBytes(store.JsonRepr, path)
}

func (store *ValuesStore) GetAsYaml() []byte {
	yamlRaw, err := convertJsonToYaml(store.JsonRepr)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	return yamlRaw
}

func (store *ValuesStore) SetByPath(path string, value interface{}) {
	newValues, err := sjson.SetBytes(store.JsonRepr, path, value)
	if err != nil {
		_, _ = fmt.Fprintf(ginkgo.GinkgoWriter, "failed to set values by path \"%s\": %s\n\nin JSON:\n%s", path, err, store.JsonRepr)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
	}

	store.JsonRepr = newValues

}

func (store *ValuesStore) SetByPathFromYaml(path string, yamlRaw []byte) {
	jsonRaw, err := convertYamlToJson(yamlRaw)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	newValues, err := sjson.SetRawBytes(store.JsonRepr, path, jsonRaw)
	if err != nil {
		_, _ = fmt.Fprintf(ginkgo.GinkgoWriter, "failed to set values by path \"%s\": %s\n\nin JSON:\n%s", path, err, store.JsonRepr)
	}

	store.JsonRepr = newValues
}

func (store *ValuesStore) SetByPathFromJson(path string, jsonRaw []byte) {
	newValues, err := sjson.SetRawBytes(store.JsonRepr, path, jsonRaw)
	if err != nil {
		_, _ = fmt.Fprintf(ginkgo.GinkgoWriter, "failed to set values by path \"%s\": %s\n\nin JSON:\n%s", path, err, store.JsonRepr)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
	}

	store.JsonRepr = newValues
}

func (store *ValuesStore) DeleteByPath(path string) {
	newValues, err := sjson.DeleteBytes(store.JsonRepr, path)
	if err != nil {
		_, _ = fmt.Fprintf(ginkgo.GinkgoWriter, "failed to delete values by path \"%s\": %s\n\nin JSON:\n%s", path, err, store.JsonRepr)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
	}

	store.JsonRepr = newValues
}

func convertYamlToJson(yamlBytes []byte) ([]byte, error) {
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

func convertJsonToYaml(jsonBytes []byte) ([]byte, error) {
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
