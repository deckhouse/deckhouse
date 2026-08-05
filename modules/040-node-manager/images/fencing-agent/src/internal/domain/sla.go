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

type SLA struct {
	Memberlist MemberlistTuning
	Fallback   FallbackTuning
	Rejoin     RejoinTuning
	Watchdog   WatchdogTuning
}

// MemberlistTuning maps 1:1 onto hashicorp/memberlist Config fields.
type MemberlistTuning struct {
	ProbeInterval           time.Duration
	ProbeTimeout            time.Duration
	SuspicionMult           int
	SuspicionMaxTimeoutMult int
	IndirectChecks          int
	AwarenessMaxMultiplier  int
	GossipInterval          time.Duration
	RetransmitMult          int
	GossipToTheDeadTime     time.Duration
}

type FallbackTuning struct {
	Heartbeat  time.Duration
	APITimeout time.Duration
}

type RejoinTuning struct {
	Interval    time.Duration
	MaxInterval time.Duration
}

type WatchdogTuning struct {
	FeedInterval time.Duration
	Timeout      time.Duration
}
