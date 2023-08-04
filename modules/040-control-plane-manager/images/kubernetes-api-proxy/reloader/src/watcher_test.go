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
	"bytes"
	"os"
	"testing"
)

func TestCopyFile(t *testing.T) {
	const src = "testdata/nginx_new.conf"
	dstFile, err := os.CreateTemp("", "nginx.conf")
	if err != nil {
		t.Fatal(err)
	}
	err = dstFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(dstFile.Name())

	err = copyFile(src, dstFile.Name())
	if err != nil {
		t.Errorf("expected nil, got: %s", err)
	}

	// Check if the contents of the copied file are the same
	srcData, err := os.ReadFile(src)
	if err != nil {
		t.Errorf("failed to read %s: %s", src, err)
	}

	dstData, err := os.ReadFile(dstFile.Name())
	if err != nil {
		t.Errorf("failed to read %s: %s", dstFile.Name(), err)
	}

	if !bytes.Equal(srcData, dstData) {
		t.Error("content of the copied file does not match original")
	}
}
