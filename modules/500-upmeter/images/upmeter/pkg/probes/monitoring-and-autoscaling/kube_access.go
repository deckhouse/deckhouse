package monitoring_and_autoscaling

import (
	"k8s.io/client-go/kubernetes"

	"upmeter/pkg/checks"
)

// This is a workaround to inject dependencies that are available asynchronously
type KubeAccessor struct {
	probe *checks.Probe
}

func newKubeAccessor(pr *checks.Probe) *KubeAccessor {
	return &KubeAccessor{pr}
}

func (in *KubeAccessor) Kubernetes() kubernetes.Interface {
	return in.probe.KubernetesClient
}

func (in *KubeAccessor) ServiceAccountToken() string {
	return in.probe.ServiceAccountToken
}
