package control_plane

import (
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"upmeter/pkg/app"
	"upmeter/pkg/probe/types"
	"upmeter/pkg/probers/util"
)

/*
CHECK:
Cluster should be able to create Deployment and Pod.
Cluster should be able to delete Deployment and Pod.

Create Deployment with Pod spec that have never-matched nodeSelector and nonexistent image.
Wait for Pod status is not Unknown. Delete Deployment, wait that Pod is disappeared.

Period: 1 minute
Create Deployment timeout: 5 seconds
Pending Pod timeout: 10 seconds
Delete Deployment timeout: 5 seconds
Pod Disappear timeout: 10 seconds
*/
func NewControlPlaneManagerProber() types.Prober {
	var mgrProbeRef = types.ProbeRef{
		Group: groupName,
		Probe: "control-plane-manager",
	}
	const mgrProbePeriod = 60 // period: 1 min
	const mgrCreateDeploymentTimeout = time.Second * 5
	const mgrPodPendingTimeout = time.Second * 10
	const mgrDeleteDeploymentTimeout = time.Second * 5
	const mgrPodDisappearTimeout = time.Second * 10
	const deleteGarbageTimeout = 10 * time.Second

	pr := &types.CommonProbe{
		ProbeRef: &mgrProbeRef,
		Period:   mgrProbePeriod,
	}

	pr.RunFn = func(start int64) {
		log := pr.LogEntry()

		// Set Unknown result if API server is unavailable
		if !CheckApiAvailable(pr) {
			return
		}

		deployName := util.RandomIdentifier("upmeter-control-plane-manager")
		deployReplicas := int32(1)
		deployment := &appsv1.Deployment{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Deployment",
				APIVersion: "apps/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: deployName,
				Labels: map[string]string{
					"heritage":      "upmeter",
					"app":           "upmeter-control-plane-manager",
					"upmeter-agent": util.AgentUniqueId(),
					"upmeter-group": "control-plane",
					"upmeter-probe": "control-plane-manager",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: &deployReplicas,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"upmeter-agent": util.AgentUniqueId(),
						"app":           deployName,
					},
				},
				Template: v1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"upmeter-agent": util.AgentUniqueId(),
							"app":           deployName,
						},
					},
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{
								Name:  "pause",
								Image: "k8s.gcr.io/supa-dupa-pause:3.1",
								Command: []string{
									"/pause",
								},
							},
						},
						NodeSelector: map[string]string{
							"gpu-flavour": "RTX-ON",
							"cpu-flavour": "QuantumContinuum",
						},
						Tolerations: []v1.Toleration{
							{Operator: v1.TolerationOpExists},
						},
					},
				},
				Strategy: appsv1.DeploymentStrategy{
					Type: appsv1.RecreateDeploymentStrategyType,
				},
			},
		}

		if !GarbageCollect(pr, deployment.Kind, deployment.Labels) {
			return
		}

		var stop bool

		util.DoWithTimer(mgrCreateDeploymentTimeout, func() {
			_, err := pr.KubernetesClient.AppsV1().Deployments(app.Namespace).Create(deployment)
			if err != nil {
				pr.ResultCh <- pr.Result(types.ProbeUnknown)
				log.Errorf("Create Deployment/%s: %v", deployName, err)
				stop = true
			}
		}, func() {
			log.Infof("Exceed timeout when create Deployment/%s", deployName)
			pr.ResultCh <- pr.Result(types.ProbeUnknown)
		})

		var pendingPodName = ""

		util.DoWithTimer(mgrPodPendingTimeout, func() {
			var listErr error
			var count = int(mgrPodPendingTimeout.Seconds())
			var lastPhase = v1.PodUnknown
			podLabels := labels.FormatLabels(map[string]string{
				"app": deployName,
			})
			listOptions := metav1.ListOptions{
				LabelSelector: podLabels,
			}
			for i := 0; i < count; i++ {
				podList, err := pr.KubernetesClient.CoreV1().Pods(app.Namespace).List(listOptions)
				if err != nil {
					listErr = err
					continue
				}
				// Stop waiting when at least one Pod is in a "Pending" state.
				for _, pod := range podList.Items {
					if pod.Status.Phase == v1.PodPending {
						pendingPodName = pod.Name
						return
					}
				}
				time.Sleep(time.Second)
			}
			if listErr != nil {
				log.Errorf("Deployment/%s list pods: %v", deployName, listErr)
			}
			// No Pod in Pending or Running state, probe is failed.
			log.Errorf("Deployment/%s has no Pending or Running pod, phase: '%s'", deployName, lastPhase)
			pr.ResultCh <- pr.Result(types.ProbeFailed)
		}, func() {
			log.Infof("Exceed timeout while waiting pending Pod")
			pr.ResultCh <- pr.Result(types.ProbeUnknown)
		})

		util.DoWithTimer(mgrDeleteDeploymentTimeout, func() {
			err := pr.KubernetesClient.AppsV1().Deployments(app.Namespace).Delete(deployment.Name, &metav1.DeleteOptions{})
			if err != nil {
				pr.ResultCh <- pr.Result(types.ProbeFailed)
				log.Errorf("Delete Deployment/%s: %v", deployName, err)
				stop = true
			}
		}, func() {
			log.Infof("Exceed timeout when delete Deployment/%s", deployName)
			pr.ResultCh <- pr.Result(types.ProbeUnknown)
		})

		if stop {
			return
		}

		util.DoWithTimer(mgrPodDisappearTimeout, func() {
			if WaitForObjectDeletion(pr, mgrPodDisappearTimeout, "Pod", pendingPodName) {
				pr.ResultCh <- pr.Result(types.ProbeSuccess)
			} else {
				log.Errorf("Deleted Deployment/%s still has Pod/%s", deployName, pendingPodName)
				pr.ResultCh <- pr.Result(types.ProbeFailed)
			}
		}, func() {
			log.Infof("Exceed timeout while wait for Pod deletion")
			pr.ResultCh <- pr.Result(types.ProbeUnknown)
		})

	}

	return pr
}
