---
title: "Monitoring the control plane: configuration"
---

The `monitoring-kubernetes-control-plane` module is configured automatically and usually does not require manual configuring. However, manual configuring may be necessary in some non-standard cases.

## Parameters

* `kubeApiserver` — parameters for collecting kube-apiserver metrics;
  * `metricsPath` — path to metrics (`/metrics` by default);
  * `accessType` — specifies the way Prometheus accesses metrics;
    * Possible values:
      * `DefaultService` — the most common option suitable for 99% of clusters; kube-apiserver is available via the `kubernetes` service in the `default` namespace;
      * `Pod` — kube-apiserver runs in a Pod, and the metrics port is accessible in the cluster via Kubernetes tools. (See the additional parameters in the `pod` section below);
      * `ThroughNode` — kube-apiserver runs on one or several nodes and is accessible on the local port. In this case, a proxy runs on nodes to connect to Prometheus. (See the additional parameters in the `throughNode` section below);
    * Set to `DefaultService` by default;
  * `pod` — additional parameters for the `Pod` accessType;
    * `port` — the port at which metrics are exposed;
      * By default:
        * It is calculated automatically using arguments derived from the kube-apiserver Pod;
        * If the calculation fails, the port is set to `6443`;
    * `podSelector` — a parameter to select service Pods (mandatory);
      * Format — a label dictionary;
    * `podNamespace` — namespace where the component's Pods are running (mandatory);
    * `authenticationMethod` —  the authentication method (mandatory);
      * Possible values:
        * `Certificate` — specify a custom certificate. (See the `certificateSecret` parameter below);
        * `PrometheusCertificate` — use the standard certificate that comes with the [prometheus](../../modules/300-prometheus/) module and grant it the appropriate RBAC-based rights to access metrics;
    * `certificateSecret` — the name of the secret in the `d8-system` namespace that stores the custom certificate. Mandatory if `authenticationMethod` == `Certificate`. The secret must contain two keys:
      * `client.crt` — a certificate;
      * `client.key` — a key;
  * `throughNode` — additional parameters for the `ThroughNode` accessType;
    * `nodeSelector` — selects nodes on which kube-apiserver is running (mandatory);
      * Format — a label dictionary;
    * `localPort` — the local kube-apiserver port (mandatory);
    * `proxyListenPort` — the port reserved for the proxy server;
      * The default port is `10361`.
    * `authenticationMethod` —  the authentication method (mandatory);
      * `Certificate` — specify a custom certificate. (See the `certificateSecret` parameter below);
      * `ProxyServiceAccount` — configure the proxy's ServiceAccount (using RBAC) to collect component metrics;
    * `certificateSecret` — the name of the secret in the `d8-system` namespace that stores the custom certificate. Mandatory if `authenticationMethod` == `Certificate`. The secret must contain two keys:
      * `client.crt` — a certificate;
      * `client.key` — a key;

* `kubeControllerManager` — parameters for collecting kube-controller-manager metrics;
  * `metricsPath` — path to metrics (`/metrics` by default);
  * `accessType` — specifies the way Prometheus gets access to metrics;
    * Possible values:
      * `Pod` — kube-controller-manager runs in a pod, and the metrics port is accessible in the cluster via Kubernetes tools. (See the additional parameters in the `pod` section below);
      * `ThroughNode` — kube-controller-manager runs on one or several nodes and is accessible on the local port. In this case, a proxy runs on nodes to connect to Prometheus. (See the additional parameters in the `throughNode` section below);
    * The default value is `ThroughNode`;
  * `pod` — additional parameters for the `Pod` accessType;
    * `port` — the port at which metrics are exposed (mandatory);
    * `scheme` — the HTTP scheme the metrics http-port uses;
    * `podSelector` — a parameter for selecting service Pods (mandatory);
      * Format — a label dictionary;
    * `podNamespace` — the namespace where kube-controller-manager Pods are running (mandatory);
    * `authenticationMethod` —  the authentication method (mandatory);
      * Possible values:
        * `None` — do not authenticate;
        * `Certificate` — specify a custom certificate. (See the `certificateSecret` parameter below);
        * `PrometheusCertificate` — use the standard certificate that comes with the [prometheus](../../modules/300-prometheus/) module and grant it the appropriate RBAC-based rights to access metrics;
    * `certificateSecret` — the name of the secret in the `d8-system` namespace that stores the custom certificate. Mandatory if `authenticationMethod` == `Certificate`. The secret must contain two keys:
      * `client.crt` — a certificate;
      * `client.key` — a key;
  * `throughNode` — additional parameters for the `ThroughNode` accessType;
    * `nodeSelector` — selects nodes on which kube-controller-manager is running;
      * Format — a label dictionary;
      * The default label is `node-role.kubernetes.io/control-plane: ""`;
    * `localPort` — the local kube-controller-manager port;
      * The default port is `10252`;
    * `scheme` — the HTTP scheme the local port uses;
      * Set to `http` by default;
    * `proxyListenPort` — the port reserved for the proxy server;
      * The default port is `10362`;
    * `authenticationMethod` — the authentication method;
      * Possible values:
        * `None` — do not authenticate;
        * `Certificate` — specify a custom certificate. (See the `certificateSecret` parameter below);
        * `ProxyServiceAccount` — configure the proxy's ServiceAccount (using RBAC) to collect component metrics;
      * The default value is `None`;
    * `certificateSecret` — the name of the secret in the `d8-system` namespace that stores the custom certificate. Mandatory if `authenticationMethod` == `Certificate`. The secret must contain two keys:
      * `client.crt` — a certificate;
      * `client.key` — a key;

* `kubeScheduler` — parameters for collecting kube-scheduler metrics;
  * `metricsPath` — path to metrics (`/metrics` by default);
  * `accessType` — specifies the way Prometheus accesses metrics;
    * Possible values:
      * `Pod` — kube-scheduler runs in a Pod, and the metrics port is accessible in the cluster via Kubernetes tools. (See the additional parameters in the `pod` section below);
      * `ThroughNode` — kube-scheduler runs on one or several nodes and is accessible from these nodes on the local port. In this case, a proxy runs on nodes to connect to Prometheus. (See the additional parameters in the `throughNode` section below);
    * The default value is `ThroughNode`;
  * `pod` — additional parameters for the `Pod` accessType;
    * `port` — the port at which metrics are exposed (mandatory);
    * `scheme` — the HTTP scheme the metrics http-port uses;
    * `podSelector` — a parameter to select service Pods (mandatory);
      * Format — a label dictionary;
    * `podNamespace` — namespace where kube-scheduler Pods are running (mandatory);
    * `authenticationMethod` —  the authentication method (mandatory);
      * Possible values:
        * `None` — do not authenticate;
        * `Certificate` — specify a custom certificate. (See the `certificateSecret` parameter below);
        * `PrometheusCertificate` — use the standard certificate that comes with the [prometheus](../../modules/300-prometheus/) module and grant it the appropriate RBAC-based rights to access metrics;
    * `certificateSecret` — the name of the secret in the `d8-system` namespace that stores the custom certificate. Mandatory if `authenticationMethod` == `Certificate`. The secret must contain two keys:
      * `client.crt` — a certificate;
      * `client.key` — a key;
  * `throughNode` — additional parameters for the `ThroughNode`.
    * `nodeSelector` — selects nodes on which kube-scheduler is running;
      * Format — a label dictionary;
      * The default label is `node-role.kubernetes.io/control-plane: ""`;
    * `localPort` — the local kube-scheduler port;
      * The default port is `10251`;
    * `scheme` — the HTTP scheme the local port uses;
      * Set to `http` by default;
    * `proxyListenPort` —  the port reserved for the proxy server;
      * The default port is `10363`;
    * `authenticationMethod` — the authentication method;
      * Possible values:
        * `None` — do not authenticate;
        * `Certificate` — specify a custom certificate. (See the `certificateSecret` parameter below).
        * `ProxyServiceAccount` — configure the proxy's ServiceAccount (using RBAC) to collect component metrics;
      * The default value is `None`.
    * `certificateSecret` — the name of the secret in the `d8-system` namespace that stores the custom certificate. Mandatory if `authenticationMethod` == `Certificate`. The secret must contain two keys:
      * `client.crt` — a certificate;
      * `client.key` — a key;

* `kubeEtcd` — parameters for collecting metrics of the primary kube-etcd instance;
  * `metricsPath` — path to metrics (`/metrics` by default);
  * `accessType` — specifies the way Prometheus accesses metrics;
    * Possible values:
      * `Pod` — kube-scheduler runs in a pod, and the metrics port is accessible in the cluster via Kubernetes tools. (See the additional parameters in the `pod` section below);
      * `ThroughNode` — kube-etcd runs on one or several nodes and is accessible from these nodes on the local port. In this case, a proxy runs on nodes to connect to Prometheus. (See the additional parameters in the `throughNode` section below);
    * The default value is `ThroughNode`;
  * `pod` — additional parameters for the `Pod` accessType;
    * `port` — the port at which metrics are exposed (mandatory);
    * `scheme` — the HTTP scheme the metrics http-port uses;
      * The default scheme is `https`;
    * `podSelector` — a parameter to select service Pods (mandatory);
      * Format — a label dictionary;
    * `podNamespace` — the namespace where kube-etcd Pods are running (mandatory);
    * `authenticationMethod` — the authentication method;
      * Possible values:
        * `Certificate` — specify a custom certificate. (See the `certificateSecret` parameter below).
        * _D8PKI_ — use the `kube-system/d8-pki`'s certificates (it is generated when bootstrapping the cluster using `dhctl`). This option is not intended for manual use (auto-configuring only);
      * Mandatory if the `kube-system/d8-pki` secret isn't available in the system;
    * `certificateSecret` — the name of the secret in the `d8-system` namespace that stores the custom certificate (mandatory). The secret must contain two keys:
      * `client.crt` — a certificate;
      * `client.key` — a key;
  * `throughNode` — additional parameters for the `ThroughNode`.
    * `nodeSelector` — selects nodes on which kube-etcd is running (mandatory);
      * Format — a label dictionary;
    * `localPort` — the local kube-etcd port;
      * By default:
        * It is calculated automatically using arguments derived from the kube-etcd pod;
        * If the calculation fails, the port is set to `2379`;
    * `scheme` — the HTTP scheme the metrics http-port uses;
      * The default scheme is `https`;
    * `proxyListenPort` —  the port reserved for the proxy server;
      * The default port is `10370`;
    * `authenticationMethod` — the authentication method;
      * Possible values:
        * `None` — do not authenticate;
        * `Certificate` — specify a custom certificate. (See the `certificateSecret` parameter below);
        * `HostPathCertificate` — use the certificate and key that are already present on the node's filesystem;
        * _D8PKI_ — use the `kube-system/d8-pki`'s certificates (it is generated when bootstrapping the cluster using `dhctl`). This option is not intended for manual use (auto-configuring only);
      * The default value is `HostPathCertificate`;
    * `certificateSecret` — the name of the secret in the `d8-system` namespace that stores the custom certificate. Mandatory if `authenticationMethod` == `Certificate`. The secret must contain two keys:
      * `client.crt` — a certificate;
      * `client.key` — a key;
    * `hostPathCertificate` — the path to the certificate on the node's filesystem;
      * By default:
        * It is calculated automatically using arguments derived from the kube-apiserver startup;
        * If the calculation fails, then it is set to `/etc/kubernetes/pki/apiserver-etcd-client.crt`;
    * `hostPathCertificateKey` — the path to the key on the node;
      * By default:
        * It is calculated automatically using arguments derived from the kube-apiserver startup;
        * If the calculation fails, then it is set to `/etc/kubernetes/pki/apiserver-etcd-client.key`;

* `kubeEtcdAdditionalInstances` — parameters for collecting metrics of the additional kube-etcd instance. They can be used, e.g., with kube-etcd-events on kops-based installations (it is discovered automatically in this module);
  * Format — an array of instances:
    * `name` — an instance's short name;
      * A mandatory parameter;
      * 12 characters max [a-z0-9].
    * `metricsPath` — path to metrics (`/metrics` by default);
    * `accessType` — specifies the way Prometheus accesses metrics;
      * Possible values:
        * `Pod` — the additional kube-scheduler runs in a pod, and the metrics port is accessible in the cluster via Kubernetes tools. (See the additional parameters in the `pod` section below);
        * `ThroughNode` — the additional kube-etcd runs on one or several nodes and is accessible from these nodes on the local port. In this case, a proxy runs on nodes to connect to Prometheus. (See the additional parameters in the `throughNode` section below);
      * A mandatory parameter;
    * `pod` — additional parameters for the `Pod` accessType;
      * `port` — the port at which metrics are exposed (mandatory);
      * `scheme` — the HTTP scheme the metrics http-port uses;
        * The default scheme is `https`;
      * `podSelector` — a parameter to select service Pods (mandatory);
        * Format — a label dictionary;
      * `podNamespace` — a namespace where additional kube-etcd Pods are running (mandatory);
      * `authenticationMethod` — the authentication method;
        * Possible values:
          * `Certificate` — specify a custom certificate. (See the `certificateSecret` parameter below);
          * _D8PKI_ — use the `kube-system/d8-pki`'s certificates (it is generated when bootstrapping the cluster using `dhctl`0. This option is not intended for manual use (auto-configuring only);
        * The mandatory parameter if the `kube-system/d8-pki` secret isn't present in the system;
      * `certificateSecret` — the name of the secret in the `d8-system` namespace that stores the custom certificate. A mandatory parameter; The secret must contain two keys:
        * `client.crt` — a certificate;
        * `client.key` — a key;
    * `throughNode` — additional parameters for the `ThroughNode` accessType;
      * `nodeSelector` — selects nodes on which the additional kube-etcd is running. A mandatory parameter;
        * Format — a label dictionary;
      * `localPort` — the local port of the additional kube-etcd. A mandatory parameter;
      * `scheme` — the HTTP scheme the metrics http-port uses;
        * The default scheme is `https`;
      * `proxyListenPort` —  the port reserved for the proxy server;
        * By default, it is calculated automatically using the following formula: `10370 + n`;
      * `authenticationMethod` — the authentication method;
        * Possible values:
          * `Certificate` — specify a custom certificate. (See the `certificateSecret` parameter below);
          * `HostPathCertificate` — use the certificate and key that are already present on the node's filesystem;
          * _D8PKI_ — use the `kube-system/d8-pki`'s certificates (it is generated when bootstrapping the cluster using `dhctl`). This option is not intended for manual use (auto-configuring only);
        * Mandatory if the `kube-system/d8-pki` isn't present in the system;
      * `certificateSecret` — the name of the secret in the `d8-system` namespace that stores the custom certificate. Mandatory if `authenticationMethod` == `Certificate`. The secret must contain two keys:
        * `client.crt` — a certificate;
        * `client.key` — a key;
      * `hostPathCertificate` — the path to the certificate on the node. Mandatory if `authenticationMethod` == `HostPathCertificate`;
      * `hostPathCertificateKey` — the path to the key on the node. Mandatory if `authenticationMethod` == `HostPathCertificate`.
