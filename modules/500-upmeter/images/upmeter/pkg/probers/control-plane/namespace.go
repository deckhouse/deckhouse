package control_plane

import (
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"upmeter/pkg/probe/types"
	"upmeter/pkg/probers/util"
)

/*
CHECK:
Cluster should be able to create and delete a Namespace.

Period: 1 minute
Create Namespace timeout: 5 seconds.
Delete Namespace timeout: 60 seconds.
*/

/*
Probe do some garbage collection,
delete namespaces left from previous invocations.

Probe do nothing if namespace is stuck in Terminating
phase to prevent API server overload.
*/

func NewNamespaceProber() types.Prober {
	var nsProbeRef = types.ProbeRef{
		Group: groupName,
		Probe: "namespace",
	}
	const nsProbePeriod = 60 // period: 1 min
	const nsCreateTimeout = time.Second * 5
	const nsDeleteTimeout = time.Second * 60

	pr := &types.CommonProbe{
		ProbeRef: &nsProbeRef,
		Period:   nsProbePeriod,
	}

	pr.RunFn = func(start int64) {
		log := pr.LogEntry()

		// Set Unknown result if API server is unavailable
		if !CheckApiAvailable(pr) {
			return
		}

		nsName := util.RandomIdentifier("upmeter-control-plane-namespace")

		ns := &v1.Namespace{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Namespace",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: nsName,
				Labels: map[string]string{
					"heritage":      "upmeter",
					"upmeter-agent": util.AgentUniqueId(),
					"upmeter-group": "control-plane",
					"upmeter-probe": "namespace",
				},
			},
		}
		// This finalizer can help during testing on local cluster.
		// https://github.com/kubernetes/kubernetes/issues/60807
		//ns.Spec = v1.NamespaceSpec{Finalizers: []v1.FinalizerName{"foregroundDeletion"}}

		if !GarbageCollect(pr, ns.Kind, ns.Labels) {
			return
		}

		// Create new Namespace, delete it and wait till it gone.

		var stop bool

		util.DoWithTimer(nsCreateTimeout, func() {
			_, err := pr.KubernetesClient.CoreV1().Namespaces().Create(ns)
			if err != nil {
				pr.ResultCh <- pr.Result(types.ProbeUnknown)
				log.Errorf("Create ns/%s: %v", nsName, err)
				stop = true
			}
		}, func() {
			log.Infof("Exceed timeout when create ns/%s", nsName)
			pr.ResultCh <- pr.Result(types.ProbeUnknown)
		})

		if stop {
			return
		}

		util.DoWithTimer(nsDeleteTimeout, func() {
			err := pr.KubernetesClient.CoreV1().Namespaces().Delete(ns.Name, &metav1.DeleteOptions{})
			if err != nil {
				log.Errorf("Delete ns/%s: %v", nsName, err)
				pr.ResultCh <- pr.Result(types.ProbeFailed)
				return
			}

			if !WaitForObjectDeletion(pr, nsDeleteTimeout, ns.Kind, ns.Name) {
				pr.ResultCh <- pr.Result(types.ProbeFailed)
				return
			}

			pr.ResultCh <- pr.Result(types.ProbeSuccess)

		}, func() {
			log.Infof("Exceed timeout when delete ns/%s", nsName)
			pr.ResultCh <- pr.Result(types.ProbeUnknown)
		})

	}

	return pr
}
