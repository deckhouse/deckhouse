// Copyright 2025 Flant JSC
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

package status

import (
	"context"
	"log/slog"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/operator/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
)

type Service struct {
	client client.Client
	getter getter

	logger *log.Logger
}

type getter func(name string) *status.Status

func NewService(client client.Client, getter getter, logger *log.Logger) *Service {
	return &Service{
		client: client,
		getter: getter,
		logger: logger.Named("status-service"),
	}
}

func (s *Service) Start(ctx context.Context, ch <-chan string) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-ch:
				s.handleEvent(ctx, event)
			}
		}
	}()
}

func (s *Service) handleEvent(ctx context.Context, ev string) {
	logger := s.logger.With(slog.String("name", ev))

	splits := strings.Split(ev, ".")

	app := new(v1alpha1.Application)
	if err := s.client.Get(ctx, client.ObjectKey{Namespace: splits[0], Name: splits[1]}, app); err != nil {
		logger.Warn("failed to get application", log.Err(err))
		return
	}

	packageStatus := s.getter(ev)
	if packageStatus == nil {
		logger.Warn("package status not found")
		return
	}

	original := app.DeepCopy()
	s.applyConditions(app, packageStatus.Conditions)
	if err := s.client.Status().Patch(ctx, app, client.MergeFrom(original)); err != nil {
		logger.Warn("failed to patch application status", log.Err(err))
	}
}

func (s *Service) applyConditions(app *v1alpha1.Application, conds []status.Condition) {
	prev := make(map[string]v1alpha1.ApplicationInternalStatusCondition)
	for _, cond := range app.Status.InternalConditions {
		prev[cond.Type] = cond
	}

	now := metav1.Now()
	applied := make([]v1alpha1.ApplicationInternalStatusCondition, 0, len(conds))
	for _, c := range conds {
		statusString := corev1.ConditionFalse
		if c.Status {
			statusString = corev1.ConditionTrue
		}

		cond := v1alpha1.ApplicationInternalStatusCondition{
			Type:               string(c.Name),
			Status:             statusString,
			Reason:             string(c.Reason),
			Message:            c.Message,
			LastTransitionTime: now,
			LastProbeTime:      now,
		}

		if p, ok := prev[cond.Type]; ok && p.Status == cond.Status {
			cond.LastTransitionTime = p.LastTransitionTime
		}

		applied = append(applied, cond)
	}

	app.Status.InternalConditions = applied
}
