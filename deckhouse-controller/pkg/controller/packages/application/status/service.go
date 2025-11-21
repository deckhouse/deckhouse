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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/operator"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
)

type Service struct {
	client client.Client
	getter ConditionsGetter

	logger *log.Logger
}

type ConditionsGetter func(namespace, name string) ([]operator.Condition, error)

func NewService(client client.Client, getter ConditionsGetter, logger *log.Logger) *Service {
	return &Service{
		client: client,
		getter: getter,
		logger: logger.Named("status-service"),
	}
}

func (s *Service) Start(ctx context.Context, ch <-chan operator.Event) {
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

func (s *Service) handleEvent(ctx context.Context, ev operator.Event) {
	logger := s.logger.With(slog.String("namespace", ev.Namespace), slog.String("name", ev.Name))

	app := new(v1alpha1.Application)
	if err := s.client.Get(ctx, client.ObjectKey{Namespace: ev.Namespace, Name: ev.Name}, app); err != nil {
		logger.Warn("failed to get application", log.Err(err))
		return
	}

	conditions, err := s.getter(ev.Namespace, ev.Name)
	if err != nil {
		logger.Warn("failed to get package conditions", log.Err(err))
		return
	}

	original := app.DeepCopy()
	s.applyConditions(app, conditions)
	if err = s.client.Status().Patch(ctx, app, client.MergeFrom(original)); err != nil {
		logger.Warn("failed to patch application status", log.Err(err))
	}
}

func (s *Service) applyConditions(app *v1alpha1.Application, conds []operator.Condition) {
	prev := make(map[string]v1alpha1.ApplicationStatusCondition)
	for _, cond := range app.Status.Conditions {
		prev[cond.Type] = cond
	}

	now := metav1.Now()
	applied := make([]v1alpha1.ApplicationStatusCondition, 0, len(conds))
	for _, c := range conds {
		cond := v1alpha1.ApplicationStatusCondition{
			Type:               c.Type,
			Status:             corev1.ConditionStatus(c.Status),
			LastTransitionTime: now,
			LastProbeTime:      now,
		}

		if p, ok := prev[cond.Type]; ok && p.Status == cond.Status {
			cond.LastTransitionTime = p.LastTransitionTime
		}

		applied = append(applied, cond)
	}

	app.Status.Conditions = applied
}
