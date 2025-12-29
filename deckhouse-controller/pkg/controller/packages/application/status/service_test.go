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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

func TestComputeAndApplyConditions_InitialInstall(t *testing.T) {
	tests := []struct {
		name                    string
		packageVersion          string
		appCurrentVersion       string
		internalConditions      []status.Condition
		expectedVersion         string
		expectedVersionCommited bool
	}{
		{
			name:              "initial install - version committed when ReadyInCluster is True",
			packageVersion:    "1.0.0",
			appCurrentVersion: "",
			internalConditions: []status.Condition{
				{Type: status.ConditionDownloaded, Status: metav1.ConditionTrue},
				{Type: status.ConditionReadyOnFilesystem, Status: metav1.ConditionTrue},
				{Type: status.ConditionReadyInRuntime, Status: metav1.ConditionTrue},
				{Type: status.ConditionReadyInCluster, Status: metav1.ConditionTrue},
			},
			expectedVersion:         "1.0.0",
			expectedVersionCommited: true,
		},
		{
			name:              "initial install - version NOT committed when ReadyInCluster is False",
			packageVersion:    "1.0.0",
			appCurrentVersion: "",
			internalConditions: []status.Condition{
				{Type: status.ConditionDownloaded, Status: metav1.ConditionTrue},
				{Type: status.ConditionReadyOnFilesystem, Status: metav1.ConditionTrue},
				{Type: status.ConditionReadyInRuntime, Status: metav1.ConditionTrue},
				{Type: status.ConditionReadyInCluster, Status: metav1.ConditionFalse},
			},
			expectedVersion:         "",
			expectedVersionCommited: false,
		},
		{
			name:              "initial install - version NOT committed when ReadyInCluster is missing",
			packageVersion:    "1.0.0",
			appCurrentVersion: "",
			internalConditions: []status.Condition{
				{Type: status.ConditionDownloaded, Status: metav1.ConditionTrue},
				{Type: status.ConditionReadyOnFilesystem, Status: metav1.ConditionTrue},
			},
			expectedVersion:         "",
			expectedVersionCommited: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a service with a mock getter
			svc := &Service{
				getter: func(name string) status.Status {
					return status.Status{
						Version:    tt.packageVersion,
						Conditions: tt.internalConditions,
					}
				},
				mapper: buildMapper(),
			}

			// Create app with initial state
			app := &v1alpha1.Application{
				Status: v1alpha1.ApplicationStatus{
					CurrentVersion: &v1alpha1.ApplicationStatusVersion{
						Version: tt.appCurrentVersion,
					},
				},
			}

			// Execute
			svc.computeAndApplyConditions("test-event", app)

			// Assert
			if tt.expectedVersionCommited {
				assert.Equal(t, tt.expectedVersion, app.Status.CurrentVersion.Version,
					"version should be committed")
			} else {
				assert.Equal(t, tt.appCurrentVersion, app.Status.CurrentVersion.Version,
					"version should NOT be committed")
			}
		})
	}
}

func TestComputeAndApplyConditions_Update(t *testing.T) {
	tests := []struct {
		name                    string
		packageVersion          string
		appCurrentVersion       string
		existingConditions      []v1alpha1.ApplicationStatusCondition
		internalConditions      []status.Condition
		expectedVersion         string
		expectedVersionCommited bool
	}{
		{
			name:              "update - version committed when ReadyInCluster is True",
			packageVersion:    "2.0.0",
			appCurrentVersion: "1.0.0",
			existingConditions: []v1alpha1.ApplicationStatusCondition{
				{Type: ConditionInstalled, Status: corev1.ConditionTrue},
			},
			internalConditions: []status.Condition{
				{Type: status.ConditionDownloaded, Status: metav1.ConditionTrue},
				{Type: status.ConditionReadyOnFilesystem, Status: metav1.ConditionTrue},
				{Type: status.ConditionReadyInRuntime, Status: metav1.ConditionTrue},
				{Type: status.ConditionReadyInCluster, Status: metav1.ConditionTrue},
			},
			expectedVersion:         "2.0.0",
			expectedVersionCommited: true,
		},
		{
			name:              "update - version NOT committed when ReadyInCluster is False (download in progress)",
			packageVersion:    "2.0.0",
			appCurrentVersion: "1.0.0",
			existingConditions: []v1alpha1.ApplicationStatusCondition{
				{Type: ConditionInstalled, Status: corev1.ConditionTrue},
			},
			internalConditions: []status.Condition{
				{Type: status.ConditionDownloaded, Status: metav1.ConditionTrue},
				{Type: status.ConditionReadyOnFilesystem, Status: metav1.ConditionFalse},
				{Type: status.ConditionReadyInRuntime, Status: metav1.ConditionFalse},
				{Type: status.ConditionReadyInCluster, Status: metav1.ConditionFalse},
			},
			expectedVersion:         "1.0.0",
			expectedVersionCommited: false,
		},
		{
			name:              "update - version NOT committed when ReadyInCluster is False",
			packageVersion:    "2.0.0",
			appCurrentVersion: "1.0.0",
			existingConditions: []v1alpha1.ApplicationStatusCondition{
				{Type: ConditionInstalled, Status: corev1.ConditionTrue},
			},
			internalConditions: []status.Condition{
				{Type: status.ConditionDownloaded, Status: metav1.ConditionTrue},
				{Type: status.ConditionReadyOnFilesystem, Status: metav1.ConditionTrue},
				{Type: status.ConditionReadyInRuntime, Status: metav1.ConditionTrue},
				{Type: status.ConditionReadyInCluster, Status: metav1.ConditionFalse},
			},
			expectedVersion:         "1.0.0",
			expectedVersionCommited: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a service with a mock getter
			svc := &Service{
				getter: func(name string) status.Status {
					return status.Status{
						Version:    tt.packageVersion,
						Conditions: tt.internalConditions,
					}
				},
				mapper: buildMapper(),
			}

			// Create app with initial state
			app := &v1alpha1.Application{
				Status: v1alpha1.ApplicationStatus{
					CurrentVersion: &v1alpha1.ApplicationStatusVersion{
						Version: tt.appCurrentVersion,
					},
					Conditions: tt.existingConditions,
				},
			}

			// Execute
			svc.computeAndApplyConditions("test-event", app)

			// Assert
			if tt.expectedVersionCommited {
				assert.Equal(t, tt.expectedVersion, app.Status.CurrentVersion.Version,
					"version should be committed")
			} else {
				assert.Equal(t, tt.appCurrentVersion, app.Status.CurrentVersion.Version,
					"version should NOT be committed")
			}
		})
	}
}

// TestComputeAndApplyConditions_VersionCommitOnlyWhenReady tests that version is only
// committed when ReadyInCluster becomes True.
func TestComputeAndApplyConditions_VersionCommitOnlyWhenReady(t *testing.T) {
	t.Run("version stays uncommitted during intermediate update states", func(t *testing.T) {
		// Simulate update process:
		// Event 1: downloaded=true, but ReadyInCluster=false
		// Event 2: more conditions ready, but ReadyInCluster=false
		// Event 3: ReadyInCluster=true -> version committed

		svc := &Service{
			mapper: buildMapper(),
		}

		app := &v1alpha1.Application{
			Status: v1alpha1.ApplicationStatus{
				CurrentVersion: &v1alpha1.ApplicationStatusVersion{
					Version: "1.0.0",
				},
				Conditions: []v1alpha1.ApplicationStatusCondition{
					{Type: ConditionInstalled, Status: corev1.ConditionTrue},
				},
			},
		}

		// Event 1: Package version 2.0.0, downloaded but not ready
		svc.getter = func(name string) status.Status {
			return status.Status{
				Version: "2.0.0",
				Conditions: []status.Condition{
					{Type: status.ConditionDownloaded, Status: metav1.ConditionTrue},
					{Type: status.ConditionReadyOnFilesystem, Status: metav1.ConditionFalse},
					{Type: status.ConditionReadyInRuntime, Status: metav1.ConditionFalse},
					{Type: status.ConditionReadyInCluster, Status: metav1.ConditionFalse},
				},
			}
		}
		svc.computeAndApplyConditions("test-event", app)

		// Version should NOT be committed yet
		assert.Equal(t, "1.0.0", app.Status.CurrentVersion.Version,
			"version should not be committed during intermediate state")

		// Event 2: Still in progress, filesystem becomes ready but cluster not ready
		svc.getter = func(name string) status.Status {
			return status.Status{
				Version: "2.0.0",
				Conditions: []status.Condition{
					{Type: status.ConditionDownloaded, Status: metav1.ConditionTrue},
					{Type: status.ConditionReadyOnFilesystem, Status: metav1.ConditionTrue},
					{Type: status.ConditionReadyInRuntime, Status: metav1.ConditionTrue},
					{Type: status.ConditionReadyInCluster, Status: metav1.ConditionFalse},
				},
			}
		}
		svc.computeAndApplyConditions("test-event", app)

		// Version should STILL not be committed
		assert.Equal(t, "1.0.0", app.Status.CurrentVersion.Version,
			"version should not be committed until ReadyInCluster is True")

		// Event 3: All conditions become True
		svc.getter = func(name string) status.Status {
			return status.Status{
				Version: "2.0.0",
				Conditions: []status.Condition{
					{Type: status.ConditionDownloaded, Status: metav1.ConditionTrue},
					{Type: status.ConditionReadyOnFilesystem, Status: metav1.ConditionTrue},
					{Type: status.ConditionReadyInRuntime, Status: metav1.ConditionTrue},
					{Type: status.ConditionReadyInCluster, Status: metav1.ConditionTrue},
				},
			}
		}
		svc.computeAndApplyConditions("test-event", app)

		// NOW version should be committed
		assert.Equal(t, "2.0.0", app.Status.CurrentVersion.Version,
			"version should be committed when ReadyInCluster becomes True")
	})
}

func TestComputeAndApplyConditions_NoVersionChange(t *testing.T) {
	t.Run("same version - no version commit", func(t *testing.T) {
		svc := &Service{
			getter: func(name string) status.Status {
				return status.Status{
					Version: "1.0.0",
					Conditions: []status.Condition{
						{Type: status.ConditionDownloaded, Status: metav1.ConditionTrue},
						{Type: status.ConditionReadyOnFilesystem, Status: metav1.ConditionTrue},
						{Type: status.ConditionReadyInRuntime, Status: metav1.ConditionTrue},
						{Type: status.ConditionReadyInCluster, Status: metav1.ConditionTrue},
					},
				}
			},
			mapper: buildMapper(),
		}

		app := &v1alpha1.Application{
			Status: v1alpha1.ApplicationStatus{
				CurrentVersion: &v1alpha1.ApplicationStatusVersion{
					Version: "1.0.0", // Same as package version
				},
				Conditions: []v1alpha1.ApplicationStatusCondition{
					{Type: ConditionInstalled, Status: corev1.ConditionTrue},
				},
			},
		}

		svc.computeAndApplyConditions("test-event", app)

		// Version should remain unchanged (no re-commit needed)
		assert.Equal(t, "1.0.0", app.Status.CurrentVersion.Version)
	})
}

func TestComputeAndApplyConditions_ConditionsAreSet(t *testing.T) {
	t.Run("internal conditions are copied to app status", func(t *testing.T) {
		svc := &Service{
			getter: func(name string) status.Status {
				return status.Status{
					Version: "1.0.0",
					Conditions: []status.Condition{
						{
							Type:    status.ConditionDownloaded,
							Status:  metav1.ConditionTrue,
							Reason:  "DownloadComplete",
							Message: "Package downloaded successfully",
						},
					},
				}
			},
			mapper: buildMapper(),
		}

		app := &v1alpha1.Application{
			Status: v1alpha1.ApplicationStatus{
				CurrentVersion: &v1alpha1.ApplicationStatusVersion{
					Version: "",
				},
			},
		}

		svc.computeAndApplyConditions("test-event", app)

		// Check internal conditions were set
		assert.Len(t, app.Status.InternalConditions, 1)
		assert.Equal(t, string(status.ConditionDownloaded), app.Status.InternalConditions[0].Type)
		assert.Equal(t, corev1.ConditionTrue, app.Status.InternalConditions[0].Status)
		assert.Equal(t, "DownloadComplete", app.Status.InternalConditions[0].Reason)
	})
}
