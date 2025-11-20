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
	"github.com/google/uuid"
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

func NewService(dc dependency.Container, logger *log.Logger) *Service {
	return &Service{
		clusterUUID: uuid.New().String(),
		dc:          dc,
		logger:      logger.Named("registry-service"),
	}
}

func (s *Service) SetClusterUUID(id string) {
	s.clusterUUID = id
}

// GetImageReader downloads the package image and extracts it.
// IMPORTANT do not forget to close reader
// <registry>/<packageName>:<tag>
func (s *Service) GetImageReader(ctx context.Context, cred Registry, packageName, tag string) (io.ReadCloser, error) {
	_, span := otel.Tracer(tracerName).Start(ctx, "GetImageReader")
	defer span.End()

	span.SetAttributes(attribute.String("package", packageName))
	span.SetAttributes(attribute.String("tag", tag))

	logger := s.logger.With("package", packageName, "tag", tag)

	logger.Debug("download package image")

	// <registry>/<packageName>
	cli, err := s.buildRegistryClient(cred, packageName)
	if err != nil {
		return nil, fmt.Errorf("build registry client: %w", err)
	}

	// get <registry>/<packageName>:<tag>
	img, err := cli.Image(ctx, tag)
	if err != nil {
		return nil, fmt.Errorf("get package image: %w", err)
	}

	size, err := img.Size()
	if err != nil {
		return nil, fmt.Errorf("get package image size: %w", err)
	}

	span.SetAttributes(attribute.Int64("size", size))

	digest, err := img.Digest()
	if err != nil {
		return nil, fmt.Errorf("get package image digest: %w", err)
	}

	span.SetAttributes(attribute.String("digest", digest.String()))

	logger.Debug("extract package image",
		slog.String("digest", digest.String()),
		slog.Int64("size", size))

	return mutate.Extract(img), nil
}

// GetImageDigest downloads package image and returns its digest
// <registry>/<package>:<tag>
func (s *Service) GetImageDigest(ctx context.Context, cred Registry, packageName, tag string) (string, error) {
	_, span := otel.Tracer(tracerName).Start(ctx, "GetImageDigest")
	defer span.End()

	span.SetAttributes(attribute.String("package", packageName))
	span.SetAttributes(attribute.String("tag", tag))

	logger := s.logger.With("package", packageName, "tag", tag)

	logger.Debug("download package image")

	// <registry>/<packageName>
	cli, err := s.buildRegistryClient(cred, packageName)
	if err != nil {
		return "", fmt.Errorf("build registry client: %w", err)
	}

	// get <registry>/<packageName>:<tag>
	img, err := cli.Image(ctx, tag)
	if err != nil {
		return "", fmt.Errorf("get image: %w", err)
	}

	digest, err := img.Digest()
	if err != nil {
		return "", fmt.Errorf("get package image digest: %w", err)
	}

	return digest.String(), nil
}

// GetImageRootHash downloads package manifest to parse rootHash from manifest annotations
// <registry>/<package>:<tag>
func (s *Service) GetImageRootHash(ctx context.Context, cred Registry, packageName, tag string) (string, error) {
	_, span := otel.Tracer(tracerName).Start(ctx, "GetImageRootHash")
	defer span.End()

	span.SetAttributes(attribute.String("package", packageName))
	span.SetAttributes(attribute.String("tag", tag))

	logger := s.logger.With("package", packageName, "tag", tag)

	logger.Debug("download package image")

	// <registry>/<packageName>
	cli, err := s.buildRegistryClient(cred, packageName)
	if err != nil {
		return "", fmt.Errorf("build registry client: %w", err)
	}

	// get <registry>/<packageName>:<tag>
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

// Download downloads package on temp fs and returns path to it
// <registry>/<package>:<tag>
func (s *Service) Download(ctx context.Context, cred Registry, packageName, tag string) (string, error) {
	_, span := otel.Tracer(tracerName).Start(ctx, "Download")
	defer span.End()

	span.SetAttributes(attribute.String("package", packageName))
	span.SetAttributes(attribute.String("tag", tag))

	logger := s.logger.With("package", packageName, "tag", tag)

	logger.Debug("download package image")

	// <registry>/<packageName>
	cli, err := s.buildRegistryClient(cred, packageName)
	if err != nil {
		return "", fmt.Errorf("build registry client: %w", err)
	}

	// get <registry>/<packageName>:<tag>
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

	tmp, err := os.MkdirTemp("", "package*")
	if err != nil {
		return "", fmt.Errorf("create tmp directory: %w", err)
	}

	span.SetAttributes(attribute.String("path", tmp))

	logger.Debug("copy package to temp",
		slog.String("digest", digest.String()),
		slog.Int64("size", size),
		slog.String("path", tmp))

	return s.download(ctx, img, tmp)
}

// download copies tar to path
func (s *Service) download(_ context.Context, img crv1.Image, output string) (string, error) {
	rc := mutate.Extract(img)
	defer rc.Close()

	if err := os.MkdirAll(output, 0o700); err != nil {
		return "", fmt.Errorf("create output path: %w", err)
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
			return "", fmt.Errorf("path traversal detected in the package archive: malicious path %v", hdr.Name)
		}

		target := filepath.Join(output, hdr.Name)
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
			if err = os.Link(path.Join(output, hdr.Linkname), target); err != nil {
				return "", fmt.Errorf("create hardlink: %w", err)
			}
		}
	}

	return output, nil
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

type Registry struct {
	Name         string `json:"name" yaml:"name"`
	Repository   string `json:"repository" yaml:"repository"`
	DockerConfig string `json:"dockercfg" yaml:"dockercfg"`
	Scheme       string `json:"scheme" yaml:"scheme"`
	CA           string `json:"ca" yaml:"ca"`
}

func BuildRegistryBySource(source *v1alpha1.ModuleSource) Registry {
	return Registry{
		Name:         source.Name,
		Repository:   source.Spec.Registry.Repo,
		DockerConfig: source.Spec.Registry.DockerCFG,
		CA:           source.Spec.Registry.CA,
		Scheme:       source.Spec.Registry.Scheme,
	}
}

func BuildRegistryByRepository(repo *v1alpha1.PackageRepository) Registry {
	return Registry{
		Name:         repo.Name,
		Repository:   repo.Spec.Registry.Repo,
		DockerConfig: repo.Spec.Registry.DockerCFG,
		CA:           repo.Spec.Registry.CA,
		Scheme:       repo.Spec.Registry.Scheme,
	}
}

func (s *Service) buildRegistryClient(cred Registry, segment string) (cr.Client, error) {
	opts := []cr.Option{
		cr.WithAuth(cred.DockerConfig),
		cr.WithUserAgent(s.clusterUUID),
		cr.WithCA(cred.CA),
		cr.WithInsecureSchema(strings.ToLower(cred.Scheme) == "http"),
	}

	cli, err := s.dc.GetRegistryClient(filepath.Join(cred.Repository, segment), opts...)
	if err != nil {
		return nil, fmt.Errorf("get registry client: %w", err)
	}

	return cli, nil
}
