package control_plane

import (
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"upmeter/pkg/app"
	"upmeter/pkg/probe/types"
	"upmeter/pkg/probers/util"
)

/*
CHECK:
Cluster should be able to schedule a Pod onto a Node.

Create Pod, wait for Running status, delete Pod.

Period: 1 minute
Pod creation timeout: 5 seconds.
Scheduler reaction timeout: 20 seconds.
Pod deletion timeout: 5 seconds.
*/
func NewSchedulerProber() types.Prober {
	var schProbeRef = types.ProbeRef{
		Group: groupName,
		Probe: "scheduler",
	}
	const schProbePeriod = 60 // period: 1 min
	const schCreatePodTimeout = time.Second * 5
	const schSchedulerReactionTimeout = time.Second * 20
	const schDeletePodTimeout = time.Second * 5

	pr := &types.CommonProbe{
		ProbeRef: &schProbeRef,
		Period:   schProbePeriod,
	}

	pr.RunFn = func(start int64) {
		log := pr.LogEntry()

		podName := util.RandomIdentifier("upmeter-control-plane-scheduler")

		pod := &v1.Pod{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Pod",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: podName,
				Labels: map[string]string{
					"heritage":      "upmeter",
					"upmeter-agent": util.AgentUniqueId(),
					"upmeter-group": "control-plane",
					"upmeter-probe": "scheduler",
				},
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name:            "pause",
						Image:           "k8s.gcr.io/pause:3.1",
						ImagePullPolicy: v1.PullIfNotPresent,
						Command: []string{
							"/pause",
						},
					},
				},
				Tolerations: []v1.Toleration{
					{Operator: v1.TolerationOpExists},
				},
			},
		}

		var err error
		var errors = 0
		util.DoWithTimer(schCreatePodTimeout, func() {
			_, err := pr.KubernetesClient.CoreV1().Pods(app.Namespace).Create(pod)
			if err != nil {
				errors++
				log.Errorf("Create Pod/%s: %v", podName, err)
			}
		}, func() {
			log.Infof("Exceed timeout when create Pod/%s", podName)
			pr.ResultCh <- pr.Result(types.ProbeFailed)
		})

		// TODO watch pod is scheduled
		util.DoWithTimer(schSchedulerReactionTimeout, func() {
			schErrors := 0
			count := int(schSchedulerReactionTimeout.Seconds())
			lastPhase := v1.PodUnknown
			for i := 0; i < count; i++ {
				podObj, err := pr.KubernetesClient.CoreV1().Pods(app.Namespace).Get(pod.Name, metav1.GetOptions{})
				if err != nil {
					schErrors++
					log.Warnf("Check Pod/%s status: %v", podName, err)
					//pr.ResultCh <- pr.ResultFail(nsProbeRef)
				}
				lastPhase = podObj.Status.Phase
				if lastPhase == v1.PodRunning {
					return
				}
				time.Sleep(time.Second)
			}
			if schErrors > 0 {
				errors++
				log.Errorf("Pod/%s is not scheduled, phase: '%s'", podName, lastPhase)
			}
		}, func() {
			log.Infof("Exceed timeout waiting Pod/%s is scheduled", podName)
			pr.ResultCh <- pr.Result(types.ProbeFailed)
		})

		util.DoWithTimer(schDeletePodTimeout, func() {
			err = pr.KubernetesClient.CoreV1().Pods(app.Namespace).Delete(pod.Name, &metav1.DeleteOptions{})
			if err != nil {
				errors++
				log.Errorf("Delete Pod/%s: %v", podName, err)
			}
		}, func() {
			log.Infof("Exceed timeout when delete Pod/%s", podName)
			pr.ResultCh <- pr.Result(types.ProbeFailed)
		})

		// Final result
		pr.ResultCh <- pr.Result(errors == 0)
	}

	return pr
}
