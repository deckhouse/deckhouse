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

	"gopkg.in/yaml.v3"
)

func ConvertYAMLToJSON(yamlBytes []byte) ([]byte, error) {
	var obj interface{}

	err := yaml.Unmarshal(yamlBytes, &obj)
	if err != nil {
		return nil, err
	}

	jsonBytes, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}

	return jsonBytes, nil
}

func ConvertJSONToYAML(jsonBytes []byte) ([]byte, error) {
	var obj interface{}

	err := json.Unmarshal(jsonBytes, &obj)
	if err != nil {
		return nil, err
	}

	yamlBytes, err := yaml.Marshal(obj)
	if err != nil {
		return nil, err
	}

	return yamlBytes, nil
}
