package etcd

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/filter"
)

const (
	DefaultMaxSize        int64 = 2 * 1024 * 1024 * 1024 // 2GB
	endpointsSnapshotName       = "etcd_endpoints"
)

type Instance struct {
	Endpoint  string
	MaxDbSize int64
	PodName   string
	Node      string
}

var (
	maxDbSizeRegExp = regexp.MustCompile(`(^|\s+)--quota-backend-bytes=(\d+)$`)

	MaintenanceConfig = go_hook.KubernetesConfig{
		Name:       endpointsSnapshotName,
		ApiVersion: "v1",
		Kind:       "Pod",
		NamespaceSelector: &types.NamespaceSelector{
			NameSelector: &types.NameSelector{
				MatchNames: []string{"kube-system"},
			},
		},
		LabelSelector: &v1.LabelSelector{
			MatchLabels: map[string]string{
				"component": "etcd",
				"tier":      "control-plane",
			},
		},
		FieldSelector: &types.FieldSelector{
			MatchExpressions: []types.FieldSelectorRequirement{
				{
					Field:    "status.phase",
					Operator: "Equals",
					Value:    "Running",
				},
			},
		},
		FilterFunc: maintenanceEtcdFilter,
	}
)

func maintenanceEtcdFilter(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var pod corev1.Pod

	err := sdk.FromUnstructured(unstructured, &pod)
	if err != nil {
		return nil, err
	}

	var ip string
	if pod.Spec.HostNetwork {
		ip = pod.Status.HostIP
	} else {
		ip = pod.Status.PodIP
	}

	curMaxDbSize := DefaultMaxSize
	maxBytesStr := filter.GetArgPodWithRegexp(&pod, maxDbSizeRegExp, 1, "")
	if maxBytesStr != "" {
		curMaxDbSize, err = strconv.ParseInt(maxBytesStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("cannot get quota-backend-bytes from etcd argument, got %s: %v", maxBytesStr, err)
		}
	}

	return &Instance{
		Endpoint:  Endpoint(ip),
		MaxDbSize: curMaxDbSize,
		PodName:   pod.GetName(),
		Node:      pod.Spec.NodeName,
	}, nil
}

func InstancesFromSnapshot(input *go_hook.HookInput) []*Instance {
	snap := input.Snapshots[endpointsSnapshotName]
	res := make([]*Instance, 0, len(snap))

	for _, raw := range snap {
		res = append(res, raw.(*Instance))
	}

	return res
}

func Endpoint(ip string) string {
	return fmt.Sprintf("https://%s:2379", ip)
}
