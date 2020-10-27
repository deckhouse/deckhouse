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
Cluster should be able to delete Deploymeny and Pod

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

	pr := &types.CommonProbe{
		ProbeRef: &mgrProbeRef,
		Period:   mgrProbePeriod,
	}

	pr.RunFn = func(start int64) {
		log := pr.LogEntry()
		deployName := util.RandomIdentifier("upmeter-control-plane-manager")

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
				Replicas: func() *int32 {
					var a int32 = 1
					return &a
				}(),
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

		var err error
		var errors = 0
		util.DoWithTimer(mgrCreateDeploymentTimeout, func() {
			_, err := pr.KubernetesClient.AppsV1().Deployments(app.Namespace).Create(deployment)
			if err != nil {
				errors++
				log.Errorf("Create Deployment/%s: %v", deployName, err)
			}
		}, func() {
			log.Infof("Exceed timeout when create Deployment/%s", deployName)
			pr.ResultCh <- pr.Result(types.ProbeFailed)
		})

		var pendingPodName = ""

		util.DoWithTimer(mgrPodPendingTimeout, func() {
			waitErrors := 0
			count := int(mgrPodPendingTimeout.Seconds())
			lastPhase := v1.PodUnknown
			podLabels := labels.FormatLabels(map[string]string{
				"app": deployName,
			})
			listOptions := metav1.ListOptions{
				LabelSelector: podLabels,
			}
			for i := 0; i < count; i++ {
				podList, err := pr.KubernetesClient.CoreV1().Pods(app.Namespace).List(listOptions)
				if err != nil {
					waitErrors++
					log.Errorf("Cannot list Pods for Deployment/%s: %v", deployName, err)
				}

				// Check is OK if at least one Pod is in a "Pending" state.
				for _, pod := range podList.Items {
					if pod.Status.Phase == v1.PodPending {
						pendingPodName = pod.Name
						return
					}
				}
				time.Sleep(time.Second)
			}
			if waitErrors > 0 {
				errors++
				log.Errorf("Deployment/%s has no Pending or Running pod, phase: '%s'", deployName, lastPhase)
			}
		}, func() {
			log.Infof("Exceed timeout while waiting pending Pod")
			pr.ResultCh <- pr.Result(types.ProbeFailed)
		})

		util.DoWithTimer(mgrDeleteDeploymentTimeout, func() {
			err = pr.KubernetesClient.AppsV1().Deployments(app.Namespace).Delete(deployment.Name, &metav1.DeleteOptions{})
			if err != nil {
				errors++
				log.Errorf("Delete Deployment/%s: %v", deployName, err)
			}
		}, func() {
			log.Infof("Exceed timeout when delete Deployment/%s", deployName)
			pr.ResultCh <- pr.Result(types.ProbeFailed)
		})

		util.DoWithTimer(mgrPodDisappearTimeout, func() {
			waitErrors := 0
			count := int(mgrPodDisappearTimeout.Seconds())
			podLabels := labels.FormatLabels(map[string]string{
				"app": deployName,
			})
			listOptions := metav1.ListOptions{
				LabelSelector: podLabels,
				FieldSelector: "metadata.name=" + pendingPodName,
			}
			for i := 0; i < count; i++ {
				podList, err := pr.KubernetesClient.CoreV1().Pods(app.Namespace).List(listOptions)
				if err == nil && len(podList.Items) == 0 {
					return
				}
				waitErrors++
				time.Sleep(time.Second)
			}
			if waitErrors > 0 {
				errors++
				log.Errorf("Deleted Deployment/%s still has Pod/%s", deployName, pendingPodName)
			}
		}, func() {
			log.Infof("Exceed timeout while wait for Pod deletion")
			pr.ResultCh <- pr.Result(types.ProbeFailed)
		})

		// Final result
		pr.ResultCh <- pr.Result(errors == 0)
	}

	return pr
}
