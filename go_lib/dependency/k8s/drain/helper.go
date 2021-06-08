package drain

import (
	"time"

	"k8s.io/client-go/kubernetes"
)

func NewDrainer(kubeClient kubernetes.Interface) *Helper {
	drainer := &Helper{
		Client:              kubeClient,
		Force:               true,
		IgnoreAllDaemonSets: true,
		DeleteEmptyDirData:  true, // same as DeleteLocalData
		GracePeriodSeconds:  -1,
		// If a pod is not evicted in 20 seconds, retry the eviction next time the
		// machine gets reconciled again (to allow other machines to be reconciled).
		Timeout: 30 * time.Second,
		// Out:    writer{klog.Info},
		// ErrOut: writer{klog.Error},
	}

	return drainer
}
