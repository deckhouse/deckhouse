/*
Copyright 2025 Flant JSC

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

package downloader

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	crv1 "github.com/google/go-containerregistry/pkg/v1"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	tracerName = "downloader"
)

type Downloader struct {
	clusterUUID string
	dc          dependency.Container
	logger      *log.Logger
}

func New(clusterUUID string, dc dependency.Container, logger *log.Logger) *Downloader {
	return &Downloader{
		clusterUUID: clusterUUID,
		dc:          dc,
		logger:      logger.Named("module-downloader"),
	}
}

type ExtractFunc func(ctx context.Context, rc io.ReadCloser) error

// Extract downloads the module image and allows us to inject into tar extraction process via ExtractFunc
func (d *Downloader) Extract(ctx context.Context, ms *v1alpha1.ModuleSource, module, tag string, f ExtractFunc) error {
	_, span := otel.Tracer(tracerName).Start(ctx, "Extract")
	defer span.End()

	span.SetAttributes(attribute.String("module", module))
	span.SetAttributes(attribute.String("tag", tag))
	span.SetAttributes(attribute.String("source", ms.GetName()))

	logger := d.logger.With("module", module, "tag", tag)

	logger.Debug("download module")

	// <registry>/modules/<module>
	cli, err := d.buildRegistryClient(ms, module)
	if err != nil {
		return fmt.Errorf("build registry client: %w", err)
	}

	// get <registry>/modules/<module>:<tag>
	img, err := cli.Image(ctx, tag)
	if err != nil {
		return fmt.Errorf("get image: %w", err)
	}

	logger.Debug("extract image")

	rc, err := cr.Extract(img)
	if err != nil {
		return fmt.Errorf("extract image: %w", err)
	}
	defer rc.Close()

	return f(ctx, rc)
}

// Download downloads module on temp fs and returns path to it
func (d *Downloader) Download(ctx context.Context, ms *v1alpha1.ModuleSource, module, tag string) (string, error) {
	_, span := otel.Tracer(tracerName).Start(ctx, "Download")
	defer span.End()

	span.SetAttributes(attribute.String("module", module))
	span.SetAttributes(attribute.String("tag", tag))
	span.SetAttributes(attribute.String("source", ms.GetName()))

	logger := d.logger.With("module", module, "tag", tag)

	logger.Debug("download module")

	// <registry>/modules/<module>
	cli, err := d.buildRegistryClient(ms, module)
	if err != nil {
		return "", fmt.Errorf("build registry client: %w", err)
	}

	// get <registry>/modules/<module>:<tag>
	img, err := cli.Image(ctx, tag)
	if err != nil {
		return "", fmt.Errorf("get image: %w", err)
	}

	tmp, err := os.MkdirTemp("", "module*")
	if err != nil {
		return "", fmt.Errorf("create tmp directory: %w", err)
	}

	logger.Debug("copy module to temp")

	return d.download(ctx, img, tmp)
}

// download copies tar to path
func (d *Downloader) download(_ context.Context, img crv1.Image, modulePath string) (string, error) {
	rc, err := cr.Extract(img)
	if err != nil {
		return "", fmt.Errorf("extract image: %w", err)
	}
	defer rc.Close()

	if err = os.MkdirAll(modulePath, 0o700); err != nil {
		return "", fmt.Errorf("create module path: %w", err)
	}

	tr := tar.NewReader(rc)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			// end of archive
			break
		}
		if err != nil {
			return "", fmt.Errorf("read tar: %w", err)
		}

		if strings.Contains(hdr.Name, "..") {
			// CWE-22 check, prevents path traversal
			return "", fmt.Errorf("path traversal detected in the module archive: malicious path %v", hdr.Name)
		}

		target := filepath.Join(modulePath, hdr.Name)

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err = os.MkdirAll(target, os.FileMode(hdr.Mode)); err != nil {
				return "", fmt.Errorf("mkdir: %w", err)
			}

		case tar.TypeReg:
			out, err := os.Create(target)
			if err != nil {
				return "", fmt.Errorf("create file: %w", err)
			}

			if _, err = io.Copy(out, tr); err != nil {
				out.Close()
				return "", fmt.Errorf("copy file: %w", err)
			}
			out.Close()

			// remove only 'user' permission bit, E.x.: 644 => 600, 755 => 700
			if err = os.Chmod(out.Name(), os.FileMode(hdr.Mode)&0o700); err != nil {
				return "", fmt.Errorf("chmod: %w", err)
			}

		case tar.TypeSymlink:
			if isRel(hdr.Linkname, target) && isRel(hdr.Name, target) {
				if err = os.Symlink(hdr.Linkname, target); err != nil {
					return "", fmt.Errorf("create symlink: %w", err)
				}
			}

		case tar.TypeLink:
			if err = os.Link(path.Join(modulePath, hdr.Linkname), target); err != nil {
				return "", fmt.Errorf("create hardlink: %w", err)
			}
		}
	}

	return modulePath, nil
}

func isRel(candidate, target string) bool {
	// GOOD: resolves all symbolic links before checking
	// that `candidate` does not escape from `target`
	if filepath.IsAbs(candidate) {
		return false
	}

	realpath, err := filepath.EvalSymlinks(filepath.Join(target, candidate))
	if err != nil {
		return false
	}

	relpath, err := filepath.Rel(target, realpath)
	return err == nil && !strings.HasPrefix(filepath.Clean(relpath), "..")
}

func (d *Downloader) buildRegistryClient(ms *v1alpha1.ModuleSource, path string) (cr.Client, error) {
	opts := []cr.Option{
		cr.WithAuth(ms.Spec.Registry.DockerCFG),
		cr.WithUserAgent(d.clusterUUID),
		cr.WithCA(ms.Spec.Registry.CA),
		cr.WithInsecureSchema(strings.ToLower(ms.Spec.Registry.Scheme) == "http"),
	}

	cli, err := d.dc.GetRegistryClient(filepath.Join(ms.Spec.Registry.Repo, path), opts...)
	if err != nil {
		return nil, fmt.Errorf("get registry client: %w", err)
	}

	return cli, nil
}
