package nodetemplate

const (
	controllerName = "node-template"
	allRequestName = "__all__"

	nodeGroupNameLabel                = "node.deckhouse.io/group"
	lastAppliedNodeTemplateAnnotation = "node-manager.deckhouse.io/last-applied-node-template"
	nodeUninitializedTaintKey         = "node.deckhouse.io/uninitialized"
	masterNodeRoleKey                 = "node-role.kubernetes.io/master"
	clusterAPIAnnotationKey           = "cluster.x-k8s.io/machine"
	heartbeatAnnotationKey            = "kubevirt.internal.virtualization.deckhouse.io/heartbeat"
	metalLBmemberLabelKey             = "l2-load-balancer.network.deckhouse.io/member"
	controlPlaneTaintKey              = "node-role.kubernetes.io/control-plane"
)
