package hooks

import (
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

func handleArguments(input *go_hook.HookInput) error {
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
