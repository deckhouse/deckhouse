---
title: "Running Argo CD"
permalink: en/admin/configuration/delivery/argocd/setup/
description: "Running Argo CD in Deckhouse Kubernetes Platform."
lang: en
relatedLinks:
  - title: "Official Argo CD website"
    url: "https://argo-cd.readthedocs.io"
  - title: "Official Argo CD Operator website"
    url: "https://argocd-operator.readthedocs.io"
---

This section describes the steps required to run an Argo CD instance in a DKP cluster:

- [Preparing to run Argo CD](#preparing-to-run-argo-cd)
- Deploying [one](#deploying-an-argo-cd-instance) or [multiple](#deploying-multiple-argo-cd-instances) Argo CD instances
- [Additional settings](#advanced-settings) for Argo CD

## Preparing to run Argo CD

Before creating Argo CD instances, complete the following steps:

1. [Enable the `operator-argo` module](/modules/operator-argo/configuration.html#enable).

1. Wait until the module switches to the `Ready` state.

   You can check the module state in the DKP web interface or with the following command:

   ```bash
   d8 k get module operator-argo -w
   ```

Detailed information about module settings is available in the [`operator-argo` module documentation](/modules/operator-argo/).

After you enable the `operator-argo` module, Argo CD custom resources become available in the DKP cluster.

To run an Argo CD instance, create an [ArgoCD](/modules/operator-argo/cr.html#argocd) object.
Working with custom resources that belong to an Argo CD instance is described in the [Usage](/products/kubernetes-platform/documentation/v1/user/delivery/argocd/) section.

{% offtopic title="List of Argo CD custom resources created by the <code>operator-argo</code> module..." %}

| CRD | Scope | Purpose |
|---|---|---|
| [`argocds.argoproj.io`](/modules/operator-argo/cr.html#argocd) | **`operator-argo` module** | A custom resource used by the `operator-argo` module to configure and maintain an Argo CD instance in the target environment. Installation parameters, component set, integrations, access policies, and other instance settings are defined through the ArgoCD object. |
| [`appprojects.argoproj.io`](/modules/operator-argo/cr.html#appproject) | **Argo&nbsp;CD instance** | Used for logical segmentation of applications and for defining policies: allowed Git repositories, target clusters and namespaces, as well as access rules and restrictions on resource usage. |
| [`applications.argoproj.io`](/modules/operator-argo/cr.html#application) | **Argo&nbsp;CD instance** | The main Argo CD application CRD for describing an application that must be synchronized from a declarative source (Git, Helm, Kustomize, and others) into Kubernetes. Defines the source, target environment, and synchronization parameters. |
| [`applicationsets.argoproj.io`](/modules/operator-argo/cr.html#applicationset) | **Argo&nbsp;CD instance** | Used for automated creation of a set of Application objects from a template. Suitable for mass application management scenarios across multiple clusters, environments, directories, teams, or repository branches. |
| [`argocdexports.argoproj.io`](/modules/operator-argo/cr.html#argocdexport) | **Argo&nbsp;CD instance** | Used for declarative export of data related to an Argo CD instance to external systems or related DKP components. Typically applied in integration scenarios where configuration, status, or access information must be published in a formalized way. |
| [`namespacemanagements.argoproj.io`](/modules/operator-argo/cr.html#namespacemanagement) | **Argo&nbsp;CD instance** | Used to manage the lifecycle of namespaces within GitOps processes. Can automate creation, configuration, and maintenance of namespaces into which applications are then deployed. |
| [`notificationsconfigurations.argoproj.io`](/modules/operator-argo/cr.html#notificationsconfiguration) | **Argo&nbsp;CD instance** | Used to configure the Argo&nbsp;CD notification mechanism. Lets you declaratively describe delivery channels and rules for sending events related to synchronization, deployment errors, application status changes, and other operational events. |
| [`imageupdaters.argocd-image-updater.argoproj.io`](/modules/operator-argo/cr.html#imageupdater) | **Argo&nbsp;CD instance** | Used to automatically track new container image versions and update application parameters according to the defined versioning and publishing strategy. |
{% endofftopic %}

## Deploying an Argo CD instance

Parameters available for configuring an Argo CD instance are listed in the [`operator-argo` module documentation](/modules/operator-argo/cr.html#argocd).

To deploy an Argo CD instance in the `argocd` namespace and publish the Argo CD web interface through Ingress, use the following example (provide your own values for `<ARGOCD_DOMAIN>` and `<TLS_SECRET_NAME>`):

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: argocd
---
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: argocd
  namespace: argocd
spec:
  server:
    host: <ARGOCD_DOMAIN>
    ingress:
      enabled: true
      tls:
        - hosts:
            - <ARGOCD_DOMAIN>
          secretName: <TLS_SECRET_NAME>
    insecure: true
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: argocd-ingress
  namespace: argocd
spec:
  dnsNames:
    - <ARGOCD_DOMAIN>
  issuerRef:
    kind: ClusterIssuer
    name: letsencrypt
  secretName: <TLS_SECRET_NAME>
```

{% alert level="warning" %}
The [`spec.server.insecure: true`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-server-insecure) parameter in the example above disables internal TLS on the Argo CD API server. This helps avoid redirect loops when publishing through Ingress.
{% endalert %}

After you create the ArgoCD object, Argo CD components are started in the `argocd` namespace:

```console
d8 k -n argocd get pods
NAME                                  READY   STATUS    RESTARTS   AGE
argocd-application-controller-0       1/1     Running   0          35m
argocd-dex-server-759fff8444-zglp4    1/1     Running   0          2d23h
argocd-redis-568f5b889c-jg5dr         1/1     Running   0          4d
argocd-repo-server-78d9d6bcc6-9rwcm   1/1     Running   0          3d22h
argocd-server-76597597f9-kfqdl        1/1     Running   0          35m
podinfo-ccdb96645-zv5tm               1/1     Running   0          2d20h
```

When all components switch to the `Running` status, the web interface of the Argo CD instance becomes available at the address specified in the [`spec.server.ingress.tls.hosts`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-server-ingress-tls-hosts) parameter (in the example — `https://<ARGOCD_DOMAIN>`).
Authentication setup and credentials retrieval are described in the [Configuring authentication and authorization](../authentication/) section.

Application delivery with Argo CD is described in the [Usage](../../../../../user/delivery/argocd/) section.

{% alert level="info" %}
You can work with Argo CD not only through the web interface and custom resources, but also with the `argocd` CLI utility. You can download the binary from the "Documentation" section of the Argo CD web interface. To get help for the CLI utility, run `argocd --help`.
{% endalert %}

## Deploying multiple Argo CD instances

If the cluster needs multiple Argo CD instances, create a separate namespace (or DKP project) and a separate ArgoCD object for each of them.

For example, create:

- a separate instance for the production environment;
- a separate instance for test environments;
- a separate instance for a specific team or project.

With this approach, each Argo CD instance is managed independently and has its own configuration.

{% alert level="warning" %}
Creating more than one ArgoCD object in a single namespace is not supported.
{% endalert %}

## Advanced settings

### Enabling high availability mode

To enable high availability mode, set the [`spec.ha.enabled: true`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-ha-enabled) parameter in the ArgoCD object.

Example:

```yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: argocd
  namespace: argocd
spec:
  ha:
    enabled: true
    resources:
      requests:
        cpu: "500m"
        memory: "512Mi"
      limits:
        cpu: "1"
        memory: "1Gi"
```

{% alert level="warning" %}
High availability mode provides high availability only for the `Redis` state store using `HAProxy`. It does not automatically make all Argo CD components highly available.
{% endalert %}

{% alert level="info" %}
Running Argo CD in high availability mode requires at least three cluster nodes because of `pod anti-affinity` rules. Clusters that use IPv6 only are not supported.

When high availability mode is enabled, changes in [`.spec.redis.resources`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-redis-resources) are not applied. Configure resource limits and requests for `Redis` through the [`.spec.ha.resources`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-ha-resources) parameter.
{% endalert %}

### Granting access to cluster resources

By default, an Argo CD instance receives privileges only for the namespace where it runs and for namespaces labeled with `argocd.argoproj.io/managed-by`. The label value must match the name of the namespace where the Argo CD instance runs.

To allow creation of cluster-wide resources, specify the namespace of the target Argo CD instance in the [`clusterConfigNamespaces`](/modules/operator-argo/configuration.html#parameters-clusterconfignamespaces) parameter of the `operator-argo` module settings.

Example:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: operator-argo
spec:
  enabled: true
  settings:
    clusterConfigNamespaces: argocd
  version: 1
```

If there are multiple Argo CD instances, list them in the `clusterConfigNamespaces` parameter as a comma-separated list.

After you change the `ModuleConfig`, the following ClusterRole and ClusterRoleBinding objects are created automatically:

```yaml
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  annotations:
    argocds.argoproj.io/name: argocd
    argocds.argoproj.io/namespace: argocd
  labels:
    app.kubernetes.io/managed-by: argocd
    app.kubernetes.io/name: argocd
    app.kubernetes.io/part-of: argocd
  name: argocd-argocd-argocd-application-controller
rules:
- apiGroups:
  - '*'
  resources:
  - '*'
  verbs:
  - '*'
- apiGroups:
  - ""
  resources:
  - serviceaccounts
  verbs:
  - impersonate
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  annotations:
    argocds.argoproj.io/name: argocd
    argocds.argoproj.io/namespace: argocd
  labels:
    app.kubernetes.io/managed-by: argocd
    app.kubernetes.io/name: argocd-application-controller
    app.kubernetes.io/part-of: argocd
  name: argocd-argocd-argocd-application-controller

roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: argocd-argocd-argocd-application-controller
subjects:
- kind: ServiceAccount
  name: argocd-argocd-application-controller
  namespace: argocd
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  annotations:
    argocds.argoproj.io/name: argocd
    argocds.argoproj.io/namespace: argocd
  labels:
    app.kubernetes.io/managed-by: argocd
    app.kubernetes.io/name: argocd
    app.kubernetes.io/part-of: argocd
  name: argocd-argocd-argocd-server
rules:
- apiGroups:
  - '*'
  resources:
  - '*'
  verbs:
  - get
  - delete
  - patch
- apiGroups:
  - argoproj.io
  resources:
  - applications
  - applicationsets
  verbs:
  - list
  - watch
- apiGroups:
  - ""
  resources:
  - events
  verbs:
  - list
- apiGroups:
  - batch
  resources:
  - jobs
  - cronjobs
  - cronjobs/finalizers
  verbs:
  - create
  - update
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  annotations:
    argocds.argoproj.io/name: argocd
    argocds.argoproj.io/namespace: argocd
  labels:
    app.kubernetes.io/managed-by: argocd
    app.kubernetes.io/name: argocd-server
    app.kubernetes.io/part-of: argocd
  name: argocd-argocd-argocd-server
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: argocd-argocd-argocd-server
subjects:
- kind: ServiceAccount
  name: argocd-argocd-server
  namespace: argocd
```

{% alert level="info" %}
If the privileges described in the default cluster roles are excessive, set [`spec.defaultClusterScopedRoleDisabled: true`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-defaultclusterscopedroledisabled) in the ArgoCD object settings. In this case, cluster roles are not created automatically, and you can define the required privilege level for the ServiceAccount used by the Argo CD instance yourself.
{% endalert %}

{% alert level="info" %}
If needed, you can override the access level to cluster resources at the [AppProject](/modules/operator-argo/cr.html#appproject) object level using the [`spec.clusterResourceBlacklist`](/modules/operator-argo/cr.html#appproject-v1alpha1-spec-clusterresourceblacklist) and [`spec.clusterResourceWhitelist`](/modules/operator-argo/cr.html#appproject-v1alpha1-spec-clusterresourcewhitelist) parameters.
{% endalert %}

### Using a custom cluster domain

If the cluster uses a domain other than `cluster.local`, specify it in the [`spec.clusterDomain`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-clusterdomain) parameter of the ArgoCD object.

Example:

```yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: argocd
  namespace: argocd
spec:
  clusterDomain: prod.local
```
