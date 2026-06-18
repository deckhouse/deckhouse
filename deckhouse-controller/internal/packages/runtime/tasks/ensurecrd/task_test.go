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

package ensurecrd_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	taskensurecrd "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/ensurecrd"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// fakePackage is a minimal packageI implementation for driving the task.
type fakePackage struct {
	name string
	path string
}

func (p *fakePackage) GetName() string { return p.name }
func (p *fakePackage) GetPath() string { return p.path }

// fakeInstaller records the path it was called with and returns a fixed error.
type fakeInstaller struct {
	err        error
	calledWith string
	calls      int
}

func (i *fakeInstaller) EnsureCRDs(_ context.Context, packagePath string) error {
	i.calls++
	i.calledWith = packagePath
	return i.err
}

// conditionStatus returns the status of the named condition, or empty if absent.
func conditionStatus(s status.Status, cond status.ConditionType) metav1.ConditionStatus {
	for _, c := range s.Conditions {
		if c.Type == cond {
			return c.Status
		}
	}
	return ""
}

func conditionReason(s status.Status, cond status.ConditionType) status.ConditionReason {
	for _, c := range s.Conditions {
		if c.Type == cond {
			return c.Reason
		}
	}
	return ""
}

func TestExecute_Success(t *testing.T) {
	const (
		name = "d8-system.cni-cilium"
		path = "/deckhouse/modules/cni-cilium"
	)

	statusSvc := status.NewService()
	statusSvc.NewStatus(name)

	inst := &fakeInstaller{}
	task := taskensurecrd.NewTask(&fakePackage{name: name, path: path}, inst, statusSvc, log.NewNop())

	require.NoError(t, task.Execute(context.Background()))

	require.Equal(t, 1, inst.calls)
	require.Equal(t, path, inst.calledWith, "installer must receive the package path")

	got := statusSvc.GetStatus(name)
	require.Equal(t, metav1.ConditionTrue, conditionStatus(got, status.ConditionCRDsEnsured))
}

func TestExecute_Failure(t *testing.T) {
	const name = "d8-system.cni-cilium"

	statusSvc := status.NewService()
	statusSvc.NewStatus(name)

	inst := &fakeInstaller{err: errors.New("apply boom")}
	task := taskensurecrd.NewTask(&fakePackage{name: name, path: "/deckhouse/modules/cni-cilium"}, inst, statusSvc, log.NewNop())

	err := task.Execute(context.Background())
	require.Error(t, err)
	require.ErrorContains(t, err, "apply boom")

	got := statusSvc.GetStatus(name)
	require.Equal(t, metav1.ConditionFalse, conditionStatus(got, status.ConditionCRDsEnsured))
	require.Equal(t, status.ConditionReason("EnsureCRDsFailed"), conditionReason(got, status.ConditionCRDsEnsured))
}

func TestString(t *testing.T) {
	task := taskensurecrd.NewTask(&fakePackage{name: "x", path: "/p"}, &fakeInstaller{}, status.NewService(), log.NewNop())
	require.Equal(t, "EnsureCRD", task.String())
}
