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

package monitor

import (
	appsv1 "k8s.io/api/apps/v1"
)

// WorkloadHealth is the per-workload health classification produced by the
// per-type reducers. Consumers (typically a package-level reducer in another
// package) compare against these constants to roll up many workloads into
// one verdict.
type WorkloadHealth int

// WorkloadHealth values, in increasing severity for the typical reduction:
// Current is healthy steady state; InProgress is a rollout in progress;
// Failed is terminal (the controller has given up); Terminating is
// "object has DeletionTimestamp but is still in cache."
const (
	Current WorkloadHealth = iota
	InProgress
	Failed
	Terminating
)

// WorkloadStatus pairs a workload's health with identifying metadata so the
// package-level reducer can build a dominant-cause reason string. Kind is
// the workload kind ("Deployment" or "StatefulSet"), Name is the object's
// metadata.name, and Cause is a short free-form description of why this
// status was reached (e.g. "ProgressDeadlineExceeded", "rolling update").
type WorkloadStatus struct {
	Kind   string
	Name   string
	Health WorkloadHealth
	Cause  string
}

// reduceDeployment classifies a Deployment according to the rules in the
// package-level docs.
func reduceDeployment(d *appsv1.Deployment) WorkloadStatus {
	s := WorkloadStatus{Kind: "Deployment", Name: d.Name}

	if d.DeletionTimestamp != nil {
		s.Health = Terminating
		s.Cause = "deletion in progress"
		return s
	}

	for _, c := range d.Status.Conditions {
		if c.Type == appsv1.DeploymentProgressing && c.Reason == "ProgressDeadlineExceeded" {
			s.Health = Failed
			s.Cause = "ProgressDeadlineExceeded"
			return s
		}
	}

	if d.Generation != d.Status.ObservedGeneration {
		s.Health = InProgress
		s.Cause = "generation not yet observed"
		return s
	}

	if d.Spec.Paused {
		s.Health = InProgress
		s.Cause = "deployment paused"
		return s
	}

	desired := int32(1)
	if d.Spec.Replicas != nil {
		desired = *d.Spec.Replicas
	}

	switch {
	case d.Status.UpdatedReplicas < desired:
		s.Health = InProgress
		s.Cause = "updated replicas below desired"
	case d.Status.Replicas > d.Status.UpdatedReplicas:
		s.Health = InProgress
		s.Cause = "old replicas terminating"
	case d.Status.AvailableReplicas < desired:
		s.Health = InProgress
		s.Cause = "replicas not ready"
	default:
		s.Health = Current
	}
	return s
}

// reduceStatefulSet classifies a StatefulSet.
func reduceStatefulSet(ss *appsv1.StatefulSet) WorkloadStatus {
	s := WorkloadStatus{Kind: "StatefulSet", Name: ss.Name}

	if ss.DeletionTimestamp != nil {
		s.Health = Terminating
		s.Cause = "deletion in progress"
		return s
	}
	if ss.Generation != ss.Status.ObservedGeneration {
		s.Health = InProgress
		s.Cause = "generation not yet observed"
		return s
	}

	desired := int32(1)
	if ss.Spec.Replicas != nil {
		desired = *ss.Spec.Replicas
	}

	switch {
	case ss.Status.CurrentRevision != ss.Status.UpdateRevision:
		s.Health = InProgress
		s.Cause = "rolling update"
	case ss.Status.ReadyReplicas < desired:
		s.Health = InProgress
		s.Cause = "replicas not ready"
	case ss.Status.UpdatedReplicas < desired:
		s.Health = InProgress
		s.Cause = "updated replicas below desired"
	default:
		s.Health = Current
	}
	return s
}
