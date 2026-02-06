/*
Copyright 2025 Flant JSC

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

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type ossItem struct {
	ID       string   `yaml:"id"`
	Version  string   `yaml:"version"`
	Versions []string `yaml:"versions"`
}

func main() {
	cwd, _ := os.Getwd()
	rootPath := filepath.Dir(cwd)

	tfVersions := filepath.Join(rootPath, "candi", "terraform_versions.yml")

	ossFiles, err := findOssFiles(rootPath)
	if err != nil {
		panic(err)
	}

	versionsByID, err := loadOssVersions(ossFiles)
	if err != nil {
		panic(err)
	}

	content, err := os.ReadFile(tfVersions)
	if err != nil {
		panic(fmt.Errorf("cannot read terraform providers file: %w", err))
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(content, &doc); err != nil {
		panic(fmt.Errorf("cannot parse terraform providers yaml: %w", err))
	}

	rootNode := doc.Content[0]
	applyVersions(rootNode, versionsByID)

	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(&doc); err != nil {
		panic(fmt.Errorf("cannot encode terraform providers yaml: %w", err))
	}
	if err := encoder.Close(); err != nil {
		panic(err)
	}

	if err := os.WriteFile(tfVersions, buf.Bytes(), 0o644); err != nil {
		panic(fmt.Errorf("cannot write terraform providers file: %w", err))
	}
}

func findOssFiles(root string) ([]string, error) {
	var files []string
	needle := filepath.Join("modules", "040-terraform-manager", "oss.yaml")

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, needle) {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(files)
	return files, nil
}

func loadOssVersions(paths []string) (map[string][]string, error) {
	versionsByID := make(map[string][]string)

	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("cannot read oss file %s: %w", path, err)
		}

		var items []ossItem
		if err := yaml.Unmarshal(content, &items); err != nil {
			return nil, fmt.Errorf("cannot parse oss file %s: %w", path, err)
		}

		for _, item := range items {
			versions := item.Versions
			if len(versions) == 0 {
				versions = []string{item.Version}
			}

			versionsByID[item.ID] = append(versionsByID[item.ID], versions...)
		}
	}

	return versionsByID, nil
}

func applyVersions(root *yaml.Node, versionsByID map[string][]string) {
	for i := 0; i < len(root.Content); i += 2 {
		valueNode := root.Content[i+1]
		if valueNode.Kind != yaml.MappingNode {
			continue
		}

		typeNode := mappingValue(valueNode, "type")
		if typeNode == nil {
			continue
		}

		providerType := typeNode.Value
		ossID := "terraform-provider-" + providerType
		versions := versionsByID[ossID]

		if len(versions) == 1 {
			setMapping(valueNode, "version", scalarNode(versions[0]))
			deleteMappingKey(valueNode, "versions")
			continue
		}

		setMapping(valueNode, "versions", sequenceNode(versions))
		deleteMappingKey(valueNode, "version")
	}
}

func mappingIndex(node *yaml.Node, key string) int {
	if node == nil || node.Kind != yaml.MappingNode {
		return -1
	}

	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return i
		}
	}

	return -1
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	index := mappingIndex(node, key)
	if index == -1 {
		return nil
	}
	return node.Content[index+1]
}

func setMapping(node *yaml.Node, key string, value *yaml.Node) {
	if node == nil || node.Kind != yaml.MappingNode {
		return
	}

	index := mappingIndex(node, key)
	if index != -1 {
		overwriteNode(node.Content[index+1], value)
		return
	}

	node.Content = append(node.Content,
		scalarNode(key),
		value,
	)
}

func deleteMappingKey(node *yaml.Node, key string) {
	if node == nil || node.Kind != yaml.MappingNode {
		return
	}

	index := mappingIndex(node, key)
	if index == -1 {
		return
	}

	node.Content = append(node.Content[:index], node.Content[index+2:]...)
}

func overwriteNode(dst, src *yaml.Node) {
	dst.Kind = src.Kind
	dst.Tag = src.Tag
	dst.Value = src.Value
	dst.Content = src.Content
	dst.Style = src.Style
}

func scalarNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

func sequenceNode(values []string) *yaml.Node {
	seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, value := range values {
		seq.Content = append(seq.Content, scalarNode(value))
	}
	return seq
}
