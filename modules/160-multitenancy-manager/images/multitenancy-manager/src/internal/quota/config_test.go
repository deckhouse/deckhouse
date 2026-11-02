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

func TestConfigReaderReusesARead(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
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
}
