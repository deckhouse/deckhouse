// Copyright 2026 Flant JSC
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

// Package respond writes debug endpoint responses.
package respond

import (
	"encoding/json"
	"fmt"
	"net/http"

	"sigs.k8s.io/yaml"
)

const (
	// formatYAML is the default response format.
	formatYAML = "yaml"
	// formatJSON is selected by the format query parameter.
	formatJSON = "json"
)

// Dump sends value in the format the format query parameter asks for: YAML when
// it is absent or "yaml", JSON when it is "json".
func Dump(w http.ResponseWriter, req *http.Request, value any) {
	switch format := req.URL.Query().Get("format"); format {
	case "", formatYAML:
		YAMLValue(w, value)
	case formatJSON:
		JSON(w, value)
	default:
		http.Error(w, fmt.Sprintf("unknown format '%s', want yaml or json", format), http.StatusBadRequest)
	}
}

// JSON marshals value and sends it as JSON.
func JSON(w http.ResponseWriter, value any) {
	data, err := json.Marshal(value)
	if err != nil {
		http.Error(w, fmt.Sprintf("marshal response: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// YAML sends an already marshalled YAML payload.
func YAML(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Type", "application/yaml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// YAMLValue marshals value and sends it as YAML.
func YAMLValue(w http.ResponseWriter, value any) {
	data, err := yaml.Marshal(value)
	if err != nil {
		http.Error(w, fmt.Sprintf("marshal response: %v", err), http.StatusInternalServerError)
		return
	}

	YAML(w, data)
}

// Text sends a plain text payload.
func Text(w http.ResponseWriter, text string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(text))
}
