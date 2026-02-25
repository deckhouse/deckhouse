package kubeconfig

type File string

const (
	SuperAdmin        File = "super-admin.conf"
	Admin             File = "admin.conf"
	Scheduler         File = "scheduler.conf"
	ControllerManager File = "controller-manager.conf"
	Kubelet           File = "kubelet.conf"
)