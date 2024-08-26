package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apimachineryYaml "k8s.io/apimachinery/pkg/util/yaml"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func (m *moduleGenerator) parseCRDs(ctx context.Context) (map[string][]string, map[string][]string, error) {
	var manageResources, useResources = make(map[string][]string), make(map[string][]string)
	for _, crd := range m.crds {
		if match := strings.HasPrefix(filepath.Base(crd), "doc-"); match {
			continue
		}
		parsed, err := m.processFile(ctx, crd)
		if err != nil {
			return nil, nil, err
		}
		if len(parsed) != 0 {
			for _, res := range parsed {
				if res.Spec.Scope == "Cluster" {
					manageResources[res.Spec.Group] = append(manageResources[res.Spec.Group], res.Spec.Names.Plural)
				} else {
					useResources[res.Spec.Group] = append(useResources[res.Spec.Group], res.Spec.Names.Plural)
				}
			}
		}
	}
	return manageResources, useResources, nil
}
func (m *moduleGenerator) processFile(ctx context.Context, path string) ([]*v1.CustomResourceDefinition, error) {
	fileReader, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fileReader.Close()
	var crds []*v1.CustomResourceDefinition
	reader := apimachineryYaml.NewDocumentDecoder(fileReader)
	for {
		n, err := reader.Read(m.buffer)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		data := m.buffer[:n]
		if len(data) == 0 {
			// some empty yaml document, or empty string before separator
			continue
		}
		crd, err := m.parseCRD(ctx, bytes.NewReader(data), n)
		if err != nil {
			return nil, err
		}
		if crd != nil {
			crds = append(crds, crd)
		}
	}
	return crds, nil
}
func (m *moduleGenerator) parseCRD(_ context.Context, reader io.Reader, bufferSize int) (*v1.CustomResourceDefinition, error) {
	var crd *v1.CustomResourceDefinition
	if err := apimachineryYaml.NewYAMLOrJSONDecoder(reader, bufferSize).Decode(&crd); err != nil {
		return nil, err
	}
	// it could be a comment or some other peace of yaml file, skip it
	if crd == nil {
		return nil, nil
	}
	if crd.APIVersion != v1.SchemeGroupVersion.String() && crd.Kind != "CustomResourceDefinition" {
		return nil, fmt.Errorf("invalid CRD document apiversion/kind: '%s/%s'", crd.APIVersion, crd.Kind)
	}
	if crd.Spec.Group != "deckhouse.io" {
		if m.allowResource(crd.Spec.Group, crd.Spec.Names.Plural) {
			return crd, nil
		}
		return nil, nil
	}
	if slices.Contains(m.forbiddenResources, crd.Spec.Names.Plural) {
		return nil, nil
	}
	return crd, nil
}
