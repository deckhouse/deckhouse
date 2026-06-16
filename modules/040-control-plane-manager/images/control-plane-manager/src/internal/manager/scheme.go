package manager

import (
	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	updateobserverv1 "control-plane-manager/internal/controllers/update-observer/pkg/v1"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(controlplanev1alpha1.AddToScheme(scheme))
	utilruntime.Must(updateobserverv1.AddToScheme(scheme))
}
