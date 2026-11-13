/*
Copyright 2026 Flant JSC

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

// Package localapi serves the FencingAgentNodeView of the own Node over a unix
// socket. Only the processes of the node that can open the socket can read it.
package localapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/pkg/log"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-agent/internal/domain"
)

const (
	shutdownTimeout = 5 * time.Second
	// listenRetry is the interval of the attempts to make the socket.
	listenRetry = 10 * time.Second
)

type Server struct {
	path   string
	node   string
	retry  time.Duration
	logger *log.Logger

	mu  sync.Mutex
	api v1alpha1.FencingAgentKubernetesAPIView
}

func New(socketPath, nodeName string, logger *log.Logger) *Server {
	return &Server{path: socketPath, node: nodeName, retry: listenRetry, logger: logger}
}

// Observe records one read of the control plane state.
func (s *Server) Observe(state domain.CPState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.api.CPSource = string(state.Source)
	s.api.LastReadOK = state.ProbeOK

	if state.ReadAt.IsZero() {
		return
	}

	readAt := metav1.NewMicroTime(state.ReadAt)
	s.api.LastReadAt = &readAt

	if state.Visible {
		s.api.LastVisibleAt = &readAt
	}
}

// Run serves the socket until ctx ends. Fencing does not need the socket:
// if the socket cannot be made, Run logs the error and tries again each retry
// interval. The directory of the socket can get its owner after the start.
func (s *Server) Run(ctx context.Context) error {
	listener, err := s.listen()
	if err != nil {
		s.logger.Error("local api is not available, the agent works without it and tries again",
			"socket", s.path, "retry", s.retry.String(), "error", err)

		if listener = s.waitListen(ctx); listener == nil {
			return nil
		}
	}

	srv := &http.Server{Handler: http.HandlerFunc(s.serve), ReadHeaderTimeout: 5 * time.Second}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(listener) }()

	s.logger.Info("local api started", "socket", s.path)

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}

		if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
			return err
		}

		return nil
	}
}

// waitListen makes the socket each retry interval until it succeeds. It
// returns nil when ctx ends.
func (s *Server) waitListen(ctx context.Context) net.Listener {
	ticker := time.NewTicker(s.retry)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}

		if listener, err := s.listen(); err == nil {
			return listener
		}
	}
}

func (s *Server) listen() (net.Listener, error) {
	// A socket file of the previous process blocks the listen.
	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("remove old socket: %w", err)
	}

	listener, err := net.Listen("unix", s.path)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", s.path, err)
	}

	if err := os.Chmod(s.path, 0o600); err != nil {
		_ = listener.Close()

		return nil, fmt.Errorf("restrict socket %s: %w", s.path, err)
	}

	return listener, nil
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "only GET is allowed", http.StatusMethodNotAllowed)

		return
	}

	s.mu.Lock()
	view := v1alpha1.FencingAgentNodeView{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.GroupVersion.String(),
			Kind:       "FencingAgentNodeView",
		},
		ObjectMeta: metav1.ObjectMeta{Name: s.node},
		Spec:       v1alpha1.FencingAgentNodeViewSpec{NodeName: s.node},
		Status:     v1alpha1.FencingAgentNodeViewStatus{KubernetesAPI: *s.api.DeepCopy()},
	}
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(view); err != nil {
		s.logger.Warn("write local api response", "error", err)
	}
}
