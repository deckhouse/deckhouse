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

package controlplane

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	libcon "github.com/deckhouse/lib-connection/pkg"

	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/plan"
)

// recreatedMasterRunner reports the one plan shape that makes AfterAction do its
// work: the master VM was destroyed and created again.
type recreatedMasterRunner struct {
	infrastructure.RunnerInterface
}

func (recreatedMasterRunner) GetChangesInPlan() int { return plan.HasDestructiveChanges }

func (recreatedMasterRunner) HasVMDestruction() bool { return true }

func (recreatedMasterRunner) GetInfrastructureOutput(_ context.Context, _ string) ([]byte, error) {
	return []byte(`"10.12.1.10"`), nil
}

func (recreatedMasterRunner) GetState() ([]byte, error) { return []byte("{}"), nil }

type sshProviderWithoutClient struct {
	libcon.SSHProvider
}

func (sshProviderWithoutClient) Client(_ context.Context) (libcon.SSHClient, error) {
	return nil, fmt.Errorf("no ssh hosts")
}

func TestAfterActionReportsUnavailableSSHClient(t *testing.T) {
	hook := NewHookForUpdatePipeline(
		nil,
		sshProviderWithoutClient{},
		map[string]string{"cluster-master-0": "10.12.1.10"},
		false,
		true,
		false,
	).WithNodeToConverge("cluster-master-0")

	err := hook.AfterAction(t.Context(), recreatedMasterRunner{})

	require.ErrorContains(t, err, "get ssh client")
}
