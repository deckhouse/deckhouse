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
      * `Pod` — kube-apiserver runs in a pod, and the metrics port is accessible in the cluster via Kubernetes tools. (See the additional parameters in the `pod` section below);
      * `ThroughNode` — kube-apiserver runs on one or several nodes and is accessible on the local port. In this case, a proxy runs on nodes to connect to Prometheus. (See the additional parameters in the `throughNode` section below);
    * Set to `DefaultService` by default;
  * `pod` — additional parameters for the `Pod` accessType;
    * `port` — the port at which metrics are exposed;
      * By default:
        * It is calculated automatically using arguments derived from the kube-apiserver pod;
        * If the calculation fails, the port is set to `6443`;
    * `podSelector` — a parameter to select service pods (mandatory);
      * Format — a label dictionary;
    * `podNamespace` — namespace where the component's pods are running (mandatory);
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
    * `port` — the port at which metrics are exposed;
    * `scheme` — http-схема, по которой работает http-порт с метриками.
    * `podSelector` — селектор подов с сервисом. Обязательный параметр.
      * Format — a label dictionary;
    * `podNamespace` — namespace, где работают поды kube-controller-manager. Обязательный параметр.
    * `authenticationMethod` —  the authentication method (mandatory);
      * Possible values:
        * `None` — не аутентифицироваться.
        * `Certificate` — specify a custom certificate. (See the `certificateSecret` parameter below).
        * `PrometheusCertificate` — использовать стандартный сертификат, который поставляется с модулем [prometheus](../../modules/300-prometheus/), и выдать ему соответствующие права на доступ к метрикам при помощи RBAC.
    * `certificateSecret` — the name of the secret in the `d8-system` namespace that stores the custom certificate. Mandatory if `authenticationMethod` == `Certificate`. The secret must contain two keys:
      * `client.crt` — a certificate;
      * `client.key` — a key;
  * `throughNode` — additional parameters for the `ThroughNode`.
    * `nodeSelector` — селектор нод, где работает kube-controller-manager.
      * Format — a label dictionary;
      * По умолчанию — `node-role.kubernetes.io/master: ""`
    * `localPort` — локальный порт kube-controller-manager.
      * По умолчанию — `10252`.
    * `scheme` — http-схема, по которой работает локальный порт.
      * По умолчанию — `http`.
    * `proxyListenPort` — на каком порту запустить прокси.
      * По умолчанию — `10362`.
    * `authenticationMethod` — как аутентифицироваться.
      * Possible values:
        * `None` — не аутентифицироваться.
        * `Certificate` — specify a custom certificate. (See the `certificateSecret` parameter below).
        * `ProxyServiceAccount` — configure the proxy's ServiceAccount (using RBAC) to collect component metrics;
      * По умолчанию — `None`.
    * `certificateSecret` — the name of the secret in the `d8-system` namespace that stores the custom certificate. Mandatory if `authenticationMethod` == `Certificate`. The secret must contain two keys:
      * `client.crt` — a certificate;
      * `client.key` — a key;

* `kubeScheduler` — параметры для сбора метрик kube-scheduler-а.
  * `metricsPath` — локейшн где искать метрики. По умолчанию `/metrics`.
  * `accessType` — способ организации доступа прометя к метрикам.
    * Possible values:
      * `Pod` — kube-scheduler работает в поде и порт с метриками доступен изнутри кластера средствами kubernetes. (See the additional parameters in the `pod` section below).
      * `ThroughNode` — kube-scheduler работает на одной или нескольких нодах и доступен с этих нод по локальному порту. В данном случае на нодах будет запущен прокси для обеспечения связи с прометеем. (See the additional parameters in the `throughNode` section below).
    * По умолчанию — `ThroughNode`.
  * `pod` — additional parameters for the `Pod`.
    * `port` — порт, где доступны метрики. Обязательный параметр.
    * `scheme` — http-схема, по которой работает http-порт с метриками.
    * `podSelector` — селектор подов с сервисом. Обязательный параметр.
      * Format — a label dictionary;
    * `podNamespace` — namespace, где работают поды kube-scheduler. Обязательный параметр.
    * `authenticationMethod` —  the authentication method (mandatory);
      * Possible values:
        * `None` — не аутентифицироваться.
        * `Certificate` — specify a custom certificate. (See the `certificateSecret` parameter below).
        * `PrometheusCertificate` — использовать стандартный сертификат, который поставляется с модулем [prometheus](../../modules/300-prometheus/), и выдать ему соответствующие права на доступ к метрикам при помощи RBAC.
    * `certificateSecret` — the name of the secret in the `d8-system` namespace that stores the custom certificate. Mandatory if `authenticationMethod` == `Certificate`. The secret must contain two keys:
      * `client.crt` — a certificate;
      * `client.key` — a key;
  * `throughNode` — additional parameters for the `ThroughNode`.
    * `nodeSelector` — селектор нод, где работает kube-scheduler.
      * Format — a label dictionary;
      * По умолчанию — `node-role.kubernetes.io/master: ""`.
    * `localPort` — локальный порт kube-scheduler.
      * По умолчанию — `10251`.
    * `scheme` — http-схема, по которой работает локальный порт.
      * По умолчанию — `http`.
    * `proxyListenPort` — на каком порту запустить прокси.
      * По умолчанию — `10363`.
    * `authenticationMethod` — как аутентифицироваться.
      * Possible values:
        * `None` — не аутентифицироваться.
        * `Certificate` — specify a custom certificate. (See the `certificateSecret` parameter below).
        * `ProxyServiceAccount` — configure the proxy's ServiceAccount (using RBAC) to collect component metrics;
      * По умолчанию — `None`.
    * `certificateSecret` — the name of the secret in the `d8-system` namespace that stores the custom certificate. Mandatory if `authenticationMethod` == `Certificate`. The secret must contain two keys:
      * `client.crt` — a certificate;
      * `client.key` — a key;

* `kubeEtcd` — параметры для сбора метрик основного инстанса kube-etcd`.
  * `metricsPath` — локейшн где искать метрики. По умолчанию `/metrics`.
  * `accessType` — способ организации доступа прометя к метрикам.
    * Possible values:
      * `Pod` — kube-etcd работает в поде и порт с метриками доступен изнутри кластера средствами kubernetes. (See the additional parameters in the `pod` section below).
      * `ThroughNode` — kube-etcd работает на одной или нескольких нодах и доступен с этих нод по локальному порту. В данном случае на нодах будет запущен прокси для обеспечения связи с прометеем. (See the additional parameters in the `throughNode` section below).
    * По умолчанию — `ThroughNode`.
  * `pod` — additional parameters for the `Pod`.
    * `port` — порт, где доступны метрики. Обязательный параметр.
    * `scheme` — http-схема, по которой работает http-порт с метриками.
      * По умолчанию — `https`.
    * `podSelector` — селектор подов с сервисом. Обязательный параметр.
      * Format — a label dictionary;
    * `podNamespace` — namespace, где работают поды kube-etcd. Обязательный параметр.
    * `authenticationMethod` — как аутентифицироваться.
      * Possible values:
        * `Certificate` — specify a custom certificate. (See the `certificateSecret` parameter below).
        * _D8PKI_ — использовать сертификаты из секрета `kube-system/d8-pki`, который генерируется при бутстрапе кластера средствами `dhctl`. Данный вариант не предназначен для ручного использования, только для автонастройки.
      * Mandatory if в системе нет секрета `kube-system/d8-pki`.
    * `certificateSecret` — the name of the secret in the `d8-system` namespace that stores the custom certificate. Обязательный параметр. The secret must contain two keys:
      * `client.crt` — a certificate;
      * `client.key` — a key;
  * `throughNode` — additional parameters for the `ThroughNode`.
    * `nodeSelector` — селектор нод, где работает kube-etcd. Обязательный параметр.
      * Format — a label dictionary;
    * `localPort` — локальный порт kube-etcd.
      * По умолчанию:
        * Вычисляется автоматически на основе аргументов из пода с kube-etcd.
        * Если не удалось, то — `2379`.
    * `scheme` — http-схема, по которой работает http-порт с метриками.
      * По умолчанию — `https`.
    * `proxyListenPort` — на каком порту запустить прокси.
      * По умолчанию — `10370`.
    * `authenticationMethod` — как аутентифицироваться.
      * Possible values:
        * `Certificate` — specify a custom certificate. (See the `certificateSecret` parameter below).
        * `HostPathCertificate` — использовать сертификат и ключ, которые уже лежат на ФС ноды.
        * _D8PKI_ — использовать сертификаты из секрета `kube-system/d8-pki`, который генерируется при бутстрапе кластера средствами `dhctl`. Данный вариант не предназначен для ручного использования, только для автонастройки.
      * По умолчанию — `HostPathCertificate`.
    * `certificateSecret` — the name of the secret in the `d8-system` namespace that stores the custom certificate. Mandatory if `authenticationMethod` == `Certificate`. The secret must contain two keys:
      * `client.crt` — a certificate;
      * `client.key` — a key;
    * `hostPathCertificate` — путь до сертификата на ФС ноды.
      * По умолчанию:
        * Вычисляется автоматически на основе аргументов запуска kube-apiserver.
        * Если не удалось, то — `/etc/kubernetes/pki/apiserver-etcd-client.crt`.
    * `hostPathCertificateKey` — путь до ключа на ФС ноды.
      * По умолчанию:
        * Вычисляется автоматически на основе аргументов запуска kube-apiserver.
        * Если не удалось, то — `/etc/kubernetes/pki/apiserver-etcd-client.key`.

* `kubeEtcdAdditionalInstances` — параметры для сбора метрик дополнительного инстанса kube-etcd. Например, для kube-etcd-events в kops-инсталляциях (который в данном модуле дискаверится автоматически).
  * Формат — массив инстансов:
    * `name` — короткое имя для инстанса.
      * Обязательный параметр.
      * Не более 12 символов [a-z0-9].
    * `metricsPath` — локейшн где искать метрики. По умолчанию — `/metrics`.
    * `accessType` — способ организации доступа прометя к метрикам.
      * Possible values:
        * `Pod` — дополнительный kube-etcd работает в поде и порт с метриками доступен изнутри кластера средствами kubernetes. (See the additional parameters in the `pod` section below).
        * `ThroughNode` — дополнительный kube-etcd работает на одной или нескольких нодах и доступен с этих нод по локальному порту. В данном случае на нодах будет запущен прокси для обеспечения связи с прометеем. (See the additional parameters in the `throughNode` section below).
      * Обязательный параметр.
    * `pod` — additional parameters for the `Pod`.
      * `port` — порт, где доступны метрики. Обязательный параметр.
      * `scheme` — http-схема, по которой работает http-порт с метриками.
        * По умолчанию — `https`.
      * `podSelector` — селектор подов с сервисом. Обязательный параметр.
        * Format — a label dictionary;
      * `podNamespace` — namespace, где работают поды с дополнительными kube-etcd. Обязательный параметр.
      * `authenticationMethod` — как аутентифицироваться.
        * Possible values:
          * `Certificate` — specify a custom certificate. (See the `certificateSecret` parameter below).
          * _D8PKI_ — использовать сертификаты из секрета `kube-system/d8-pki`, который генерируется при бутстрапе кластера средствами `dhctl`. Данный вариант не предназначен для ручного использования, только для автонастройки.
        * Обязательный параметр, если в системе нет секрета `kube-system/d8-pki`.
      * `certificateSecret` — the name of the secret in the `d8-system` namespace that stores the custom certificate. Обязательный параметр. The secret must contain two keys:
        * `client.crt` — a certificate;
        * `client.key` — a key;
    * `throughNode` — additional parameters for the `ThroughNode`.
      * `nodeSelector` — селектор нод, где работает дополнительней kube-etcd. Обязательный параметр.
        * Format — a label dictionary;
      * `localPort` — локальный порт дополнительного kube-etcd. Обязательный параметр.
      * `scheme` — http-схема, по которой работает http-порт с метриками.
        * По умолчанию — `https`.
      * `proxyListenPort` — на каком порту запустить прокси.
        * По умолчанию — вычисляется автоматически по формуле `10370 + n`.
      * `authenticationMethod` — как аутентифицироваться.
        * Possible values:
          * `Certificate` — specify a custom certificate. (See the `certificateSecret` parameter below).
          * `HostPathCertificate` — использовать сертификат и ключ, которые уже лежат на ФС ноды.
          * _D8PKI_ — использовать сертификаты из секрета `kube-system/d8-pki`, который генерируется при бутстрапе кластера средствами `dhctl`. Данный вариант не предназначен для ручного использования, только для автонастройки.
        * Mandatory if в системе нет секрета `kube-system/d8-pki`.
      * `certificateSecret` — the name of the secret in the `d8-system` namespace that stores the custom certificate. Mandatory if `authenticationMethod` == `Certificate`. The secret must contain two keys:
        * `client.crt` — a certificate;
        * `client.key` — a key;
      * `hostPathCertificate` — путь до сертификата на ФС ноды. Mandatory if `authenticationMethod` == `HostPathCertificate`.
      * `hostPathCertificateKey` — путь до ключа на ФС ноды.Mandatory if `authenticationMethod` == `HostPathCertificate`.
