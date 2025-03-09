// Copyright 2023 Flant JSC
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

package sender

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	neturl "net/url"
	"registry-modules-watcher/internal/backends"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/deckhouse/deckhouse/pkg/log"
)

var (
	MaxElapsedTime = backoff.DefaultMaxElapsedTime
	MaxInterval    = backoff.DefaultMaxInterval
)

const maxRetries = 10

type Sender struct {
	client *http.Client

	logger *log.Logger
}

// New creates a new sender instance with the provided logger.
// It initializes an HTTP client with a custom transport.
// Default values: MaxIdleConns = 10, IdleConnTimeout = 30s.
func (s *Sender) newBackOff() *backoff.ExponentialBackOff {
	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = MaxElapsedTime
	b.MaxInterval = MaxInterval
	return b
}

// New creates a new sender instance with the provided logger.
// It initializes an HTTP client with a custom transport.
func New(logger *log.Logger) *Sender {
	tr := &http.Transport{
		MaxIdleConns:    10,
		IdleConnTimeout: 30 * time.Second,
	}
	client := &http.Client{Transport: tr}

	return &Sender{
		client: client,
		logger: logger,
	}
}

func (s *Sender) Send(ctx context.Context, listBackends map[string]struct{}, versions []backends.DocumentationTask) {
	syncChan := make(chan struct{}, 10)
	wg := new(sync.WaitGroup)

	for backend := range listBackends {
		syncChan <- struct{}{}
		wg.Add(1)

		go func(backend string) {
			defer func() {
				wg.Done()
				<-syncChan
			}()

			s.processBackend(ctx, backend, versions)
		}(backend)
	}

	wg.Wait()
}

func (s *Sender) processBackend(ctx context.Context, backend string, versions []backends.DocumentationTask) {
	for _, version := range versions {
		if version.Task == backends.TaskDelete {
			err := s.delete(ctx, backend, version)
			if err != nil {
				s.logger.Error("send delete docs", log.Err(err))
			}

			continue
		}

		err := s.upload(ctx, backend, version)
		if err != nil {
			s.logger.Error("send upload docs", log.Err(err))
		}
	}

	err := s.build(ctx, backend)
	if err != nil {
		s.logger.Error("send build docs", log.Err(err))
	}
}

var ErrBadStatusCode = errors.New("bad status code")

func (s *Sender) delete(ctx context.Context, backend string, version backends.DocumentationTask) error {
	url := fmt.Sprintf("http://%s/api/v1/doc/%s", backend, version.Module)

	s.logger.Info("delete documentation", slog.String("url", url))

	if len(version.ReleaseChannels) > 0 {
		params := neturl.Values{}
		params.Add("channels", strings.Join(version.ReleaseChannels, ","))
		url += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("client: could not create request: %s", err)
	}

	operation := func() error {
		resp, err := s.client.Do(req)
		if err != nil {
			return fmt.Errorf("client: error making http request: %s", err)
		}

		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNoContent {
			s.logger.Warn("delete response", slog.Int("status_code", resp.StatusCode))

			return fmt.Errorf("%w: %s", ErrBadStatusCode, resp.Status)
		}

		s.logger.Info("delete response", slog.Int("status_code", resp.StatusCode))

		return nil
	}

	b := s.newBackOff()

	err = backoff.Retry(operation, backoff.WithMaxRetries(b, maxRetries))
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}

	return nil
}

func (s *Sender) upload(ctx context.Context, backend string, version backends.DocumentationTask) error {
	url := "http://" + backend + "/api/v1/doc/" + version.Module + "/" + version.Version + "?channels=" + strings.Join(version.ReleaseChannels, ",")

	s.logger.Info("upload archive", slog.String("url", url))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(version.TarFile))
	if err != nil {
		return fmt.Errorf("client: could not create request: %s", err)
	}

	req.Header.Set("Content-Type", "application/tar")

	operation := func() error {
		resp, err := s.client.Do(req)
		if err != nil {
			return fmt.Errorf("client: error making http request: %s", err)
		}

		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			s.logger.Warn("upload archive response", slog.Int("status_code", resp.StatusCode))

			return fmt.Errorf("%w: %s", ErrBadStatusCode, resp.Status)
		}

		s.logger.Info("upload archive response", slog.Int("status_code", resp.StatusCode))

		return nil
	}

	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = MaxElapsedTime
	b.MaxInterval = MaxInterval

	err = backoff.Retry(operation, backoff.WithMaxRetries(b, maxRetries))
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}

	return nil
}

func (s *Sender) build(ctx context.Context, backend string) error {
	url := "http://" + backend + "/api/v1/build"

	s.logger.Info("send build", slog.String("url", url))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("client: could not create request: %s", err)
	}

	req.Header.Set("Content-Type", "application/json")

	operation := func() error {
		resp, err := s.client.Do(req)
		if err != nil {
			return fmt.Errorf("client: error making http request: %s", err)
		}

		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			s.logger.Warn("build response", slog.Int("status_code", resp.StatusCode))

			return fmt.Errorf("%w: %s", ErrBadStatusCode, resp.Status)
		}

		s.logger.Info("build response", slog.Int("status_code", resp.StatusCode))

		return nil
	}

	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = MaxElapsedTime
	b.MaxInterval = MaxInterval

	err = backoff.Retry(operation, backoff.WithMaxRetries(b, maxRetries))
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}

	return nil
}
