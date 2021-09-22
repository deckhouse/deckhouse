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

package fs

import (
	"fmt"
	"os"
)

func CreateFileWithContent(path, content string) error {
	newFile, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file %s: %v", path, err)
	}
	defer newFile.Close()

	if content != "" {
		_, err = newFile.WriteString(content)
		if err != nil {
			return fmt.Errorf("create file with content %s: %v", path, err)
		}
	}
	return nil
}

func CreateEmptyFile(path string) error {
	return CreateFileWithContent(path, "")
}

func TouchFile(path string) error {
	err := CreateEmptyFile(path)

	if err != nil && os.IsExist(err) {
		return nil
	}

	return err
}
