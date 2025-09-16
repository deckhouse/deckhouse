/*
Copyright 2021 Flant JSC

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

package hooks

import (
	"context"
	"math"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        moduleQueue,
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
}, handleArguments)

type arguments struct {
	NodeMonitorGracePeriodSeconds       int64   `json:"nodeMonitorGracePeriod,omitempty"`
	NodeMonitorPeriod                   float64 `json:"nodeMonitorPeriod,omitempty"`
	PodEvictionTimeout                  int64   `json:"podEvictionTimeout,omitempty"`
	DefaultUnreachableTolerationSeconds int64   `json:"defaultUnreachableTolerationSeconds,omitempty"`
}

func handleArguments(_ context.Context, input *go_hook.HookInput) error {
	var arg arguments
	nodeMonitorGrace, ok := input.Values.GetOk("controlPlaneManager.nodeMonitorGracePeriodSeconds")
	if ok {
		nodeMonitorGraceSeconds := nodeMonitorGrace.Int()
		arg.NodeMonitorGracePeriodSeconds = nodeMonitorGraceSeconds
		arg.NodeMonitorPeriod = math.Round(float64(nodeMonitorGraceSeconds) / 8)
	}

	failedNodePodEvictionTimeout, ok := input.Values.GetOk("controlPlaneManager.failedNodePodEvictionTimeoutSeconds")
	if ok {
		podEvictionTimeout := failedNodePodEvictionTimeout.Int()
		arg.PodEvictionTimeout = podEvictionTimeout
		arg.DefaultUnreachableTolerationSeconds = podEvictionTimeout
	}

	if (arg == arguments{}) {
		input.Values.Remove("controlPlaneManager.internal.arguments")
	} else {
		input.Values.Set("controlPlaneManager.internal.arguments", arg)
	}
	return nil
}
