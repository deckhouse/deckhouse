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

package module

import (
	"archive/tar"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
)

func ExtractDocs(img v1.Image) io.ReadCloser {
	pr, pw := io.Pipe()

	go func() {
		pw.CloseWithError(extractDocumentation(mutate.Extract(img), pw))
	}()

	return pr
}

func extractDocumentation(rc io.ReadCloser, output io.Writer) error {
	defer rc.Close()

	r := tar.NewReader(rc)
	w := tar.NewWriter(output)

	for {
		hdr, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar reader next: %w", err)
		}

		if !IsDocsPath(filepath.Clean(hdr.Name)) {
			continue
		}

		err = w.WriteHeader(hdr)
		if err != nil {
			return fmt.Errorf("write tar header: %w", err)
		}

		if hdr.Typeflag != tar.TypeReg {
			continue
		}

		_, err = io.Copy(w, r)
		if err != nil {
			return fmt.Errorf("copy tar file: %w", err)
		}
	}

	return nil
}

func IsDocsPath(path string) bool {
	return strings.HasPrefix(path, "docs") ||
		strings.HasPrefix(path, "crds") ||
		strings.HasPrefix(path, "openapi")
}
