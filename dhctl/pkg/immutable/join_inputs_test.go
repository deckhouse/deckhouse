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

package immutable_test

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/immutable"
	"github.com/deckhouse/deckhouse/dhctl/pkg/immutable/immutabletest"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

// tokenPublishDelay makes the token appear while the first attempt is already
// over: shorter than the loop's interval, so the second attempt finds it.
const tokenPublishDelay = 100 * time.Millisecond

// The bootstrap token of a NodeGroup is minted by an asynchronous node-manager
// hook, so the first read of a young master group finds none. Without a retry
// the whole multi-master bootstrap ends there — after the first master is up.
func TestBuildJoinPayloadFromClusterWaitsForTheBootstrapToken(t *testing.T) {
	immutabletest.NoRetryCollapse(t)

	kubeCl := client.NewFakeKubernetesClient()
	immutabletest.CreateJoinInputsWithoutToken(t, kubeCl)

	// Published after the first attempt has already failed, the way the hook
	// publishes it. Bounded by the context so a lost retry fails the test in
	// seconds instead of sitting out the whole budget.
	go func() {
		time.Sleep(tokenPublishDelay)
		_, _ = kubeCl.CoreV1().Secrets(global.ConfigsNS).
			Create(context.Background(), immutabletest.BootstrapTokenSecret(), metav1.CreateOptions{})
	}()

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	payload, nodeConfigDocument, err := immutable.BuildJoinPayloadFromCluster(
		ctx, kubeCl, immutabletest.MetaConfig(t), "example-master-1", nil, "", "master")
	require.NoError(t, err, "a token that is not published yet is what the wait exists for")

	document, err := base64.StdEncoding.DecodeString(payload)
	require.NoError(t, err)
	require.Contains(t, string(document), immutabletest.BootstrapToken,
		"the payload must carry the token the cluster published")

	documents := immutabletest.PayloadDocuments(t, document)
	require.Len(t, documents, 1, "a joining master gets no ControlPlaneConfig")
	require.Equal(t, string(nodeConfigDocument), documents[0],
		"the join path checks the very bytes the joining machine is handed")
}
