/*
Copyright 2021 Flant CJSC

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

package git

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type TreeObject struct {
	Mode   string
	Type   string
	Object string
	File   string
}

func ListTreeObjects(path string) ([]TreeObject, error) {
	cmd := exec.Command("git", "ls-tree", "@", "--", ".")
	cmd.Dir = path

	output, err := cmd.Output()

	switch e := err.(type) {
	case *exec.ExitError:
		return nil, fmt.Errorf("failed to run \"git\" command: %s\n\n%s", err, e.Stderr)
	case *os.PathError:
		// images directory does not exist in module folder, return an empty images array
		return []TreeObject{}, nil
	case nil:
		parsedObjects := parseLsTreeOutput(bytes.NewReader(output))
		return parsedObjects, nil
	default:
		return nil, fmt.Errorf("unknown error occurred while reading images: %v", err)
	}
}

func parseLsTreeOutput(reader io.Reader) (objects []TreeObject) {
	scanner := bufio.NewScanner(reader)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		line := strings.Fields(scanner.Text())
		object := TreeObject{Mode: line[0], Type: line[1], Object: line[2], File: line[3]}
		objects = append(objects, object)
	}

	return
}
