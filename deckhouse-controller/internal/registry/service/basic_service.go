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

package service

import (
	"context"
	"fmt"
	"log/slog"

	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/deckhouse/deckhouse/pkg/registry"
)

// BasicService provides common registry operations with standardized logging
type BasicService struct {
	name   string
	client registry.Client
	logger *log.Logger
}

// NewBasicService creates a new basic service
func NewBasicService(name string, client registry.Client, logger *log.Logger) *BasicService {
	return &BasicService{
		name:   name,
		client: client,
		logger: logger,
	}
}

// GetImage retrieves an image from the registry
func (s *BasicService) GetImage(ctx context.Context, tag string, opts ...registry.ImageGetOption) (registry.Image, error) {
	logger := s.logger.With(slog.String("service", s.name), slog.String("tag", tag))

	logger.Debug("Getting image")

	img, err := s.client.GetImage(ctx, tag, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to get image: %w", err)
	}

	logger.Debug("Image retrieved successfully")

	return img, nil
}

// GetDigest retrieves a digest from the registry
func (s *BasicService) GetDigest(ctx context.Context, tag string) (*v1.Hash, error) {
	logger := s.logger.With(slog.String("service", s.name), slog.String("tag", tag))

	logger.Debug("Getting digest")

	hash, err := s.client.GetDigest(ctx, tag)
	if err != nil {
		return nil, fmt.Errorf("failed to get digest: %w", err)
	}

	logger.Debug("Digest retrieved successfully")

	return hash, nil
}

func (s *BasicService) CheckImageExists(ctx context.Context, tag string) error {
	logger := s.logger.With(slog.String("service", s.name), slog.String("tag", tag))

	logger.Debug("Checking if image exists")

	err := s.client.CheckImageExists(ctx, tag)
	if err != nil {
		return fmt.Errorf("failed to check if image exists: %w", err)
	}

	s.logger.Debug("Image existence check completed")

	return nil
}

func (s *BasicService) ListTags(ctx context.Context) ([]string, error) {
	logger := s.logger.With(slog.String("service", s.name))

	logger.Debug("Listing tags")

	tags, err := s.client.ListTags(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}

	logger.Debug("Tags listed successfully", slog.Int("count", len(tags)))

	return tags, nil
}
