package upmeter

import (
	"github.com/flant/shell-operator/pkg/kube"
	"github.com/flant/shell-operator/pkg/metric_storage"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func Test_kube_connection(t *testing.T) {
	var MetricStorage *metric_storage.MetricStorage
	var KubernetesClient kube.KubernetesClient
	var err error

	// Metric storage
	MetricStorage = metric_storage.NewMetricStorage()

	// Kubernetes client
	KubernetesClient = kube.NewKubernetesClient()
	//KubernetesClient.WithContextName(shapp.KubeContext)
	//KubernetesClient.WithConfigPath(shapp.KubeConfig)
	//KubernetesClient.WithRateLimiterSettings(shapp.KubeClientQps, shapp.KubeClientBurst)
	KubernetesClient.WithMetricStorage(MetricStorage)

	err = KubernetesClient.Init()
	if err != nil {
		t.Fatalf("init kubernetes client: %v", err)
		return
	}

	list, err := KubernetesClient.CoreV1().Namespaces().List(v1.ListOptions{})

	if err != nil {
		t.Fatalf("list ns: %v type:%T", err, err)
		return
	}

	t.Logf("Got %d namespaces", len(list.Items))
}
