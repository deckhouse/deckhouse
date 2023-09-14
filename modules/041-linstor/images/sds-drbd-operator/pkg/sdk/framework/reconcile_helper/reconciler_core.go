package reconcile_helper

import (
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ReconcilerOptions struct {
	Client   client.Client
	Cache    cache.Cache
	Recorder record.EventRecorder
	Scheme   *runtime.Scheme
	Log      logr.Logger
}
