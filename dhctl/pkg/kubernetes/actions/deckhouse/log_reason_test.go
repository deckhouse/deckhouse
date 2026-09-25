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

package deckhouse

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

// TestPodNotRunningReason: the wait printed "Deckhouse pod found: … (Pending)" once per poll for
// fifteen minutes while the reason sat in the pod the whole time. Kubernetes records it; nothing
// was reading it back.
func TestPodNotRunningReason(t *testing.T) {
	tests := []struct {
		name string
		pod  *corev1.Pod
		want string
	}{
		{
			// The one an air-gapped registry produces, and the one the fifteen minutes were
			// usually spent on.
			name: "the image cannot be pulled",
			pod: &corev1.Pod{Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{{
					Name: "deckhouse",
					State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{
						Reason:  "ImagePullBackOff",
						Message: `Back-off pulling image "registry.example.com/deckhouse/ee:v1.70.3"`,
					}},
				}},
			}},
			want: `container "deckhouse" is ImagePullBackOff: Back-off pulling image "registry.example.com/deckhouse/ee:v1.70.3"`,
		},
		{
			name: "the container crashes on start",
			pod: &corev1.Pod{Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{{
					Name:  "deckhouse",
					State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "Error"}},
				}},
			}},
			want: `container "deckhouse" is Error`,
		},
		{
			// An init container fails before the main one has a status at all.
			name: "an init container is stuck",
			pod: &corev1.Pod{Status: corev1.PodStatus{
				InitContainerStatuses: []corev1.ContainerStatus{{
					Name:  "init",
					State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ErrImagePull"}},
				}},
			}},
			want: `container "init" is ErrImagePull`,
		},
		{
			// Nothing has started because nothing will place it.
			name: "nothing will schedule it",
			pod: &corev1.Pod{Status: corev1.PodStatus{
				Conditions: []corev1.PodCondition{{
					Type:    corev1.PodScheduled,
					Status:  corev1.ConditionFalse,
					Reason:  "Unschedulable",
					Message: "0/1 nodes are available: 1 node(s) had untolerated taint",
				}},
			}},
			want: "Unschedulable: 0/1 nodes are available: 1 node(s) had untolerated taint",
		},
		{
			name: "the pod says nothing yet",
			pod:  &corev1.Pod{Status: corev1.PodStatus{}},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, podNotRunningReason(tt.pod))
		})
	}
}

// TestKubernetesAPIWaitAdvice: the wait spends nearly four minutes and then reports the raw
// transport error. Each of these means a different thing is wrong and a different place to look.
func TestKubernetesAPIWaitAdvice(t *testing.T) {
	tests := []struct {
		name string
		err  string
		want string // a phrase that must appear; "" means no advice at all
	}{
		{
			name: "nothing listening behind the tunnel",
			err:  "kubernetes API is not Ready: Get \"http://127.0.0.1:6445/version\": dial tcp 127.0.0.1:6445: connect: connection refused",
			want: "crictl ps -a --name kube-apiserver",
		},
		{
			name: "requests go unanswered",
			err:  "kubernetes API is not Ready: dial tcp 127.0.0.1:6445: i/o timeout",
			want: "port 6445",
		},
		{
			// The sudo case: kubectl proxy cannot read admin.conf and exits mid-request.
			name: "the proxy keeps exiting",
			err:  "kubernetes API is not Ready: Get \"http://127.0.0.1:6445/version\": EOF",
			want: "admin.conf readable",
		},
		{
			name: "the certificate is rejected",
			err:  "kubernetes API is not Ready: x509: certificate has expired or is not yet valid",
			want: "clock is wrong",
		},
		{
			// Nothing recognised: say nothing rather than guess. The loop's own message
			// already carries the error.
			name: "an error we have no advice for",
			err:  "kubernetes API is not Ready: some unrecognised failure",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := kubernetesAPIWaitAdvice(errors.New(tt.err))
			if tt.want == "" {
				assert.Empty(t, got)
				return
			}
			assert.Contains(t, got, tt.want)
		})
	}
}
