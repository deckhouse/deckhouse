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
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/gammazero/deque"
	"github.com/mohae/deepcopy"
	"gopkg.in/yaml.v2"
	"helm.sh/helm/v3/pkg/chartutil"
)

const (
	// Variations add ability to create variants for tests matrix
	ConstantVariation string = "__ConstantChoices__"
	RangeVariation    string = "__RangeChoices__"

	// Item works like variation arguments to include special variants of values
	EmptyItem string = "__EmptyItem__"
)

type FileController struct {
	Prefix string
	TmpDir string
	Queue  *deque.Deque
}

type Node struct {
	Keys []interface{}
	Item interface{}
}

func NewNode(item interface{}) Node {
	return Node{Item: item}
}

func LoadConfiguration(path, prefix, dir string) (FileController, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return FileController{}, fmt.Errorf("formatting path failed: %v", err)
	}

	configurationFile, err := ioutil.ReadFile(absPath)
	if err != nil {
		return FileController{}, fmt.Errorf("read matrix tests configuration file failed: %v", err)
	}

	result := make(map[interface{}]interface{})

	err = yaml.Unmarshal(configurationFile, result)
	if err != nil {
		return FileController{}, fmt.Errorf("configuration unmarshalling error: %v", err)
	}

	filesQueue := deque.Deque{}
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

	tmpDir, err := ioutil.TempDir(dir, "")
	if err != nil {
		return FileController{}, fmt.Errorf("tmp directory error: %v", err)
	}
	return FileController{Queue: &filesQueue, Prefix: prefix, TmpDir: tmpDir}, nil
}

func findVariations(nodeData interface{}) ([]interface{}, []interface{}) {
	queue := deque.Deque{}
	queue.PushBack(NewNode(nodeData))

	for queue.Len() > 0 {
		tempNode := queue.PopFront().(Node)

		switch data := tempNode.Item.(type) {
		case map[interface{}]interface{}:
			for key, value := range data {
				key := key.(string)

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
	case map[interface{}]interface{}:
		if len(keys)-1 == counter {
			if resultItem == EmptyItem {
				delete(f, key)
			} else {
				f[key] = resultItem
			}
		} else {
			// Recursive call, need to be fixed
			counter++
			f[key] = formatFile(f[key], keys, resultItem, counter)
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

func (f *FileController) SaveValues() error {
	counter := 1
	for f.Queue.Len() > 0 {
		filename := fmt.Sprintf("%s%s%s%v.yaml", f.TmpDir, string(os.PathSeparator), f.Prefix, counter)
		out, err := yaml.Marshal(f.Queue.PopFront())

		if err != nil {
			return fmt.Errorf("saving values file %s failed: %v", filename, err)
		}

		err = ioutil.WriteFile(filename, out, 0755)
		if err != nil {
			return fmt.Errorf("saving values file %s failed: %v", filename, err)
		}
		counter++
	}
	return nil
}

func (f *FileController) ReturnValues() ([]chartutil.Values, error) {
	valuesFiles := make([]chartutil.Values, 0, f.Queue.Len())
	for f.Queue.Len() > 0 {
		valuesFiles = append(valuesFiles, f.Queue.PopFront().(map[string]interface{}))
	}
	return valuesFiles, nil
}

func (f *FileController) Close() {
	_ = os.RemoveAll(f.TmpDir)
}
