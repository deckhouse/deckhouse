// Copyright 2025 Flant JSC
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

package tar

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func createTestDir(baseDir string) error {
	testDir := filepath.Join(baseDir, "testdir")
	err := os.MkdirAll(testDir, 0755)
	if err != nil {
		return err
	}

	files := []string{"file1.txt", "file2.txt"}
	for _, file := range files {
		err := os.WriteFile(filepath.Join(testDir, file), []byte("This is a test file."), 0644)
		if err != nil {
			return err
		}
	}

	return nil
}

func TestCreateTar(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tar_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	err = createTestDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	tarFilePath := filepath.Join(tmpDir, "output.tar")
	err = CreateTar(tarFilePath, tmpDir, "testdir")
	if err != nil {
		t.Fatalf("Failed to create tar file: %v", err)
	}

	tarFile, err := os.Open(tarFilePath)
	if err != nil {
		t.Fatalf("Failed to open tar file: %v", err)
	}
	defer tarFile.Close()

	tarReader := tar.NewReader(tarFile)

	_, err = tarReader.Next()
	if err != nil {
		t.Fatalf("Failed to read from tar file: %v", err)
	}

	expectedFiles := []string{"testdir/file1.txt", "testdir/file2.txt"}
	for _, expectedFile := range expectedFiles {
		header, err := tarReader.Next()
		if err != nil {
			t.Fatalf("Failed to read from tar file: %v", err)
		}

		if header.Name != expectedFile {
			t.Errorf("Expected %s, got %s", expectedFile, header.Name)
		}

		content, err := io.ReadAll(tarReader)
		if err != nil {
			t.Fatalf("Failed to read file content from tar: %v", err)
		}

		if string(content) != "This is a test file." {
			t.Errorf("Content mismatch for %s", expectedFile)
		}
	}
}
