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

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/pruning"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	sigsyaml "sigs.k8s.io/yaml"
)

const speKind = "SecurityPolicyException"

// gator loads inventory objects as they are, without the CRD schema the API
// server would apply. A SecurityPolicyException fixture that the API server
// would reject can therefore make a case pass against a policy that no real
// exception is able to satisfy. Every SecurityPolicyException in the inventory
// of a generated suite is validated against the module CRD for this reason.
// A field the schema does not define fails the check as well: the API server
// prunes it silently, so a misspelled field would be absent in a cluster.
func validateSuiteSPEInventory(suite suiteOut, baseDir, matrixDir string) error {
	chartRoot := findConstraintTemplatesChartRoot(matrixDir)
	if chartRoot == "" {
		return fmt.Errorf("cannot locate the constraint-templates chart root from %s", matrixDir)
	}
	crdPath := filepath.Join(chartRoot, "..", "..", "crds", "security-policy-exception.yaml")

	paths := map[string]struct{}{}
	for _, tb := range suite.Tests {
		for _, c := range tb.Cases {
			for _, p := range c.Inventory {
				paths[p] = struct{}{}
			}
		}
	}
	sorted := make([]string, 0, len(paths))
	for p := range paths {
		sorted = append(sorted, p)
	}
	sort.Strings(sorted)

	var validators map[string]crdSchema
	var problems []string
	for _, rel := range sorted {
		docs, err := readYAMLDocuments(filepath.Join(baseDir, filepath.FromSlash(rel)))
		if err != nil {
			return fmt.Errorf("inventory %s: %w", rel, err)
		}
		for _, doc := range docs {
			if doc["kind"] != speKind {
				continue
			}
			if validators == nil {
				if validators, err = loadCRDValidators(crdPath); err != nil {
					return err
				}
			}
			version := strings.TrimPrefix(fmt.Sprint(doc["apiVersion"]), "deckhouse.io/")
			schema, ok := validators[version]
			if !ok {
				problems = append(problems, fmt.Sprintf("%s: unknown %s version %q", rel, speKind, version))
				continue
			}
			for _, e := range validation.ValidateCustomResource(nil, doc, schema.validator) {
				problems = append(problems, fmt.Sprintf("%s: %s", rel, e.Error()))
			}
			unknown := pruning.PruneWithOptions(doc, schema.structural, true, structuralschema.UnknownFieldPathOptions{TrackUnknownFieldPaths: true})
			for _, path := range unknown {
				problems = append(problems, fmt.Sprintf("%s: %s: field not declared in the schema, the API server would drop it", rel, path))
			}
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("%s inventory does not match the CRD %s:\n  %s", speKind, crdPath, strings.Join(problems, "\n  "))
	}
	return nil
}

type crdSchema struct {
	validator  validation.SchemaValidator
	structural *structuralschema.Structural
}

func loadCRDValidators(crdPath string) (map[string]crdSchema, error) {
	data, err := os.ReadFile(crdPath)
	if err != nil {
		return nil, fmt.Errorf("read CRD: %w", err)
	}
	var crd apiextensionsv1.CustomResourceDefinition
	if err := sigsyaml.Unmarshal(data, &crd); err != nil {
		return nil, fmt.Errorf("parse CRD %s: %w", crdPath, err)
	}
	validators := map[string]crdSchema{}
	for _, v := range crd.Spec.Versions {
		if v.Schema == nil || v.Schema.OpenAPIV3Schema == nil {
			continue
		}
		schema := new(apiextensions.JSONSchemaProps)
		if err := apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(v.Schema.OpenAPIV3Schema, schema, nil); err != nil {
			return nil, fmt.Errorf("convert CRD %s schema %s: %w", crdPath, v.Name, err)
		}
		validator, _, err := validation.NewSchemaValidator(schema)
		if err != nil {
			return nil, fmt.Errorf("build validator for CRD %s schema %s: %w", crdPath, v.Name, err)
		}
		structural, err := structuralschema.NewStructural(schema)
		if err != nil {
			return nil, fmt.Errorf("build structural schema for CRD %s schema %s: %w", crdPath, v.Name, err)
		}
		validators[v.Name] = crdSchema{validator: validator, structural: structural}
	}
	return validators, nil
}

func readYAMLDocuments(path string) ([]map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var docs []map[string]interface{}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	for {
		var node yaml.Node
		if err := dec.Decode(&node); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		raw, err := yaml.Marshal(&node)
		if err != nil {
			return nil, err
		}
		// Round-trip through JSON so the validator receives the same value types
		// (float64 numbers, map[string]interface{}) the API server decodes into.
		var doc map[string]interface{}
		if err := sigsyaml.Unmarshal(raw, &doc); err != nil {
			return nil, err
		}
		if doc != nil {
			docs = append(docs, doc)
		}
	}
	return docs, nil
}
