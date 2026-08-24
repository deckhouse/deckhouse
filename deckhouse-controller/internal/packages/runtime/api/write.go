// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"sigs.k8s.io/yaml"
)

const (
	// outputParam selects the response format.
	outputParam = "output"
	// outputJSON is the default: the API answers machines first.
	outputJSON = "json"
	// outputYAML is for reading a dump by eye.
	outputYAML = "yaml"
)

// Write sends value in the format the output query parameter asks for: JSON when
// it is absent or "json", YAML when it is "yaml". Any other value is refused, so
// a typo does not silently return the default.
func Write(w http.ResponseWriter, req *http.Request, value any) {
	output := req.URL.Query().Get(outputParam)

	var (
		data        []byte
		err         error
		contentType string
	)

	switch output {
	case "", outputJSON:
		data, err = json.Marshal(value)
		contentType = "application/json"
	case outputYAML:
		data, err = yaml.Marshal(value)
		contentType = "application/yaml"
	default:
		http.Error(w, fmt.Sprintf("unknown output '%s', want json or yaml", output), http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, fmt.Sprintf("marshal response: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
