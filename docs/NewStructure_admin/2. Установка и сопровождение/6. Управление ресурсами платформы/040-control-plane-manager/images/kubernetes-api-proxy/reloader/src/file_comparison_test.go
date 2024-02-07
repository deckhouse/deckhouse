/*
Copyright 2023 Flant JSC

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

package src

import (
	"log"
	"os"
	"testing"
)

func TestCheckHashEqualityAndInequality(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test")
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		// Clean up the temporary test files
		err := os.RemoveAll(tempDir)
		if err != nil {
			t.Errorf("failed to remove test file: %s", err)
		}
	}()

	file1EqualPath := createTestFile(tempDir, "Hello, World!")
	if err != nil {
		t.Fatal(err)
	}
	file2EqualPath := createTestFile(tempDir, "Hello, World!")
	if err != nil {
		t.Fatal(err)
	}
	file1UnequalPath := createTestFile(tempDir, "Hello, World!")
	if err != nil {
		t.Fatal(err)
	}
	file2UnequalPath := createTestFile(tempDir, "Hello, Golang!")
	if err != nil {
		t.Fatal(err)
	}

	areHashesEqual, err := fileContentsEqual(file1EqualPath, file2EqualPath)
	if err != nil {
		t.Fatal(err)
	}
	if !areHashesEqual {
		t.Errorf("expected hash equality, got false")
	}

	areHashesUnequal, err := fileContentsEqual(file1UnequalPath, file2UnequalPath)
	if err != nil {
		t.Fatal(err)
	}
	if areHashesUnequal {
		t.Errorf("expected hash inequality, got false")
	}
}

func createTestFile(tempDir string, content string) string {
	file, err := os.CreateTemp(tempDir, "testfile")
	if err != nil {
		log.Fatal(err)
	}

	_, err = file.Write([]byte(content))
	if err != nil {
		log.Fatal(err)
	}

	err = file.Close()
	if err != nil {
		log.Fatal(err)
	}

	return file.Name()
}
