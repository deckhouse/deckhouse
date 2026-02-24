package corev1

// PodConditionType defines the condition of pod
type PodConditionType string

const (
	// ContainersReady indicates whether all containers in the pod are ready.
	ContainersReady PodConditionType = "ContainersReady"
	// PodInitialized means that all init containers in the pod have started successfully.
	PodInitialized PodConditionType = "Initialized"
	// PodReady means the pod is able to service requests and should be added to the
	// load balancing pools of all matching services.
	PodReady PodConditionType = "Ready"
	// PodScheduled represents status of the scheduling process for this pod.
	PodScheduled PodConditionType = "PodScheduled"
	// DisruptionTarget indicates the pod is about to be terminated due to a
	// disruption (such as preemption, eviction API or garbage-collection).
	DisruptionTarget PodConditionType = "DisruptionTarget"
	// PodReadyToStartContainers pod sandbox is successfully configured and
	// the pod is ready to launch containers.
	PodReadyToStartContainers PodConditionType = "PodReadyToStartContainers"
	// PodResizePending indicates that the pod has been resized, but kubelet has not
	// yet allocated the resources. If both PodResizePending and PodResizeInProgress
	// are set, it means that a new resize was requested in the middle of a previous
	// pod resize that is still in progress.
	PodResizePending PodConditionType = "PodResizePending"
	// PodResizeInProgress indicates that a resize is in progress, and is present whenever
	// the Kubelet has allocated resources for the resize, but has not yet actuated all of
	// the required changes.
	// If both PodResizePending and PodResizeInProgress are set, it means that a new resize was
	// requested in the middle of a previous pod resize that is still in progress.
	PodResizeInProgress PodConditionType = "PodResizeInProgress"
)

// ConditionStatus is the status of the condition.
type ConditionStatus string

// These are valid condition statuses. "ConditionTrue" means a resource is in the condition.
// "ConditionFalse" means a resource is not in the condition. "ConditionUnknown" means kubernetes
// can't decide if a resource is in the condition or not. In the future, we could add other
// intermediate conditions, e.g. ConditionDegraded.
const (
	ConditionTrue    ConditionStatus = "True"
	ConditionFalse   ConditionStatus = "False"
	ConditionUnknown ConditionStatus = "Unknown"
)
