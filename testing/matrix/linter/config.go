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

package linter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gammazero/deque"
	"github.com/mohae/deepcopy"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/chartutil"

	"github.com/deckhouse/deckhouse/testing/matrix/linter/utils"
)

const (
	// Variations add ability to create variants for tests matrix
	ConstantVariation string = "__ConstantChoices__"
	RangeVariation    string = "__RangeChoices__"

	// Item works like variation arguments to include special variants of values
	EmptyItem string = "__EmptyItem__"
)

type FileController struct {
	Module utils.Module
	Prefix string
	TmpDir string
	Queue  *deque.Deque[any]
}

type Node struct {
	Keys []interface{}
	Item interface{}
}

func NewNode(item interface{}) Node {
	return Node{Item: item}
}

func LoadConfiguration(m utils.Module, path, prefix, dir string) (FileController, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return FileController{}, fmt.Errorf("formatting path failed: %v", err)
	}

	configurationFile, err := os.ReadFile(absPath)
	if err != nil {
		return FileController{}, fmt.Errorf("read matrix tests configuration file failed: %v", err)
	}

	result := make(map[string]interface{})

	err = yaml.Unmarshal(configurationFile, result)
	if err != nil {
		return FileController{}, fmt.Errorf("configuration unmarshalling error: %v", err)
	}

	filesQueue := deque.Deque[any]{}
	filesQueue.PushBack(result)

	if prefix == "" {
		prefix = "values"
	}

	if dir == "" {
		dir = os.TempDir()
	} else {
		dir, err = filepath.Abs(dir)
		if err != nil {
			return FileController{}, fmt.Errorf("saving values failed: %v", err)
		}
	}
	dir = strings.TrimSuffix(dir, "/")
	_ = os.Mkdir(dir, 0755)

	tmpDir, err := os.MkdirTemp(dir, "")
	if err != nil {
		return FileController{}, fmt.Errorf("tmp directory error: %v", err)
	}
	return FileController{Module: m, Queue: &filesQueue, Prefix: prefix, TmpDir: tmpDir}, nil
}

func findVariations(nodeData interface{}) ([]interface{}, []interface{}) {
	queue := deque.Deque[any]{}
	queue.PushBack(NewNode(nodeData))

	for queue.Len() > 0 {
		tempNode := queue.PopFront().(Node)

		switch data := tempNode.Item.(type) {
		case map[string]interface{}:
			for key, value := range data {
				if key == ConstantVariation || key == RangeVariation {
					return tempNode.Keys, value.([]interface{})
				}
				copiedKeys := deepcopy.Copy(append(tempNode.Keys, key)).([]interface{})

				queue.PushBack(Node{Keys: copiedKeys, Item: value})
			}
		case []interface{}:
			for index, value := range data {
				copiedKeys := deepcopy.Copy(append(tempNode.Keys, index)).([]interface{})

				queue.PushBack(Node{Keys: copiedKeys, Item: value})
			}
		}
	}
	return nil, nil
}

func (f *FileController) FindAll() {
	var file interface{}
	counter := 0

	for f.Queue.Len() > counter {
		file = f.Queue.PopFront()

		keys, values := findVariations(file)
		if keys == nil {
			counter++
			f.Queue.PushBack(file)
			continue
		}

		for _, item := range values {
			f.Queue.PushBack(formatFile(deepcopy.Copy(file), keys, item, 0))
		}
	}
}

func formatFile(file interface{}, keys []interface{}, resultItem interface{}, counter int) interface{} {
	key := keys[counter]

	switch f := file.(type) {
	case map[string]interface{}:
		strKey := key.(string)
		if len(keys)-1 == counter {
			if resultItem == EmptyItem {
				delete(f, strKey)
			} else {
				f[strKey] = resultItem
			}
		} else {
			// Recursive call, need to be fixed
			counter++
			f[strKey] = formatFile(f[strKey], keys, resultItem, counter)
		}

		return f

	case []interface{}:
		intKey := key.(int)

		if len(keys)-1 == counter {
			if resultItem == EmptyItem {
				// Delete an element from array
				f[intKey] = f[len(f)-1]
				f[len(f)-1] = ""
				f = f[:len(f)-1]
			} else {
				f[intKey] = resultItem
			}
		} else {
			// Recursive call, need to be fixed
			counter++
			f[intKey] = formatFile(f[intKey], keys, resultItem, counter)
		}

		return f
	}

	return file
}

func (f *FileController) ReturnValues() ([]chartutil.Values, error) {
	valuesFiles := make([]chartutil.Values, 0, f.Queue.Len())
	for f.Queue.Len() > 0 {
		top := map[string]interface{}{
			"Chart": f.Module.Chart.Metadata,
			"Release": map[string]interface{}{
				"Name":      f.Module.Name,
				"Namespace": f.Module.Namespace,
				"IsUpgrade": true,
				"IsInstall": true,
				"Revision":  0,
				"Service":   "Helm",
			},
			"Values": f.Queue.PopFront().(map[string]interface{}),
		}

		valuesFiles = append(valuesFiles, top)
	}
	return valuesFiles, nil
}

func (f *FileController) Close() {
	_ = os.RemoveAll(f.TmpDir)
}
