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

package quota

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func configMap(managed string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: ConfigNamespace, Name: ConfigMapName},
		Data:       map[string]string{"controlPlaneManaged": managed},
	}
}

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name     string
		cm       *corev1.ConfigMap
		expected Config
		metric   string
	}{
		{name: "missing", cm: nil, expected: Config{ControlPlaneManaged: true}, metric: "Enforce"},
		{name: "managed control plane", cm: configMap("true"), expected: Config{ControlPlaneManaged: true}, metric: "Enforce"},
		{name: "control plane not managed by Deckhouse", cm: configMap("false"), expected: Config{}, metric: "NotSupported"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseConfig(tt.cm)
			assert.Equal(t, tt.expected, got)
			assert.Equal(t, tt.metric, got.MetricMode())
		})
	}
}

// The families the module renders tell whether the ledger counts the virtual machines; a ConfigMap
// that does not say is taken as counting them, as before the key was rendered.
func TestConfigCountsVirtualMachines(t *testing.T) {
	families := func(value string) *corev1.ConfigMap {
		cm := configMap("true")
		cm.Data["families"] = value
		return cm
	}
	assert.True(t, ParseConfig(families("lb,vm")).CountsVirtualMachines())
	assert.False(t, ParseConfig(families("lb")).CountsVirtualMachines())
	assert.False(t, ParseConfig(families("")).CountsVirtualMachines())
	assert.True(t, ParseConfig(configMap("true")).CountsVirtualMachines())
}

func TestConfigReaderReusesARead(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, coordinationv1.AddToScheme(scheme))
	require.NoError(t, admissionregistrationv1.AddToScheme(scheme))
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(configMap("true")).Build()
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	r := &ConfigReader{Reader: c, Now: func() time.Time { return now }}

	got, err := r.Read(context.Background())
	require.NoError(t, err)
	assert.True(t, got.ControlPlaneManaged)

	require.NoError(t, c.Update(context.Background(), configMap("false")))
	got, err = r.Read(context.Background())
	require.NoError(t, err)
	assert.True(t, got.ControlPlaneManaged, "a read is reused for a few seconds")

	now = now.Add(configTTL)
	got, err = r.Read(context.Background())
	require.NoError(t, err)
	assert.False(t, got.ControlPlaneManaged)

	require.NoError(t, c.Update(context.Background(), configMap("true")))
	r.Invalidate()
	got, err = r.Read(context.Background())
	require.NoError(t, err)
	assert.True(t, got.ControlPlaneManaged, "a change of an object it reads drops the reused read")
}

func TestConfigReaderReadsTheCanaryOfTheLedger(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, coordinationv1.AddToScheme(scheme))
	require.NoError(t, admissionregistrationv1.AddToScheme(scheme))
	state := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: ConfigNamespace, Name: LedgerStateConfigMapName},
		Data: map[string]string{LedgerStateNotCalledKey: "10.0.0.1:6443,10.0.0.2:6443"}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(configMap("true"), state).Build()
	got, err := (&ConfigReader{Reader: c}).Read(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.1:6443", "10.0.0.2:6443"}, got.WebhookNotCalledBy)

	state.Data[LedgerStateNotCalledKey] = ""
	require.NoError(t, c.Update(context.Background(), state))
	got, err = (&ConfigReader{Reader: c}).Read(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got.WebhookNotCalledBy)
}

func TestIsConfigObject(t *testing.T) {
	assert.True(t, IsConfigObject("", LedgerWebhookName))
	assert.True(t, IsConfigObject(ConfigNamespace, LedgerStateConfigMapName))
	assert.True(t, IsConfigObject(ConfigNamespace, SuspendConfigMapName))
	assert.False(t, IsConfigObject("team", SuspendConfigMapName))
	assert.False(t, IsConfigObject("", "another-webhook"))
}
