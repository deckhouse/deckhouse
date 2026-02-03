// Copyright 2026 Flant JSC
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

package operator

import (
	"context"
	"errors"
	"testing"

	"github.com/Masterminds/semver/v3"
	operatormock "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/operator/mock"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/checker/dependency"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/flant/addon-operator/pkg/module_manager/models/modules"
	"github.com/gojuno/minimock/v3"
	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_buildScheduler(t *testing.T) {
	c := minimock.NewController(t)

	t.Run("scheduler.Check", func(t *testing.T) {
		cluster := func(ctx context.Context, moduleName string) (string, error) {
			return "test", nil
		}

		o := new(Operator)
		o.logger = log.NewLogger()

		t.Run("kubernetes check", func(t *testing.T) {
			mm := operatormock.NewModuleManagerMock(c)

			o.buildScheduler(cluster, mm)
			t.Run("not found in global values", func(t *testing.T) {
				gm, _ := modules.NewGlobalModule("", map[string]interface{}{}, nil, nil, nil, false)

				o.buildScheduler(cluster, mm)
				mm.GetGlobalMock.Return(gm)

				constraint, _ := semver.NewConstraint(">=1.21")
				err := o.scheduler.Check(schedule.Checks{
					Kubernetes: constraint,
				})

				var statusErr *status.Error
				if assert.True(t, errors.As(err, &statusErr)) && assert.Len(t, statusErr.Conditions, 1) {
					assert.Equal(t, string(statusErr.Conditions[0].Reason), "get version: discovery section not found in global values")
				}
			})
		})
		t.Run("module check", func(t *testing.T) {
			t.Run("error receiving module information from the cluster", func(t *testing.T) {
				mm := operatormock.NewModuleManagerMock(c)

				cluster := func(ctx context.Context, moduleName string) (string, error) {
					return "test", &apierrors.StatusError{ErrStatus: metav1.Status{
						Reason:  metav1.StatusReasonInternalError,
						Message: "test error",
					}}
				}

				o.buildScheduler(cluster, mm)

				constraint, _ := semver.NewConstraint(">=1.21")
				err := o.scheduler.Check(schedule.Checks{
					Modules: map[string]dependency.Dependency{
						"test": {
							Constraint: constraint,
							Optional:   true,
						},
					},
				})

				var statusErr *status.Error
				if assert.True(t, errors.As(err, &statusErr)) && assert.Len(t, statusErr.Conditions, 1) {
					assert.Equal(t, string(statusErr.Conditions[0].Reason), "error receiving module information from the cluster: test error")
				}
			})
			t.Run("StatusReasonNotFound optional", func(t *testing.T) {
				mm := operatormock.NewModuleManagerMock(c)

				cluster := func(ctx context.Context, moduleName string) (string, error) {
					return "test", &apierrors.StatusError{ErrStatus: metav1.Status{
						Reason: metav1.StatusReasonNotFound,
					}}
				}
				o.buildScheduler(cluster, mm)
				schedule.WithBootstrapCondition(func() bool { return true })(o.scheduler)

				constraint, _ := semver.NewConstraint(">=1.21")
				err := o.scheduler.Check(schedule.Checks{
					Modules: map[string]dependency.Dependency{
						"test": {
							Constraint: constraint,
							Optional:   true,
						},
					},
				})

				assert.NoError(t, err)
			})
			t.Run("StatusReasonNotFound not optional", func(t *testing.T) {
				mm := operatormock.NewModuleManagerMock(c)

				cluster := func(ctx context.Context, moduleName string) (string, error) {
					return "test", &apierrors.StatusError{ErrStatus: metav1.Status{
						Reason: metav1.StatusReasonNotFound,
					}}
				}

				o.buildScheduler(cluster, mm)
				schedule.WithBootstrapCondition(func() bool { return true })(o.scheduler)

				constraint, _ := semver.NewConstraint(">=1.21")
				err := o.scheduler.Check(schedule.Checks{
					Modules: map[string]dependency.Dependency{
						"test": {
							Constraint: constraint,
							Optional:   false,
						},
					},
				})

				var statusErr *status.Error
				if assert.True(t, errors.As(err, &statusErr)) && assert.Len(t, statusErr.Conditions, 1) {
					assert.Equal(t, string(statusErr.Conditions[0].Reason), "dependency 'test' not found")
				}
			})
			t.Run("1.20.0 is less than 1.21", func(t *testing.T) {
				mm := operatormock.NewModuleManagerMock(c)

				cluster := func(ctx context.Context, moduleName string) (string, error) {
					return "1.20", nil
				}

				o.buildScheduler(cluster, mm)
				schedule.WithBootstrapCondition(func() bool { return true })(o.scheduler)

				mm.IsModuleEnabledMock.When("test").Then(true)

				constraint, _ := semver.NewConstraint(">=1.21")
				err := o.scheduler.Check(schedule.Checks{
					Modules: map[string]dependency.Dependency{
						"test": {
							Constraint: constraint,
							Optional:   false,
						},
					},
				})

				var statusErr *status.Error
				if assert.True(t, errors.As(err, &statusErr)) && assert.Len(t, statusErr.Conditions, 1) {
					assert.Equal(t, string(statusErr.Conditions[0].Reason), "dependency test error: 1.20.0 is less than 1.21")
				}
			})
			t.Run("StatusSuccess", func(t *testing.T) {
				mm := operatormock.NewModuleManagerMock(c)
				cluster := func(ctx context.Context, moduleName string) (string, error) {
					return "1.22", nil
				}

				o.buildScheduler(cluster, mm)
				schedule.WithBootstrapCondition(func() bool { return true })(o.scheduler)

				mm.IsModuleEnabledMock.When("test").Then(true)

				constraint, _ := semver.NewConstraint(">=1.21")
				err := o.scheduler.Check(schedule.Checks{
					Modules: map[string]dependency.Dependency{
						"test": {
							Constraint: constraint,
							Optional:   false,
						},
					},
				})

				assert.NoError(t, err)
			})
		})
	})
}
