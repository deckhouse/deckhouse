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

package evaluator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestResourceQuota(t *testing.T) {
	var e Evaluator = ResourceQuota{}
	assert.Equal(t, "rq", e.Family())

	used := corev1.ResourceList{"pods": resource.MustParse("3")}
	got, err := e.Usage(&corev1.ResourceQuota{Status: corev1.ResourceQuotaStatus{Used: used}})
	require.NoError(t, err)
	assert.Equal(t, used, got)

	_, err = e.Usage(&corev1.Pod{})
	assert.Error(t, err)
}
