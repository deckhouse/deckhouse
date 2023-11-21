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

		if !isDocsPath(filepath.Clean(hdr.Name)) {
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

func isDocsPath(path string) bool {
	return strings.HasPrefix(path, "docs") ||
		strings.HasPrefix(path, "crds") ||
		strings.HasPrefix(path, "openapi")
}
