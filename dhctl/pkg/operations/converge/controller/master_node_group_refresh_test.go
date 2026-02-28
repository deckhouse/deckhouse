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

package controller

import (
	stdcontext "context"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	convergecontext "github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/context"
)

func TestMasterNodeGroupController_getNodeInternalIP_NoInternalIP(t *testing.T) {
	kubeCl := client.NewFakeKubernetesClient()
	_, err := kubeCl.CoreV1().Nodes().Create(stdcontext.Background(), &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "master-0"},
		Status: corev1.NodeStatus{
			Addresses: []corev1.NodeAddress{
				{Type: corev1.NodeExternalIP, Address: "203.0.113.10"},
			},
		},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	ctx := convergecontext.NewContext(stdcontext.Background(), convergecontext.Params{KubeClient: kubeCl})
	controller := newTestMasterNodeGroupController()

	_, err = controller.getNodeInternalIP(ctx, "master-0")
	require.Error(t, err)
	require.ErrorContains(t, err, "has no InternalIP")
}

func TestMasterNodeGroupController_refreshCloudConfigForMasterUpdate_Error(t *testing.T) {
	kubeCl := client.NewFakeKubernetesClient()
	rawCtx, cancel := stdcontext.WithTimeout(stdcontext.Background(), 100*time.Millisecond)
	defer cancel()

	ctx := convergecontext.NewContext(rawCtx, convergecontext.Params{KubeClient: kubeCl})
	controller := newTestMasterNodeGroupController()

	err := controller.refreshCloudConfigForMasterUpdate(ctx, "master-0", "")
	require.Error(t, err)
	require.ErrorContains(t, err, "failed to refresh cloud config before converging node")
}

func TestMasterNodeGroupController_makeMasterNodeVariablesRefresher_RefreshesCloudConfig(t *testing.T) {
	kubeCl := client.NewFakeKubernetesClient()
	cloudConfig := "#!/bin/bash\nexport PACKAGES_PROXY_ADDRESSES=\"10.241.36.14:4219\""
	_, err := kubeCl.CoreV1().Secrets("d8-cloud-instance-manager").Create(stdcontext.Background(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "manual-bootstrap-for-master",
			Namespace: "d8-cloud-instance-manager",
		},
		Data: map[string][]byte{
			"cloud-config": []byte(cloudConfig),
		},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	ctx := convergecontext.NewContext(stdcontext.Background(), convergecontext.Params{KubeClient: kubeCl})
	controller := newTestMasterNodeGroupController()
	metaConfig := &config.MetaConfig{}

	refresher := controller.makeMasterNodeVariablesRefresher(ctx, metaConfig, "master-0", 0, "10.241.32.22")
	data, err := refresher(stdcontext.Background())
	require.NoError(t, err)
	require.NotEmpty(t, data)

	expectedCloudConfig := base64.StdEncoding.EncodeToString([]byte(cloudConfig))
	require.Equal(t, expectedCloudConfig, controller.cloudConfig)

	var vars map[string]any
	err = json.Unmarshal(data, &vars)
	require.NoError(t, err)
	require.Equal(t, expectedCloudConfig, vars["cloudConfig"])
}

func newTestMasterNodeGroupController() *MasterNodeGroupController {
	return &MasterNodeGroupController{
		NodeGroupController: &NodeGroupController{
			name: global.MasterNodeGroupName,
		},
	}
}
