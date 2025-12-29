// Copyright 2025 Flant JSC
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

package status

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/conditionmapper"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
)

func TestConditionInstalled(t *testing.T) {
	tests := []struct {
		name     string
		status   conditionmapper.Status
		expected *metav1.Condition
	}{
		{
			name: "successful initial installation",
			status: conditionmapper.Status{
				Internal: map[string]metav1.Condition{
					string(status.ConditionDownloaded): {
						Type:   string(status.ConditionDownloaded),
						Status: metav1.ConditionTrue,
					},
					string(status.ConditionReadyOnFilesystem): {
						Type:   string(status.ConditionReadyOnFilesystem),
						Status: metav1.ConditionTrue,
					},
					string(status.ConditionReadyInRuntime): {
						Type:   string(status.ConditionReadyInRuntime),
						Status: metav1.ConditionTrue,
					},
					string(status.ConditionReadyInCluster): {
						Type:   string(status.ConditionReadyInCluster),
						Status: metav1.ConditionTrue,
					},
				},
				External: map[string]metav1.Condition{},
			},
			expected: &metav1.Condition{
				Type:   ConditionInstalled,
				Status: metav1.ConditionTrue,
			},
		},
		{
			name: "already installed, condition not updated",
			status: conditionmapper.Status{
				Internal: map[string]metav1.Condition{
					string(status.ConditionDownloaded): {
						Type:   string(status.ConditionDownloaded),
						Status: metav1.ConditionTrue,
					},
					string(status.ConditionReadyOnFilesystem): {
						Type:   string(status.ConditionReadyOnFilesystem),
						Status: metav1.ConditionTrue,
					},
					string(status.ConditionReadyInRuntime): {
						Type:   string(status.ConditionReadyInRuntime),
						Status: metav1.ConditionTrue,
					},
					string(status.ConditionReadyInCluster): {
						Type:   string(status.ConditionReadyInCluster),
						Status: metav1.ConditionTrue,
					},
				},
				External: map[string]metav1.Condition{
					ConditionInstalled: {
						Type:   ConditionInstalled,
						Status: metav1.ConditionTrue,
					},
				},
			},
			expected: nil,
		},
		{
			name: "download failed",
			status: conditionmapper.Status{
				Internal: map[string]metav1.Condition{
					string(status.ConditionDownloaded): {
						Type:    string(status.ConditionDownloaded),
						Status:  metav1.ConditionFalse,
						Reason:  "DownloadFailed",
						Message: "failed to download package from registry",
					},
					string(status.ConditionReadyOnFilesystem): {
						Type:   string(status.ConditionReadyOnFilesystem),
						Status: metav1.ConditionTrue,
					},
					string(status.ConditionReadyInRuntime): {
						Type:   string(status.ConditionReadyInRuntime),
						Status: metav1.ConditionTrue,
					},
					string(status.ConditionReadyInCluster): {
						Type:   string(status.ConditionReadyInCluster),
						Status: metav1.ConditionTrue,
					},
				},
				External: map[string]metav1.Condition{},
			},
			expected: &metav1.Condition{
				Type:    ConditionInstalled,
				Status:  metav1.ConditionFalse,
				Reason:  "DownloadFailed",
				Message: "failed to download package from registry",
			},
		},
		{
			name: "deployment not ready",
			status: conditionmapper.Status{
				Internal: map[string]metav1.Condition{
					string(status.ConditionDownloaded): {
						Type:   string(status.ConditionDownloaded),
						Status: metav1.ConditionTrue,
					},
					string(status.ConditionReadyOnFilesystem): {
						Type:   string(status.ConditionReadyOnFilesystem),
						Status: metav1.ConditionTrue,
					},
					string(status.ConditionReadyInRuntime): {
						Type:   string(status.ConditionReadyInRuntime),
						Status: metav1.ConditionTrue,
					},
					string(status.ConditionReadyInCluster): {
						Type:    string(status.ConditionReadyInCluster),
						Status:  metav1.ConditionFalse,
						Reason:  "DeploymentFailed",
						Message: "deployment not ready",
					},
				},
				External: map[string]metav1.Condition{},
			},
			expected: &metav1.Condition{
				Type:    ConditionInstalled,
				Status:  metav1.ConditionFalse,
				Reason:  "DeploymentFailed",
				Message: "deployment not ready",
			},
		},
	}

	mapper := buildMapper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapper.Map(tt.status)
			actual := findCondition(result, ConditionInstalled)

			if tt.expected == nil {
				assert.Nil(t, actual, "expected no condition")
			} else {
				assert.NotNil(t, actual, "expected condition but got nil")
				if actual != nil {
					assert.Equal(t, tt.expected.Type, actual.Type)
					assert.Equal(t, tt.expected.Status, actual.Status)
					assert.Equal(t, tt.expected.Reason, actual.Reason)
					assert.Equal(t, tt.expected.Message, actual.Message)
				}
			}
		})
	}
}

func TestConditionUpdateInstalled(t *testing.T) {
	tests := []struct {
		name     string
		status   conditionmapper.Status
		expected *metav1.Condition
	}{
		{
			name: "successful update installation",
			status: conditionmapper.Status{
				VersionChanged: true,
				Internal: map[string]metav1.Condition{
					string(status.ConditionDownloaded): {
						Type:   string(status.ConditionDownloaded),
						Status: metav1.ConditionTrue,
					},
					string(status.ConditionReadyOnFilesystem): {
						Type:   string(status.ConditionReadyOnFilesystem),
						Status: metav1.ConditionTrue,
					},
					string(status.ConditionReadyInRuntime): {
						Type:   string(status.ConditionReadyInRuntime),
						Status: metav1.ConditionTrue,
					},
					string(status.ConditionReadyInCluster): {
						Type:   string(status.ConditionReadyInCluster),
						Status: metav1.ConditionTrue,
					},
				},
				External: map[string]metav1.Condition{
					ConditionInstalled: {
						Type:   ConditionInstalled,
						Status: metav1.ConditionTrue,
					},
				},
			},
			expected: &metav1.Condition{
				Type:   ConditionUpdateInstalled,
				Status: metav1.ConditionTrue,
			},
		},
		{
			name: "version not changed, no update",
			status: conditionmapper.Status{
				VersionChanged: false,
				Internal: map[string]metav1.Condition{
					string(status.ConditionDownloaded): {
						Type:   string(status.ConditionDownloaded),
						Status: metav1.ConditionTrue,
					},
					string(status.ConditionReadyOnFilesystem): {
						Type:   string(status.ConditionReadyOnFilesystem),
						Status: metav1.ConditionTrue,
					},
					string(status.ConditionReadyInRuntime): {
						Type:   string(status.ConditionReadyInRuntime),
						Status: metav1.ConditionTrue,
					},
					string(status.ConditionReadyInCluster): {
						Type:   string(status.ConditionReadyInCluster),
						Status: metav1.ConditionTrue,
					},
				},
				External: map[string]metav1.Condition{
					ConditionInstalled: {
						Type:   ConditionInstalled,
						Status: metav1.ConditionTrue,
					},
				},
			},
			expected: nil,
		},
		{
			name: "version changed but not previously installed",
			status: conditionmapper.Status{
				VersionChanged: true,
				Internal: map[string]metav1.Condition{
					string(status.ConditionDownloaded): {
						Type:   string(status.ConditionDownloaded),
						Status: metav1.ConditionTrue,
					},
					string(status.ConditionReadyOnFilesystem): {
						Type:   string(status.ConditionReadyOnFilesystem),
						Status: metav1.ConditionTrue,
					},
					string(status.ConditionReadyInRuntime): {
						Type:   string(status.ConditionReadyInRuntime),
						Status: metav1.ConditionTrue,
					},
					string(status.ConditionReadyInCluster): {
						Type:   string(status.ConditionReadyInCluster),
						Status: metav1.ConditionTrue,
					},
				},
				External: map[string]metav1.Condition{},
			},
			expected: nil,
		},
		{
			name: "update download failed",
			status: conditionmapper.Status{
				VersionChanged: true,
				Internal: map[string]metav1.Condition{
					string(status.ConditionDownloaded): {
						Type:    string(status.ConditionDownloaded),
						Status:  metav1.ConditionFalse,
						Reason:  "UpdateFailed",
						Message: "failed to download update",
					},
					string(status.ConditionReadyOnFilesystem): {
						Type:   string(status.ConditionReadyOnFilesystem),
						Status: metav1.ConditionTrue,
					},
					string(status.ConditionReadyInRuntime): {
						Type:   string(status.ConditionReadyInRuntime),
						Status: metav1.ConditionTrue,
					},
					string(status.ConditionReadyInCluster): {
						Type:   string(status.ConditionReadyInCluster),
						Status: metav1.ConditionTrue,
					},
				},
				External: map[string]metav1.Condition{
					ConditionInstalled: {
						Type:   ConditionInstalled,
						Status: metav1.ConditionTrue,
					},
				},
			},
			expected: &metav1.Condition{
				Type:    ConditionUpdateInstalled,
				Status:  metav1.ConditionFalse,
				Reason:  "UpdateFailed",
				Message: "failed to download update",
			},
		},
	}

	mapper := buildMapper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapper.Map(tt.status)
			actual := findCondition(result, ConditionUpdateInstalled)

			if tt.expected == nil {
				assert.Nil(t, actual, "expected no condition")
			} else {
				assert.NotNil(t, actual, "expected condition but got nil")
				if actual != nil {
					assert.Equal(t, tt.expected.Type, actual.Type)
					assert.Equal(t, tt.expected.Status, actual.Status)
					assert.Equal(t, tt.expected.Reason, actual.Reason)
					assert.Equal(t, tt.expected.Message, actual.Message)
				}
			}
		})
	}
}

func TestConditionReady(t *testing.T) {
	tests := []struct {
		name     string
		status   conditionmapper.Status
		expected *metav1.Condition
	}{
		{
			name: "application fully ready",
			status: conditionmapper.Status{
				Internal: map[string]metav1.Condition{
					string(status.ConditionDownloaded): {
						Type:   string(status.ConditionDownloaded),
						Status: metav1.ConditionTrue,
					},
					string(status.ConditionReadyOnFilesystem): {
						Type:   string(status.ConditionReadyOnFilesystem),
						Status: metav1.ConditionTrue,
					},
					string(status.ConditionReadyInRuntime): {
						Type:   string(status.ConditionReadyInRuntime),
						Status: metav1.ConditionTrue,
					},
					string(status.ConditionReadyInCluster): {
						Type:   string(status.ConditionReadyInCluster),
						Status: metav1.ConditionTrue,
					},
				},
			},
			expected: &metav1.Condition{
				Type:   ConditionReady,
				Status: metav1.ConditionTrue,
			},
		},
		{
			name: "runtime initialization failed",
			status: conditionmapper.Status{
				Internal: map[string]metav1.Condition{
					string(status.ConditionDownloaded): {
						Type:   string(status.ConditionDownloaded),
						Status: metav1.ConditionTrue,
					},
					string(status.ConditionReadyOnFilesystem): {
						Type:   string(status.ConditionReadyOnFilesystem),
						Status: metav1.ConditionTrue,
					},
					string(status.ConditionReadyInRuntime): {
						Type:    string(status.ConditionReadyInRuntime),
						Status:  metav1.ConditionFalse,
						Reason:  "RuntimeError",
						Message: "runtime initialization failed",
					},
					string(status.ConditionReadyInCluster): {
						Type:   string(status.ConditionReadyInCluster),
						Status: metav1.ConditionTrue,
					},
				},
			},
			expected: &metav1.Condition{
				Type:    ConditionReady,
				Status:  metav1.ConditionFalse,
				Reason:  "RuntimeError",
				Message: "runtime initialization failed",
			},
		},
		{
			name: "multiple failures",
			status: conditionmapper.Status{
				Internal: map[string]metav1.Condition{
					string(status.ConditionDownloaded): {
						Type:    string(status.ConditionDownloaded),
						Status:  metav1.ConditionFalse,
						Reason:  "DownloadFailed",
						Message: "download failed",
					},
					string(status.ConditionReadyOnFilesystem): {
						Type:    string(status.ConditionReadyOnFilesystem),
						Status:  metav1.ConditionFalse,
						Reason:  "FilesystemError",
						Message: "filesystem error",
					},
					string(status.ConditionReadyInRuntime): {
						Type:   string(status.ConditionReadyInRuntime),
						Status: metav1.ConditionTrue,
					},
					string(status.ConditionReadyInCluster): {
						Type:   string(status.ConditionReadyInCluster),
						Status: metav1.ConditionTrue,
					},
				},
			},
			expected: &metav1.Condition{
				Type:    ConditionReady,
				Status:  metav1.ConditionFalse,
				Reason:  "DownloadFailed",
				Message: "download failed",
			},
		},
	}

	mapper := buildMapper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapper.Map(tt.status)
			actual := findCondition(result, ConditionReady)

			if tt.expected == nil {
				assert.Nil(t, actual, "expected no condition")
			} else {
				assert.NotNil(t, actual, "expected condition but got nil")
				if actual != nil {
					assert.Equal(t, tt.expected.Type, actual.Type)
					assert.Equal(t, tt.expected.Status, actual.Status)
					assert.Equal(t, tt.expected.Reason, actual.Reason)
					assert.Equal(t, tt.expected.Message, actual.Message)
				}
			}
		})
	}
}

func TestConditionPartiallyDegraded(t *testing.T) {
	tests := []struct {
		name     string
		status   conditionmapper.Status
		expected *metav1.Condition
	}{
		{
			name: "application not degraded",
			status: conditionmapper.Status{
				Internal: map[string]metav1.Condition{
					string(status.ConditionReadyInRuntime): {
						Type:   string(status.ConditionReadyInRuntime),
						Status: metav1.ConditionTrue,
					},
					string(status.ConditionReadyInCluster): {
						Type:   string(status.ConditionReadyInCluster),
						Status: metav1.ConditionTrue,
					},
				},
			},
			expected: &metav1.Condition{
				Type:   ConditionPartiallyDegraded,
				Status: metav1.ConditionFalse,
			},
		},
		{
			name: "runtime partially degraded",
			status: conditionmapper.Status{
				Internal: map[string]metav1.Condition{
					string(status.ConditionReadyInRuntime): {
						Type:    string(status.ConditionReadyInRuntime),
						Status:  metav1.ConditionFalse,
						Reason:  "RuntimeDegraded",
						Message: "runtime partially degraded",
					},
					string(status.ConditionReadyInCluster): {
						Type:   string(status.ConditionReadyInCluster),
						Status: metav1.ConditionTrue,
					},
				},
			},
			expected: &metav1.Condition{
				Type:    ConditionPartiallyDegraded,
				Status:  metav1.ConditionTrue,
				Reason:  "RuntimeDegraded",
				Message: "runtime partially degraded",
			},
		},
		{
			name: "cluster partially degraded",
			status: conditionmapper.Status{
				Internal: map[string]metav1.Condition{
					string(status.ConditionReadyInRuntime): {
						Type:   string(status.ConditionReadyInRuntime),
						Status: metav1.ConditionTrue,
					},
					string(status.ConditionReadyInCluster): {
						Type:    string(status.ConditionReadyInCluster),
						Status:  metav1.ConditionFalse,
						Reason:  "ClusterDegraded",
						Message: "cluster partially degraded",
					},
				},
			},
			expected: &metav1.Condition{
				Type:    ConditionPartiallyDegraded,
				Status:  metav1.ConditionTrue,
				Reason:  "ClusterDegraded",
				Message: "cluster partially degraded",
			},
		},
		{
			name: "both runtime and cluster degraded",
			status: conditionmapper.Status{
				Internal: map[string]metav1.Condition{
					string(status.ConditionReadyInRuntime): {
						Type:    string(status.ConditionReadyInRuntime),
						Status:  metav1.ConditionFalse,
						Reason:  "RuntimeDegraded",
						Message: "runtime degraded",
					},
					string(status.ConditionReadyInCluster): {
						Type:    string(status.ConditionReadyInCluster),
						Status:  metav1.ConditionFalse,
						Reason:  "ClusterDegraded",
						Message: "cluster degraded",
					},
				},
			},
			expected: &metav1.Condition{
				Type:    ConditionPartiallyDegraded,
				Status:  metav1.ConditionTrue,
				Reason:  "RuntimeDegraded",
				Message: "runtime degraded",
			},
		},
	}

	mapper := buildMapper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapper.Map(tt.status)
			actual := findCondition(result, ConditionPartiallyDegraded)

			if tt.expected == nil {
				assert.Nil(t, actual, "expected no condition")
			} else {
				assert.NotNil(t, actual, "expected condition but got nil")
				if actual != nil {
					assert.Equal(t, tt.expected.Type, actual.Type)
					assert.Equal(t, tt.expected.Status, actual.Status)
					assert.Equal(t, tt.expected.Reason, actual.Reason)
					assert.Equal(t, tt.expected.Message, actual.Message)
				}
			}
		})
	}
}

func TestConditionManaged(t *testing.T) {
	tests := []struct {
		name     string
		status   conditionmapper.Status
		expected *metav1.Condition
	}{
		{
			name: "application is managed successfully",
			status: conditionmapper.Status{
				Internal: map[string]metav1.Condition{
					string(status.ConditionReadyInCluster): {
						Type:   string(status.ConditionReadyInCluster),
						Status: metav1.ConditionTrue,
					},
					string(status.ConditionHooksProcessed): {
						Type:   string(status.ConditionHooksProcessed),
						Status: metav1.ConditionTrue,
					},
				},
			},
			expected: &metav1.Condition{
				Type:   ConditionManaged,
				Status: metav1.ConditionTrue,
			},
		},
		{
			name: "cluster resources not ready",
			status: conditionmapper.Status{
				Internal: map[string]metav1.Condition{
					string(status.ConditionReadyInCluster): {
						Type:    string(status.ConditionReadyInCluster),
						Status:  metav1.ConditionFalse,
						Reason:  "ClusterNotReady",
						Message: "cluster resources not ready",
					},
					string(status.ConditionHooksProcessed): {
						Type:   string(status.ConditionHooksProcessed),
						Status: metav1.ConditionTrue,
					},
				},
			},
			expected: &metav1.Condition{
				Type:    ConditionManaged,
				Status:  metav1.ConditionFalse,
				Reason:  "ClusterNotReady",
				Message: "cluster resources not ready",
			},
		},
		{
			name: "hooks processing failed",
			status: conditionmapper.Status{
				Internal: map[string]metav1.Condition{
					string(status.ConditionReadyInCluster): {
						Type:   string(status.ConditionReadyInCluster),
						Status: metav1.ConditionTrue,
					},
					string(status.ConditionHooksProcessed): {
						Type:    string(status.ConditionHooksProcessed),
						Status:  metav1.ConditionFalse,
						Reason:  "HooksFailed",
						Message: "hooks processing failed",
					},
				},
			},
			expected: &metav1.Condition{
				Type:    ConditionManaged,
				Status:  metav1.ConditionFalse,
				Reason:  "HooksFailed",
				Message: "hooks processing failed",
			},
		},
		{
			name: "both cluster and hooks failed",
			status: conditionmapper.Status{
				Internal: map[string]metav1.Condition{
					string(status.ConditionReadyInCluster): {
						Type:    string(status.ConditionReadyInCluster),
						Status:  metav1.ConditionFalse,
						Reason:  "ClusterNotReady",
						Message: "cluster not ready",
					},
					string(status.ConditionHooksProcessed): {
						Type:    string(status.ConditionHooksProcessed),
						Status:  metav1.ConditionFalse,
						Reason:  "HooksFailed",
						Message: "hooks failed",
					},
				},
			},
			expected: &metav1.Condition{
				Type:    ConditionManaged,
				Status:  metav1.ConditionFalse,
				Reason:  "ClusterNotReady",
				Message: "cluster not ready",
			},
		},
	}

	mapper := buildMapper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapper.Map(tt.status)
			actual := findCondition(result, ConditionManaged)

			if tt.expected == nil {
				assert.Nil(t, actual, "expected no condition")
			} else {
				assert.NotNil(t, actual, "expected condition but got nil")
				if actual != nil {
					assert.Equal(t, tt.expected.Type, actual.Type)
					assert.Equal(t, tt.expected.Status, actual.Status)
					assert.Equal(t, tt.expected.Reason, actual.Reason)
					assert.Equal(t, tt.expected.Message, actual.Message)
				}
			}
		})
	}
}

func TestConditionConfigurationApplied(t *testing.T) {
	tests := []struct {
		name     string
		status   conditionmapper.Status
		expected *metav1.Condition
	}{
		{
			name: "configuration applied successfully",
			status: conditionmapper.Status{
				Internal: map[string]metav1.Condition{
					string(status.ConditionSettingsValid): {
						Type:   string(status.ConditionSettingsValid),
						Status: metav1.ConditionTrue,
					},
					string(status.ConditionHooksProcessed): {
						Type:   string(status.ConditionHooksProcessed),
						Status: metav1.ConditionTrue,
					},
				},
			},
			expected: &metav1.Condition{
				Type:   ConditionConfigurationApplied,
				Status: metav1.ConditionTrue,
			},
		},
		{
			name: "settings validation failed",
			status: conditionmapper.Status{
				Internal: map[string]metav1.Condition{
					string(status.ConditionSettingsValid): {
						Type:    string(status.ConditionSettingsValid),
						Status:  metav1.ConditionFalse,
						Reason:  "ValidationFailed",
						Message: "settings validation failed",
					},
					string(status.ConditionHooksProcessed): {
						Type:   string(status.ConditionHooksProcessed),
						Status: metav1.ConditionTrue,
					},
				},
			},
			expected: &metav1.Condition{
				Type:    ConditionConfigurationApplied,
				Status:  metav1.ConditionFalse,
				Reason:  "ValidationFailed",
				Message: "settings validation failed",
			},
		},
		{
			name: "hooks processing error",
			status: conditionmapper.Status{
				Internal: map[string]metav1.Condition{
					string(status.ConditionSettingsValid): {
						Type:   string(status.ConditionSettingsValid),
						Status: metav1.ConditionTrue,
					},
					string(status.ConditionHooksProcessed): {
						Type:    string(status.ConditionHooksProcessed),
						Status:  metav1.ConditionFalse,
						Reason:  "HooksError",
						Message: "hooks processing error",
					},
				},
			},
			expected: &metav1.Condition{
				Type:    ConditionConfigurationApplied,
				Status:  metav1.ConditionFalse,
				Reason:  "HooksError",
				Message: "hooks processing error",
			},
		},
		{
			name: "both settings and hooks failed",
			status: conditionmapper.Status{
				Internal: map[string]metav1.Condition{
					string(status.ConditionSettingsValid): {
						Type:    string(status.ConditionSettingsValid),
						Status:  metav1.ConditionFalse,
						Reason:  "InvalidSettings",
						Message: "invalid settings",
					},
					string(status.ConditionHooksProcessed): {
						Type:    string(status.ConditionHooksProcessed),
						Status:  metav1.ConditionFalse,
						Reason:  "HooksError",
						Message: "hooks error",
					},
				},
			},
			expected: &metav1.Condition{
				Type:    ConditionConfigurationApplied,
				Status:  metav1.ConditionFalse,
				Reason:  "InvalidSettings",
				Message: "invalid settings",
			},
		},
	}

	mapper := buildMapper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapper.Map(tt.status)
			actual := findCondition(result, ConditionConfigurationApplied)

			if tt.expected == nil {
				assert.Nil(t, actual, "expected no condition")
			} else {
				assert.NotNil(t, actual, "expected condition but got nil")
				if actual != nil {
					assert.Equal(t, tt.expected.Type, actual.Type)
					assert.Equal(t, tt.expected.Status, actual.Status)
					assert.Equal(t, tt.expected.Reason, actual.Reason)
					assert.Equal(t, tt.expected.Message, actual.Message)
				}
			}
		})
	}
}

func findCondition(conditions []metav1.Condition, condType string) *metav1.Condition {
	for _, c := range conditions {
		if c.Type == condType {
			return &c
		}
	}
	return nil
}
