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

package domain

import "time"

// CPSource is the source of the control plane state.
type CPSource string

const (
	// CPSourceProxy reads /upstreams of the local kubernetes-api-proxy.
	CPSourceProxy CPSource = "proxy"
	// CPSourceSelf probes the control plane from the agent.
	CPSourceSelf CPSource = "self"
)

// CPState is one read of the control plane state.
type CPState struct {
	Source CPSource
	// Visible is the result of the control plane visibility rule.
	Visible bool
	// Seen is true when the source has a ready probe. With the proxy source
	// the probe can be from before the agent start.
	Seen bool
	// LastReadyAgo is the age of the last ready probe at ReadAt.
	LastReadyAgo time.Duration
	// ProbeSeq and ProbeOK are the number and the result of the last finished probe cycle.
	ProbeSeq uint64
	ProbeOK  bool
	ReadAt   time.Time
}
