package hooks

const (
	linstorNamespace             = "d8-linstor"
	linstorServiceName           = "linstor"
	linstorServiceHost           = linstorServiceName + "." + linstorNamespace + ".svc"
	linstorHTTPSControllerSecret = "linstor-controller-https-cert"
	linstorHTTPSClientSecret     = "linstor-client-https-cert"
	linstorSSLControllerSecret   = "linstor-controller-ssl-cert"
	linstorSSLNodeSecret         = "linstor-node-ssl-cert"

	httpsControllerCertPath = "linstor.internal.httpsControllerCert"
	httpsClientCertPath     = "linstor.internal.httpsClientCert"
	sslControllerCertPath   = "linstor.internal.sslControllerCert"
	sslNodeCertPath         = "linstor.internal.sslNodeCert"

	spaasServiceName = "spaas"
	spaasServiceHost = spaasServiceName + "." + linstorNamespace + ".svc"
	spaasSecretName  = "spaas-certs"
	spaasCertPath    = "linstor.internal.spaasCert"
)
