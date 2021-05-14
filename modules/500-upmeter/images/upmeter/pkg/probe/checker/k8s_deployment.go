package checker

import (
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"d8.io/upmeter/pkg/check"
	k8s "d8.io/upmeter/pkg/kubernetes"
	"d8.io/upmeter/pkg/probe/util"
)

// DeploymentLifecycle is a checker constructor and configurator
type DeploymentLifecycle struct {
	Access                    k8s.Access
	Namespace                 string
	DeploymentCreationTimeout time.Duration
	DeploymentDeletionTimeout time.Duration
	PodAppearTimeout          time.Duration
	PodDisappearTimeout       time.Duration
	GarbageCollectionTimeout  time.Duration
}

func (c DeploymentLifecycle) Checker() check.Checker {
	return &deploymentLifecycleChecker{
		access:                    c.Access,
		namespace:                 c.Namespace,
		deploymentCreationTimeout: c.DeploymentCreationTimeout,
		deploymentDeletionTimeout: c.DeploymentDeletionTimeout,
		podAppearTimeout:          c.PodAppearTimeout,
		podDisappearTimeout:       c.PodDisappearTimeout,
		garbageCollectionTimeout:  c.GarbageCollectionTimeout,
	}
}

type deploymentLifecycleChecker struct {
	access                    k8s.Access
	namespace                 string
	deploymentCreationTimeout time.Duration
	deploymentDeletionTimeout time.Duration
	podAppearTimeout          time.Duration
	podDisappearTimeout       time.Duration
	garbageCollectionTimeout  time.Duration

	// inner state
	checker check.Checker
}

func (c *deploymentLifecycleChecker) BusyWith() string {
	return c.checker.BusyWith()
}

func (c *deploymentLifecycleChecker) Check() check.Error {
	deployment := createDeploymentObject()
	c.checker = c.new(deployment)
	return c.checker.Check()
}

/*
 1. check control plane availability
 2. collect the garbage of the deployment/rs/pods from previous runs
 3. create the deployment in api        (deploymentCreationTimeout)
 4. wait for pending pod to appear      (podAppearTimeout, retry each 1 sec)
 5. delete the deployment in api        (deploymentDeletionTimeout)
 6. wait for the pod to disappear       (podDisappearTimeout, retry each 1 sec)
*/
func (c *deploymentLifecycleChecker) new(deployment *appsv1.Deployment) check.Checker {
	deploymentCreated := withTimeout(
		&deploymentCreationChecker{
			access:     c.access,
			namespace:  c.namespace,
			deployment: deployment,
		},
		c.deploymentCreationTimeout,
	)

	deploymentDeleted := withTimeout(
		&deploymentDeletionChecker{
			access:     c.access,
			namespace:  c.namespace,
			deployment: deployment,
		},
		c.deploymentDeletionTimeout,
	)

	podListOptions := listOptsByLabels(map[string]string{"app": deployment.Name})

	podAppeared := withRetryEachSeconds(
		&pendingPodChecker{
			access:    c.access,
			namespace: c.namespace,
			listOpts:  podListOptions,
		},
		c.podAppearTimeout)

	podDisappeared := withRetryEachSeconds(
		&objectIsNotListedChecker{
			access:    c.access,
			namespace: c.namespace,
			kind:      "Pod",
			listOpts:  podListOptions,
		},
		c.podDisappearTimeout)

	collectGarbage := newGarbageCollectorCheckerByLabels(c.access, deployment.Kind, c.namespace, deployment.Labels, c.garbageCollectionTimeout)

	checker := sequence(
		&controlPlaneChecker{c.access},
		collectGarbage,
		deploymentCreated,
		podAppeared,
		deploymentDeleted,
		podDisappeared,
	)

	timeout := c.deploymentCreationTimeout + c.deploymentDeletionTimeout + c.podAppearTimeout + c.podDisappearTimeout
	return withTimeout(checker, timeout)
}

type deploymentCreationChecker struct {
	access     k8s.Access
	namespace  string
	deployment *appsv1.Deployment
}

func (c *deploymentCreationChecker) BusyWith() string {
	return fmt.Sprintf("creating deployment %s/%s", c.namespace, c.deployment.Name)
}

func (c *deploymentCreationChecker) Check() check.Error {
	client := c.access.Kubernetes()

	_, err := client.AppsV1().Deployments(c.namespace).Create(c.deployment)
	if err != nil {
		return check.ErrUnknown("creating deployment/%s: %v", c.deployment.Name, err)
	}

	return nil
}

type deploymentDeletionChecker struct {
	access     k8s.Access
	namespace  string
	deployment *appsv1.Deployment
}

func (c *deploymentDeletionChecker) BusyWith() string {
	return fmt.Sprintf("deleting deployment %s/%s", c.namespace, c.deployment.Name)
}

func (c *deploymentDeletionChecker) Check() check.Error {
	client := c.access.Kubernetes()

	err := client.AppsV1().Deployments(c.namespace).Delete(c.deployment.Name, &metav1.DeleteOptions{})
	if err != nil {
		return check.ErrFail("failed to delete deployment/%s: %v", c.deployment.Name, err)
	}

	return nil
}

func createDeploymentObject() *appsv1.Deployment {
	name := util.RandomIdentifier("upmeter-controller-manager")
	replicas := int32(1)
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"heritage":      "upmeter",
				"app":           "upmeter-controller-manager",
				"upmeter-agent": util.AgentUniqueId(),
				"upmeter-group": "control-plane",
				"upmeter-probe": "controller-manager",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"upmeter-agent": util.AgentUniqueId(),
					"app":           name,
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"upmeter-agent": util.AgentUniqueId(),
						"app":           name,
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
}
