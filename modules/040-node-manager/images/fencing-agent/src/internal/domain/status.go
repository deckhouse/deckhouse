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

// Reason is the reason of the Fencing condition on the own Node.
type Reason string

// Reasons of the Fencing condition. The condition status is True only with ReasonArmed.
const (
	ReasonArmed                   Reason = "Armed"
	ReasonAgentStopping           Reason = "AgentStopping"
	ReasonMaintenance             Reason = "Maintenance"
	ReasonGroupDisabled           Reason = "GroupDisabled"
	ReasonWaitingForControlPlane  Reason = "WaitingForControlPlane"
	ReasonNoEvacuationTarget      Reason = "NoEvacuationTarget"
	ReasonWatchdogTimeoutMismatch Reason = "WatchdogTimeoutMismatch"
	ReasonWatchdogDeviceMissing   Reason = "WatchdogDeviceMissing"
	ReasonKernelPanicMismatch     Reason = "KernelPanicMismatch"
	ReasonInvalidKey              Reason = "InvalidKey"
	ReasonNodeIPMismatch          Reason = "NodeIPMismatch"
)

// Status publishes the agent state on the own Node: the Fencing condition,
// the annotations, the Events and the metrics of the condition and the CP source.
// It is the only writer of the own Node.
type Status interface {
	// SetCondition sets the Fencing condition. The last call wins.
	SetCondition(reason Reason, message string)
	SetCPSource(source CPSource)
	// SetProfile sets the value of the profile annotation: the name of the
	// FencingSLAProfile of the group, or "disabled".
	SetProfile(name string)
}

// FeedDecision is the result of one decision about the watchdog feed.
type FeedDecision string

const (
	DecisionFeed FeedDecision = "feed"
	DecisionStop FeedDecision = "stop"
)

// AnswerClass is the class of a poll answer.
type AnswerClass string

const (
	AnswerWitness AnswerClass = "witness"
	// AnswerSupport is an answer that does not give a witness.
	AnswerSupport AnswerClass = "support"
	AnswerSilent  AnswerClass = "silent"
)

// Metrics counts the agent events. The metrics of the condition and of the CP
// source come from Status.
type Metrics interface {
	Decision(decision FeedDecision)
	Poll()
	PollAnswer(network Network, class AnswerClass)
	// ConfigInvalid sets or clears the invalid value flag of the parameter.
	ConfigInvalid(parameter string, invalid bool)
}
