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

package ensurecrd

import (
	"context"
	"fmt"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
)

type packageI interface {
	GetName() string
	GetPath() string
}

type statusService interface {
	HandleError(name string, err error)
}

type crdsInstaller interface {
	EnsureCRDs(ctx context.Context, packagePath string) error
}

// task applies CRD manifests from the package's filesystem path to the cluster.
// It runs early in the install pipeline — before hooks or Helm — so that
// custom resources referenced by later stages already exist.
type task struct {
	pkg packageI

	crdsInstaller crdsInstaller
	status        statusService
}

// NewTask creates an EnsureCRD task for the given package.
// On failure the error is forwarded to the status service so that
// the corresponding condition is updated before the task retries.
func NewTask(pkg packageI, crdsInstaller crdsInstaller, status statusService) queue.Task {
	return &task{
		pkg:           pkg,
		crdsInstaller: crdsInstaller,
		status:        status,
	}
}

func (t *task) String() string {
	return "EnsureCRD"
}

// Execute scans the package directory for CRD manifests and applies them to the cluster.
// A returned error triggers infinite retry with exponential backoff.
func (t *task) Execute(ctx context.Context) error {
	if err := t.crdsInstaller.EnsureCRDs(ctx, t.pkg.GetPath()); err != nil {
		t.status.HandleError(t.pkg.GetName(), err)
		return fmt.Errorf("ensure CRDs: %v", err)
	}

	return nil
}
