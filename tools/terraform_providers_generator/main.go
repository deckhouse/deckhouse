package main

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type ossItem struct {
	ID      string `yaml:"id"`
	Version string `yaml:"version"`
}

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("failed to get cwd: %v", err)
	}
	rootPath := filepath.Dir(cwd)

	versions, err := loadOSSVersions(rootPath)
	if err != nil {
		log.Fatalf("failed to load oss versions: %v", err)
	}

	targetFile := filepath.Join(rootPath, "candi", "terraform_versions.yml")
	data, err := os.ReadFile(targetFile)
	if err != nil {
		log.Fatalf("failed to read target file: %v", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		log.Fatalf("failed to parse yaml: %v", err)
	}

	updateNodes(doc.Content[0], versions)

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		log.Fatalf("failed to encode yaml: %v", err)
	}
	enc.Close()

	if err := os.WriteFile(targetFile, buf.Bytes(), 0o644); err != nil {
		log.Fatalf("failed to write file: %v", err)
	}
}

func loadOSSVersions(root string) (map[string][]string, error) {
	out := make(map[string][]string)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, "modules/040-terraform-manager/oss.yaml") {
			return err
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		var items []ossItem
		if err := yaml.Unmarshal(data, &items); err != nil {
			return err
		}

		for _, item := range items {
			if item.Version != "" {
				out[item.ID] = append(out[item.ID], item.Version)
			}
		}
		return nil
	})
	return out, err
}

func updateNodes(root *yaml.Node, versions map[string][]string) {
	for i := 0; i < len(root.Content); i += 2 {
		keyNode := root.Content[i]
		valNode := root.Content[i+1]

		if keyNode.Value == "terraform" || keyNode.Value == "opentofu" {
			if v, ok := versions[keyNode.Value]; ok && len(v) > 0 {
				valNode.Value = v[0]
				valNode.Style = yaml.DoubleQuotedStyle
				valNode.Tag = "!!str"
			}
			continue
		}

		if valNode.Kind != yaml.MappingNode {
			continue
		}

		typeNode := findKey(valNode, "type")
		if typeNode == nil {
			continue
		}

		ossID := "terraform-provider-" + typeNode.Value
		vers, ok := versions[ossID]
		if !ok || len(vers) == 0 {
			continue
		}

		if len(vers) == 1 {
			setKey(valNode, "version", strNode(vers[0]))
			removeKey(valNode, "versions")
		} else {
			seq := &yaml.Node{Kind: yaml.SequenceNode}
			for _, v := range vers {
				seq.Content = append(seq.Content, strNode(v))
			}
			setKey(valNode, "versions", seq)
			removeKey(valNode, "version")
		}
	}
}

func strNode(val string) *yaml.Node {
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: val,
		Style: yaml.DoubleQuotedStyle,
	}
}

func findKey(node *yaml.Node, key string) *yaml.Node {
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func setKey(node *yaml.Node, key string, val *yaml.Node) {
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			node.Content[i+1] = val
			return
		}
	}
	node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: key}, val)
}

func removeKey(node *yaml.Node, key string) {
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			node.Content = append(node.Content[:i], node.Content[i+2:]...)
			return
		}
	}
}
