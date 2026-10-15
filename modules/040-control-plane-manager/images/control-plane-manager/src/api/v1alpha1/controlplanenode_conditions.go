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

package v1alpha1

// Per-component pod conditions: True = the component's static pod is running and ready.
const (
	CPNConditionEtcdPodReady              = "EtcdPodReady"
	CPNConditionAPIServerPodReady         = "APIServerPodReady"
	CPNConditionControllerManagerPodReady = "ControllerManagerPodReady"
	CPNConditionSchedulerPodReady         = "SchedulerPodReady"
)

const CPNConditionCertificatesHealthy = "CertificatesHealthy"

// Per-component sync conditions: True = no active operation changes the component and the
// applied checksums (status.components) match the desired ones (spec).
const (
	CPNConditionEtcdInSync              = "EtcdInSync"
	CPNConditionAPIServerInSync         = "APIServerInSync"
	CPNConditionControllerManagerInSync = "ControllerManagerInSync"
	CPNConditionSchedulerInSync         = "SchedulerInSync"
)

// Per-component summary conditions (`kubectl get cpn` columns, dhctl readiness checks):
// True = <Component>PodReady is True and <Component>InSync is True.
const (
	CPNConditionEtcdReady              = "EtcdReady"
	CPNConditionAPIServerReady         = "APIServerReady"
	CPNConditionControllerManagerReady = "ControllerManagerReady"
	CPNConditionSchedulerReady         = "SchedulerReady"
)

const (
	CPNReasonReady    = "Ready"
	CPNReasonNotReady = "NotReady"

	// InSync reasons:
	// OperationInQueue  — an active operation exists, not approved yet;
	// OperationInFlight — an active operation is approved and executing;
	// OutOfSync         — no active operation, but applied checksums differ from the desired ones;
	// InSync            — no active operation and checksums match.
	CPNReasonOperationInQueue  = "OperationInQueue"
	CPNReasonOperationInFlight = "OperationInFlight"
	CPNReasonOutOfSync         = "OutOfSync"
	CPNReasonInSync            = "InSync"

	CPNReasonWaitingForComponents = "WaitingForComponents"
	CPNReasonHealthy              = "Healthy"
	CPNReasonExpiredSoon          = "ExpiredSoon"
	CPNReasonUnknown              = "Unknown"
)
