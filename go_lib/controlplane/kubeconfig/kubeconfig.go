package kubeconfig

type Component string

const (
	ComponentAdmin             Component = "admin"
	ComponentScheduler         Component = "kube-scheduler"
	ComponentControllerManager Component = "kube-controller-manager"
	ComponentKubelet           Component = "kubelet"
)

type CertProvider interface {
	GetCA() ([]byte, error)
	GetClientCert(component Component) ([]byte, error)
	GetClientKey(component Component) ([]byte, error)
}

type options struct {
	ClusterName string
	APIServer   string
	CAPath      string
	CertDir     string
	Namespace   string

	CertProvider CertProvider
}

type controlPlaneOptions struct {
	options
	OutputDir string
}

type kubeletOptions struct {
	options

	KubeletKubeconfigPath string
	KubeletClientCertPath string
	KubeletClientKeyPath  string
}

func CreateControlPlaneKubeConfigFiles(opts controlPlaneOptions) error {
	panic("not implemented")
}

func CreateKubeletKubeConfigFile(opts kubeletOptions) error {
	panic("not implemented")
}

func CreateKubeConfigFile(kubeConfigFileName string, outputDir string, opts options) error {
	panic("not implemented")
}
