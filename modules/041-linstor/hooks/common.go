/*
Copyright 2022 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
