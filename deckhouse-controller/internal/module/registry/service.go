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

package registry

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"

	crv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	tracerName = "registry"

	annotationRootHash = "io.deckhouse.delivery-kit.dm-verity-root-hash"
)

type Service struct {
	clusterUUID string
	dc          dependency.Container
	logger      *log.Logger
}

func NewService(clusterUUID string, dc dependency.Container, logger *log.Logger) *Service {
	return &Service{
		clusterUUID: clusterUUID,
		dc:          dc,
		logger:      logger.Named("registry-service"),
	}
}

// GetImageReader downloads the module image and extracts it.
// IMPORTANT do not forget to close reader
// <registry>/modules/<module>:<tag>
func (s *Service) GetImageReader(ctx context.Context, ms *v1alpha1.ModuleSource, module, tag string) (io.ReadCloser, error) {
	_, span := otel.Tracer(tracerName).Start(ctx, "GetImageReader")
	defer span.End()

	span.SetAttributes(attribute.String("module", module))
	span.SetAttributes(attribute.String("tag", tag))
	span.SetAttributes(attribute.String("source", ms.GetName()))

	logger := s.logger.With("module", module, "tag", tag)

	logger.Debug("download module image")

	// <registry>/modules/<module>
	cli, err := s.buildRegistryClient(ms, module)
	if err != nil {
		return nil, fmt.Errorf("build registry client: %w", err)
	}

	// get <registry>/modules/<module>:<tag>
	img, err := cli.Image(ctx, tag)
	if err != nil {
		return nil, fmt.Errorf("get module image: %w", err)
	}

	size, err := img.Size()
	if err != nil {
		return nil, fmt.Errorf("get module image size: %w", err)
	}

	span.SetAttributes(attribute.Int64("size", size))

	digest, err := img.Digest()
	if err != nil {
		return nil, fmt.Errorf("get module image digest: %w", err)
	}

	span.SetAttributes(attribute.String("digest", digest.String()))

	logger.Debug("extract module image",
		slog.String("digest", digest.String()),
		slog.Int64("size", size))

	return mutate.Extract(img), nil
}

// GetImageDigest downloads module image and returns its digest
// <registry>/modules/<module>:<tag>
func (s *Service) GetImageDigest(ctx context.Context, ms *v1alpha1.ModuleSource, module, tag string) (string, error) {
	_, span := otel.Tracer(tracerName).Start(ctx, "GetImageDigest")
	defer span.End()

	span.SetAttributes(attribute.String("module", module))
	span.SetAttributes(attribute.String("tag", tag))
	span.SetAttributes(attribute.String("source", ms.GetName()))

	logger := s.logger.With("module", module, "tag", tag)

	logger.Debug("download module image")

	// <registry>/modules/<module>
	cli, err := s.buildRegistryClient(ms, module)
	if err != nil {
		return "", fmt.Errorf("build registry client: %w", err)
	}

	// get <registry>/modules/<module>:<tag>
	img, err := cli.Image(ctx, tag)
	if err != nil {
		return "", fmt.Errorf("get image: %w", err)
	}

	digest, err := img.Digest()
	if err != nil {
		return "", fmt.Errorf("get module image digest: %w", err)
	}

	return digest.String(), nil
}

// GetImageRootHash downloads module manifest to parse rootHash from manifest annotations
// <registry>/modules/<module>:<tag>
func (s *Service) GetImageRootHash(ctx context.Context, ms *v1alpha1.ModuleSource, module, tag string) (string, error) {
	_, span := otel.Tracer(tracerName).Start(ctx, "GetImageRootHash")
	defer span.End()

	span.SetAttributes(attribute.String("module", module))
	span.SetAttributes(attribute.String("tag", tag))
	span.SetAttributes(attribute.String("source", ms.GetName()))

	logger := s.logger.With("module", module, "tag", tag)

	logger.Debug("download module image")

	// <registry>/modules/<module>
	cli, err := s.buildRegistryClient(ms, module)
	if err != nil {
		return "", fmt.Errorf("build registry client: %w", err)
	}

	// get <registry>/modules/<module>:<tag>
	img, err := cli.Image(ctx, tag)
	if err != nil {
		return "", fmt.Errorf("get image: %w", err)
	}

	manifest, err := img.Manifest()
	if err != nil {
		return "", fmt.Errorf("get manifest: %w", err)
	}

	var rootHash string
	if len(manifest.Annotations) > 0 {
		rootHash = manifest.Annotations[annotationRootHash]
	}

	return rootHash, nil
}

// Download downloads module on temp fs and returns path to it
// <registry>/modules/<module>:<tag>
func (s *Service) Download(ctx context.Context, ms *v1alpha1.ModuleSource, module, tag string) (string, error) {
	_, span := otel.Tracer(tracerName).Start(ctx, "Download")
	defer span.End()

	span.SetAttributes(attribute.String("module", module))
	span.SetAttributes(attribute.String("tag", tag))
	span.SetAttributes(attribute.String("source", ms.GetName()))

	logger := s.logger.With("module", module, "tag", tag)

	logger.Debug("download module image")

	// <registry>/modules/<module>
	cli, err := s.buildRegistryClient(ms, module)
	if err != nil {
		return "", fmt.Errorf("build registry client: %w", err)
	}

	// get <registry>/modules/<module>:<tag>
	img, err := cli.Image(ctx, tag)
	if err != nil {
		return "", fmt.Errorf("get image: %w", err)
	}

	size, err := img.Size()
	if err != nil {
		return "", fmt.Errorf("get image size: %w", err)
	}

	span.SetAttributes(attribute.Int64("size", size))

	digest, err := img.Digest()
	if err != nil {
		return "", fmt.Errorf("get image digest: %w", err)
	}

	span.SetAttributes(attribute.String("digest", digest.String()))

	tmp, err := os.MkdirTemp("", "module*")
	if err != nil {
		return "", fmt.Errorf("create tmp directory: %w", err)
	}

	span.SetAttributes(attribute.String("path", tmp))

	logger.Debug("copy module to temp",
		slog.String("digest", digest.String()),
		slog.Int64("size", size),
		slog.String("path", tmp))

	return s.download(ctx, img, tmp)
}

// download copies tar to path
func (s *Service) download(_ context.Context, img crv1.Image, modulePath string) (string, error) {
	rc := mutate.Extract(img)
	defer rc.Close()

	if err := os.MkdirAll(modulePath, 0o700); err != nil {
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

func (s *Service) buildRegistryClient(ms *v1alpha1.ModuleSource, path string) (cr.Client, error) {
	opts := []cr.Option{
		cr.WithAuth(ms.Spec.Registry.DockerCFG),
		cr.WithUserAgent(s.clusterUUID),
		cr.WithCA(ms.Spec.Registry.CA),
		cr.WithInsecureSchema(strings.ToLower(ms.Spec.Registry.Scheme) == "http"),
	}

	cli, err := s.dc.GetRegistryClient(filepath.Join(ms.Spec.Registry.Repo, path), opts...)
	if err != nil {
		return nil, fmt.Errorf("get registry client: %w", err)
	}

	return cli, nil
}
