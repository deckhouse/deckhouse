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

		var err error
		var errors = 0
		util.DoWithTimer(nsCreateTimeout, func() {
			_, err := pr.KubernetesClient.CoreV1().Namespaces().Create(ns)
			if err != nil {
				errors++
				log.Errorf("Create ns/%s: %v", nsName, err)
			}
		}, func() {
			log.Infof("Exceed timeout when create ns/%s", nsName)
			pr.ResultCh <- pr.Result(types.ProbeFailed)
		})

		util.DoWithTimer(nsDeleteTimeout, func() {
			err = pr.KubernetesClient.CoreV1().Namespaces().Delete(ns.Name, &metav1.DeleteOptions{})
			if err != nil {
				errors++
				log.Errorf("Delete ns/%s: %v", nsName, err)
			}
		}, func() {
			log.Infof("Exceed timeout when delete ns/%s", nsName)
			pr.ResultCh <- pr.Result(types.ProbeFailed)
		})

		// Final result
		pr.ResultCh <- pr.Result(errors == 0)
	}

	return pr
}
