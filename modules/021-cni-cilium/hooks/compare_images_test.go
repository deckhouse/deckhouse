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

package hooks

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	imagesDir     = "../images"
	ciliumDir     = "cilium"
	virtCiliumDir = "virt-cilium"
)

var (
	ciliumImage     = filepath.Join(imagesDir, ciliumDir)
	virtCiliumImage = filepath.Join(imagesDir, virtCiliumDir)
	missingFiles    []string
	differentFiles  []string
)

func visit(path string, di fs.DirEntry, err error) error {
	// Skip directories
	if di.IsDir() {
		return nil
	}

	relPath, err := filepath.Rel(ciliumImage, path)
	if err != nil {
		return err
	}
	virtCiliumPath := filepath.Join(virtCiliumImage, relPath)

	if _, err := os.Stat(virtCiliumPath); err != nil {
		missingFiles = append(missingFiles, relPath)
		return nil
	}

	same, err := FileCmp(path, virtCiliumPath, 0)
	if err != nil {
		return err
	}
	if !same {
		differentFiles = append(differentFiles, relPath)
		return nil
	}

	return nil
}

var _ = Describe("Modules :: cni-cilium :: images :: compare_images ::", func() {

	Context("Compare cilium and virt-cilium images", func() {
		var foundProblems error
		err := filepath.WalkDir(ciliumImage, visit)
		if len(missingFiles) != 0 || len(differentFiles) != 0 {
			foundProblems = fmt.Errorf("missing files:\n- %s\ndifferent files:\n- %s\n",
				strings.Join(missingFiles, "\n- "),
				strings.Join(differentFiles, "\n- "),
			)
		}
		It("Image virt-cilium should include all files from cilium", func() {
			Expect(err).ShouldNot(HaveOccurred())
			Expect(foundProblems).ShouldNot(HaveOccurred())
		})
	})

})

// Decide if two files have the same contents or not.
// chunkSize is the size of the blocks to scan by; pass 0 to get a sensible default.
// *Follows* symlinks.
//
// May return an error if something else goes wrong; in this case, you should ignore the value of 'same'.
//
// derived from https://stackoverflow.com/a/30038571
// under CC-BY-SA-4.0 by several contributors
func FileCmp(file1, file2 string, chunkSize int) (same bool, err error) {

	if chunkSize == 0 {
		chunkSize = 4 * 1024
	}

	// shortcuts: check file metadata
	stat1, err := os.Stat(file1)
	if err != nil {
		return false, err
	}

	stat2, err := os.Stat(file2)
	if err != nil {
		return false, err
	}

	// are inputs are literally the same file?
	if os.SameFile(stat1, stat2) {
		return true, nil
	}

	// do inputs at least have the same size?
	if stat1.Size() != stat2.Size() {
		return false, nil
	}

	// long way: compare contents
	f1, err := os.Open(file1)
	if err != nil {
		return false, err
	}
	defer f1.Close()

	f2, err := os.Open(file2)
	if err != nil {
		return false, err
	}
	defer f2.Close()

	b1 := make([]byte, chunkSize)
	b2 := make([]byte, chunkSize)
	for {
		n1, err1 := io.ReadFull(f1, b1)
		n2, err2 := io.ReadFull(f2, b2)

		// https://pkg.go.dev/io#Reader
		// > Callers should always process the n > 0 bytes returned
		// > before considering the error err. Doing so correctly
		// > handles I/O errors that happen after reading some bytes
		// > and also both of the allowed EOF behaviors.

		if !bytes.Equal(b1[:n1], b2[:n2]) {
			return false, nil
		}

		if (err1 == io.EOF && err2 == io.EOF) || (err1 == io.ErrUnexpectedEOF && err2 == io.ErrUnexpectedEOF) {
			return true, nil
		}

		// some other error, like a dropped network connection or a bad transfer
		if err1 != nil {
			return false, err1
		}
		if err2 != nil {
			return false, err2
		}
	}
}
