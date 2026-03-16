package rcname

type ReconcilerName string

func (cn ReconcilerName) String() string { return string(cn) }

const (
	NodeGroup              ReconcilerName = "nodegroup"
	NodeGroupStatus        ReconcilerName = "nodegroup-status"
	NodeGroupInstanceClass ReconcilerName = "nodegroup-instanceclass"
	NodeGroupMaster        ReconcilerName = "nodegroup-master"
	NodeUpdate             ReconcilerName = "node-update"
	NodeTemplate           ReconcilerName = "node-template"
	NodeGPU                ReconcilerName = "node-gpu"
	NodeProviderID         ReconcilerName = "node-provider-id"
	NodeBashibleCleanup    ReconcilerName = "node-bashible-cleanup"
	NodeFencing            ReconcilerName = "node-fencing"
	Instance               ReconcilerName = "instance"
	MachineDeployment      ReconcilerName = "machine-deployment"
	CSITaint               ReconcilerName = "csi-taint"
	CSRApprover            ReconcilerName = "csr-approver"
	CRDWebhook             ReconcilerName = "crd-webhook"
	BashiblePod            ReconcilerName = "bashible-pod"
	BashibleLock           ReconcilerName = "bashible-lock"
	ControlPlane           ReconcilerName = "control-plane"
	ChaosMonkey            ReconcilerName = "chaos-monkey"
	NodeUser               ReconcilerName = "node-user"
	YCPreemptible          ReconcilerName = "yc-preemptible"
	MetricsCAPS            ReconcilerName = "metrics-caps"
	MetricsNGConfig        ReconcilerName = "metrics-ng-config"
	MetricsContainerd      ReconcilerName = "metrics-containerd"
	MetricsOSVersion       ReconcilerName = "metrics-os-version"
	MetricsCloudConditions ReconcilerName = "metrics-cloud-conditions"
)
