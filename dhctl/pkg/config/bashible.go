// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"fmt"
	"os"

	"sigs.k8s.io/yaml"
)

func ParseBashibleConfig(path, specPath string) (map[string]interface{}, error) {
	fileContent, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loading schema file: %v", err)
	}

	schemaStore := NewSchemaStore()
	err = schemaStore.UploadByPath(specPath)
	if err != nil {
		return nil, fmt.Errorf("loading bashible schema: %v", err)
	}

	_, err = schemaStore.Validate(&fileContent)
	if err != nil {
		return nil, fmt.Errorf("config validation: %v", err)
	}

	var data map[string]interface{}
	if err = yaml.Unmarshal(fileContent, &data); err != nil {
		return nil, fmt.Errorf("config unmarshal: %v", err)
	}

	return data, nil
}
