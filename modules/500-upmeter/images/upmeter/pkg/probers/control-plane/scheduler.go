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
	const deleteGarbageTimeout = 10 * time.Second

	pr := &types.CommonProbe{
		ProbeRef: &schProbeRef,
		Period:   schProbePeriod,
	}

	pr.RunFn = func(start int64) {
		log := pr.LogEntry()

		// Set Unknown result if API server is unavailable
		if !CheckApiAvailable(pr) {
			return
		}

		// Probe checks if scheduler is working, so Pod exits immediately and never restarts.
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
						Image:           "alpine:3.12",
						ImagePullPolicy: v1.PullIfNotPresent,
						Command: []string{
							"true",
						},
					},
				},
				RestartPolicy: v1.RestartPolicyNever,
				Tolerations: []v1.Toleration{
					{Operator: v1.TolerationOpExists},
				},
			},
		}

		if !GarbageCollect(pr, pod.Kind, pod.Labels) {
			return
		}

		var stop bool

		// Create test Pod.
		util.DoWithTimer(schCreatePodTimeout, func() {
			_, err := pr.KubernetesClient.CoreV1().Pods(app.Namespace).Create(pod)
			if err != nil {
				pr.ResultCh <- pr.Result(types.ProbeUnknown)
				log.Errorf("Create Pod/%s: %v", podName, err)
				stop = true
			}
		}, func() {
			log.Infof("Exceed timeout when create Pod/%s", podName)
			pr.ResultCh <- pr.Result(types.ProbeUnknown)
		})

		if stop {
			return
		}

		// If Pod is created, wait for scheduler decision and delete Pod.
		util.DoWithTimer(schSchedulerReactionTimeout, func() {
			var count = int(schSchedulerReactionTimeout.Seconds())
			var lastPhase = v1.PodUnknown
			var getErr error
			for i := 0; i < count; i++ {
				podObj, err := pr.KubernetesClient.CoreV1().Pods(app.Namespace).Get(pod.Name, metav1.GetOptions{})
				if err != nil {
					getErr = err
					//log.Warnf("Check Pod/%s status: %v", podName, err)
					continue
				}
				lastPhase = podObj.Status.Phase
				if lastPhase == v1.PodRunning || lastPhase == v1.PodSucceeded || podObj.Spec.Hostname != "" {
					pr.ResultCh <- pr.Result(types.ProbeSuccess)
					return
				}
				time.Sleep(time.Second)
			}
			if getErr != nil {
				log.Errorf("Pod/%s get error: %s", podName, getErr)
			}
			log.Errorf("Pod/%s is not scheduled, phase: '%s'", podName, lastPhase)
			pr.ResultCh <- pr.Result(types.ProbeFailed)
		}, func() {
			log.Infof("Exceed timeout waiting Pod/%s is scheduled", podName)
			pr.ResultCh <- pr.Result(types.ProbeUnknown)
		})

		// Delete does not change probe result. Next probe execution will change result
		// to Unknown if garbage is non-deletable.
		util.DoWithTimer(schDeletePodTimeout, func() {
			err := pr.KubernetesClient.CoreV1().Pods(app.Namespace).Delete(pod.Name, &metav1.DeleteOptions{})
			if err != nil {
				log.Errorf("Delete Pod/%s: %v", podName, err)
				return
			}

			if !WaitForObjectDeletion(pr, schDeletePodTimeout, pod.Kind, pod.Name) {
				return
			}
		}, func() {
			log.Infof("Exceed timeout deleting Pod/%s", podName)
		})

	}

	return pr
}
