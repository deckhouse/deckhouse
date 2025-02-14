package config

type AuthMode int

const (
	AuthDev AuthMode = iota
	AuthCert
)

type Config struct {
	AuthMode                 AuthMode
	SocketPath               string
	APIHosts                 []string
	CertPath                 string
	KeyPath                  string
	CACertPath               string
	HAProxyConfigurationFile string
	HAProxyHAProxyBin        string
	HAProxyTransactionsDir   string
	ConfigPath               string
}

type BackendConfig struct {
	Backends []Backend `yaml:"backends"`
}

type Backend struct {
	Name string `yaml:"name"`
	K8S  struct {
		Namespace    string `yaml:"namespace"`
		EndpointName string `yaml:"endpointName"`
		PortName     string `yaml:"portName"`
	} `yaml:"k8s"`
	HAProxy struct {
		DefautlServer string `yaml:"defautlServer"`
	} `yaml:"haproxy"`
}
