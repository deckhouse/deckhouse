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

package hooks_test

import (
	"context"
	"testing"

	addonhooks "github.com/flant/addon-operator/pkg/module_manager/models/hooks"
	"github.com/flant/addon-operator/pkg/module_manager/models/hooks/kind"
	bctx "github.com/flant/shell-operator/pkg/hook/binding_context"
	"github.com/flant/shell-operator/pkg/hook/controller"
	shtypes "github.com/flant/shell-operator/pkg/hook/types"
	"github.com/stretchr/testify/suite"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/hooks"
	utils "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
)

// collidingName is the shape that broke a cluster: the SDK derives a Go hook's
// name from the file that registered it, so two RegisterFunc calls in one file
// produce two hooks under this one name.
const collidingName = "160-multitenancy-manager/hooks/alert_on_grant_forbidden_resource_use.go"

// fakeHook is a minimal hooks.Hook used to drive Storage; the real hooks wrap
// addon-operator types that need a live SDK registry.
type fakeHook struct {
	name   string
	config *addonhooks.ModuleHookConfig
	ctrl   *controller.HookController
}

// newKubeHook builds a hook declaring a single OnKubernetesEvent binding.
func newKubeHook(name string) *fakeHook {
	cfg := new(addonhooks.ModuleHookConfig)
	cfg.OnKubernetesEvents = []shtypes.OnKubernetesEventConfig{{
		CommonBindingConfig: shtypes.CommonBindingConfig{BindingName: "grants"},
	}}

	return &fakeHook{name: name, config: cfg}
}

// newScheduleHook builds a hook declaring a single Schedule binding.
func newScheduleHook(name string) *fakeHook {
	cfg := new(addonhooks.ModuleHookConfig)
	cfg.Schedules = []shtypes.ScheduleConfig{{
		CommonBindingConfig: shtypes.CommonBindingConfig{BindingName: "grants"},
	}}

	return &fakeHook{name: name, config: cfg}
}

func (h *fakeHook) GetName() string                                 { return h.name }
func (h *fakeHook) GetConfigVersion() string                        { return "v1" }
func (h *fakeHook) GetHookConfig() *addonhooks.ModuleHookConfig     { return h.config }
func (h *fakeHook) Order(_ shtypes.BindingType) float64             { return 0 }
func (h *fakeHook) InitializeHookConfig() error                     { return nil }
func (h *fakeHook) GetHookController() *controller.HookController   { return h.ctrl }
func (h *fakeHook) WithHookController(c *controller.HookController) { h.ctrl = c }
func (h *fakeHook) WithTmpDir(_ string)                             {}
func (h *fakeHook) SynchronizationNeeded() bool                     { return false }

func (h *fakeHook) Execute(_ context.Context, _ string, _ []bctx.BindingContext, _ string,
	_, _ utils.Values, _ map[string]string,
) (*kind.HookResult, error) {
	return nil, nil
}

// StorageSuite exercises hooks.Storage through its exported surface.
type StorageSuite struct {
	suite.Suite

	storage *hooks.Storage
}

// TestStorageSuite is the testing.T entry point that runs the suite.
func TestStorageSuite(t *testing.T) {
	suite.Run(t, new(StorageSuite))
}

// SetupTest builds a fresh storage for every case.
func (s *StorageSuite) SetupTest() {
	s.storage = hooks.NewStorage()
}

// TestGetHooksKeepsBothHooksOfOneFile is the regression: GetHooks used to be
// built from the name index, so the second hook of a file vanished from the set
// InitializeHooks walks — and the Enable task then dereferenced its nil
// controller while enabling Kubernetes bindings.
func (s *StorageSuite) TestGetHooksKeepsBothHooksOfOneFile() {
	kubeHook := newKubeHook(collidingName)
	scheduleHook := newScheduleHook(collidingName)

	s.storage.Add(kubeHook)
	s.storage.Add(scheduleHook)

	all := s.storage.GetHooks()
	s.Require().Len(all, 2, "a name is not a unique key — both hooks must survive")
	s.Contains(all, hooks.Hook(kubeHook))
	s.Contains(all, hooks.Hook(scheduleHook))
}

// TestEveryHookReachableForControllerWiring pins the property the panic was
// about: wiring controllers over GetHooks must leave no hook behind, including
// one reachable only through the binding index.
func (s *StorageSuite) TestEveryHookReachableForControllerWiring() {
	kubeHook := newKubeHook(collidingName)
	scheduleHook := newScheduleHook(collidingName)

	s.storage.Add(kubeHook)
	s.storage.Add(scheduleHook)

	for _, hook := range s.storage.GetHooks() {
		hook.WithHookController(controller.NewHookController())
	}

	for _, hook := range s.storage.GetHooksByBinding(shtypes.OnKubernetesEvent) {
		s.NotNil(hook.GetHookController(), "kube-bound hook must be wired")
	}

	for _, hook := range s.storage.GetHooksByBinding(shtypes.Schedule) {
		s.NotNil(hook.GetHookController(), "schedule-bound hook must be wired")
	}
}

// TestGetHooksReturnsInsertionOrder keeps iteration deterministic: wiring and
// startup walk this slice, and a map-backed result reordered them every run.
func (s *StorageSuite) TestGetHooksReturnsInsertionOrder() {
	first := newKubeHook("a.go")
	second := newKubeHook("b.go")
	third := newScheduleHook("c.go")

	s.storage.Add(first)
	s.storage.Add(second)
	s.storage.Add(third)

	all := s.storage.GetHooks()
	s.Require().Len(all, 3)
	s.Equal("a.go", all[0].GetName())
	s.Equal("b.go", all[1].GetName())
	s.Equal("c.go", all[2].GetName())
}

// TestGetHooksReturnsCopy confirms a caller cannot mutate the stored set.
func (s *StorageSuite) TestGetHooksReturnsCopy() {
	s.storage.Add(newKubeHook("a.go"))

	all := s.storage.GetHooks()
	all[0] = nil

	s.NotNil(s.storage.GetHooks()[0], "the returned slice must not alias storage")
}
