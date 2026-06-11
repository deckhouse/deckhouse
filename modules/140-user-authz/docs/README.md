---
title: "The user-authz module"
description: "Authorization and role-based access control to the resources of the Deckhouse Kubernetes Platform cluster."
---

The module generates role-based access model objects based on the standard Kubernetes RBAC mechanism. The module creates a set of cluster roles (`ClusterRole`) suitable for most user and group access management tasks.

{% alert level="warning" %}
Starting from Deckhouse Kubernetes Platform v1.64, the module features a experimental role-based access model. The current role-based access model will continue to operate but support for it will be discontinued in the future.

The experimental role-based access model is incompatible with the current one.
{% endalert %}

The module implements a role-based access model based on the standard RBAC Kubernetes mechanism. It creates a set of cluster roles (`ClusterRole`) suitable for most user and group access management tasks.

<div style="height: 0;" id="the-new-role-based-model"></div>

## Experimental role-based model

Unlike the [current DKP role-based model](#current-role-based-model), the new role-based one does not use `ClusterAuthorizationRule` and `AuthorizationRule` resources. All access rights are configured in the standard Kubernetes RBAC way, i.e., by creating `RoleBinding` or `ClusterRoleBinding` resources and specifying one of the roles prepared by the `user-authz` module in them.

The module creates special aggregated cluster roles (`ClusterRole`). By using these roles in `RoleBinding` or `ClusterRoleBinding`, you can do the following:
- Manage access to modules of a specific [subsystem](#subsystems-of-the-role-based-model).

  For example, you can use the `d8:subsystem:networking:manager` role in `ClusterRoleBinding` to allow a network administrator to configure *network* modules (such as `cni-cilium`, `ingress-nginx`, `istio`, etc.).
- Manage access to *user* resources of modules within the namespace.

  For example, the `d8:namespace:manager` role in `RoleBinding` enables deleting/creating/editing the [PodLoggingConfig](../log-shipper/cr.html#podloggingconfig) resource in the namespace. At the same time, it does not grant access to the cluster-wide [ClusterLoggingConfig](../log-shipper/cr.html#clusterloggingconfig) and [ClusterLogDestination](../log-shipper/cr.html#clusterlogdestination) resources of the `log-shipper` module, nor does it allow configuration of the `log-shipper` module itself.

The roles created by the module are divided into two classes:
- [Namespace roles](#namespace-roles) — for assigning rights to users (such as application developers) **in a specific namespace**.
- [System and subsystem roles](#system-and-subsystem-roles) — for assigning rights to administrators.

<div style="height: 0;" id="use-roles"></div>

### Namespace roles

{% alert level="warning" %}
The namespace role can only be used in the `RoleBinding` resource.
{% endalert %}

Namespace roles are intended to assign rights to a user **in a specific namespace**. Users refer to, for example, developers who use a cluster configured by an administrator to deploy their applications. Such users don't need to manage DKP modules or a cluster, but they need to be able to, for example, create their Ingress resources, configure application authentication, and collect logs from applications.

The namespace role defines permissions for accessing namespaced resources of modules and standard namespaced resources of Kubernetes (`Pod`, `Deployment`, `Secret`, `ConfigMap`, etc.).

The module creates the following namespace roles:
- `d8:namespace:viewer` — allows viewing standard Kubernetes resources in a specific namespace, except for Secrets and RBAC resources, as well as authenticating in the cluster;
- `d8:namespace:user` — in addition to the role `d8:namespace:viewer` it allows viewing secrets and RBAC resources in a specific namespace, connecting to pods, deleting pods (but not creating or modifying them), executing `kubectl port-forward` and `kubectl proxy`, as well as changing the number of replicas of controllers;
- `d8:namespace:manager` — in addition to the role `d8:namespace:user` it allows managing module resources (for example, `Certificate`, `PodLoggingConfig`, etc.) and standard namespaced Kubernetes resources (`Pod`, `ConfigMap`, `CronJob`, etc.) in a specific namespace;
- `d8:namespace:admin` — in addition to the role `d8:namespace:manager` it allows managing the resources `ResourceQuota`, `ServiceAccount`, `Role`, `RoleBinding`, `NetworkPolicy` in a specific namespace;
- `d8:namespace:superadmin` — in addition to the role `d8:namespace:admin` it allows managing system resources placed in the namespace (for example, Dex pods or pods/PVCs of virtual machines).

<div style="height: 0;" id="manage-roles"></div>

### System and subsystem roles

{% alert level="warning" %}
The system and subsystem roles do not grant access to the namespace of user applications.

They grant access only to system namespaces (starting with `d8-` or `kube-`), and only to those system namespaces where the modules of the corresponding role subsystem are running.
{% endalert %}

System and subsystem roles are intended for assigning rights to manage the entire platform or a part of it (the [subsystem](#subsystems-of-the-role-based-model)), but not the users applications themselves. The subsystem role, for example, can allow a security administrator to manage security modules (responsible for the security functions of the cluster). Thus, the security administrator will be able to configure authentication, authorization, security policies, etc., but will not be able to manage other cluster functions (such as network and monitoring settings) or change settings in the namespaces of users applications.

The system/subsystem role defines access rights:
- to cluster-wide Kubernetes resources;
- to manage DKP modules (`moduleConfig` resource) within the [subsystem](#subsystems-of-the-role-based-model) of the role, or to all DKP modules for the role `d8:system:*`;
- to manage cluster-wide resources of DKP modules within the [subsystem](#subsystems-of-the-role-based-model) of the role, or to all resources of DKP modules for the role `d8:system:*`;
- to system namespaces (starting with `d8-` or `kube-`) in which the modules of the [subsystem](#subsystems-of-the-role-based-model) of the role operate, or to all system namespaces for the role `d8:system:*`.

The role name format is `d8:system:<ACCESS_LEVEL>` for the system roles and `d8:subsystem:<SUBSYSTEM>:<ACCESS_LEVEL>` for the subsystem roles, where:
- `SUBSYSTEM` is the role's [subsystem](#subsystems-of-the-role-based-model);
- `ACCESS_LEVEL` is the access level.

  Examples:
  - `d8:system:viewer` — access to view the configuration of all DKP modules (`moduleConfig` resource), their cluster-wide resources, their namespaced resources, and standard Kubernetes objects (except Secrets and RBAC resources) in all system namespaces (starting with `d8-` or `kube-`);
  - `d8:system:manager` — similar to the role `d8:system:viewer`, but with admin-level access, i.e., view/create/modify/delete the configuration of all DKP modules (`moduleConfig` resource), their cluster-wide resources, their namespaced resources, and standard Kubernetes objects in all system namespaces (starting with `d8-` or `kube-`);
  - `d8:subsystem:observability:viewer` — access to view the configuration of DKP modules (`moduleConfig` resource) from the `observability` area, their cluster-wide resources, their namespaced resources, and standard Kubernetes objects (except secrets and RBAC resources) in the system namespaces `d8-log-shipper`, `d8-monitoring`, `d8-okmeter`, `d8-operator-prometheus`, `d8-upmeter`, `kube-prometheus-pushgateway`.

The module provides three access levels for administrators:
- `viewer` — allows viewing standard Kubernetes resources, the configuration of modules (resources `moduleConfig`), cluster-wide resources of modules, and namespaced resources of modules in the module namespace;
- `manager` — in addition to the level `viewer` it allows managing standard Kubernetes resources, the configuration of modules (resources `moduleConfig`), cluster-wide resources of modules, and namespaced resources of modules in the module namespace;
- `superadmin` — in addition to the level `manager` it allows managing system resources of the subsystem modules.

### Subsystems of the role-based model

Each DKP module belongs to a specific subsystem. For each subsystem, there is a set of roles with different levels of access. Roles are updated automatically when the module is enabled or disabled.

For example, for the `networking` subsystem, there are the following subsystem roles that can be used in `ClusterRoleBinding`:

- `d8:subsystem:networking:viewer`
- `d8:subsystem:networking:manager`
- `d8:subsystem:networking:superadmin`

The scope of a role depends on which subsystem it belongs to:

- The scope of the `d8:system:*` roles is all system namespaces (starting with `d8-` or `kube-`) in the cluster.
- The scope of subsystem roles includes the namespaces in which the subsystem’s modules operate (see the subsystem composition table), as well as all cluster-wide objects of the subsystem’s modules.

Role-based model subsystems composition table.

{% include rbac/rbac-subsystems-list.liquid %}

<div style="height: 0;" id="the-obsolete-role-based-model"></div>

## Current role-based model

Features:
- Manages user and group access control using Kubernetes RBAC;
- Manages access to scaling tools (the `allowScale` parameter of the [`ClusterAuthorizationRule`](cr.html#clusterauthorizationrule-v1-spec-allowscale) or [AuthorizationRule](cr.html#authorizationrule-v1alpha1-spec-allowscale) Custom Resource);
- Manages access to port forwarding (the `portForwarding` parameter of the [`ClusterAuthorizationRule`](cr.html#clusterauthorizationrule-v1-spec-portforwarding) or [AuthorizationRule](cr.html#authorizationrule-v1alpha1-spec-portforwarding) Custom Resource);
- Manages the list of allowed namespaces with a labelSelector (the `namespaceSelector` parameter of the [`ClusterAuthorizationRule`](cr.html#clusterauthorizationrule-v1-spec-namespaceselector) Custom Resource);

In addition to the RBAC, you can use a set of high-level roles in the module:
- `User` — has access to information about all objects (including viewing pod logs) but cannot exec into containers, read secrets, and perform port-forwarding;
- `PrivilegedUser` — the same as `User` + can exec into containers, read secrets, and delete pods (and thus, restart them);
- `Editor` — is the same as `PrivilegedUser` + can create and edit all objects that are usually required for application tasks.
- `Admin` — the same as `Editor` + can delete service objects (auxiliary resources such as `ReplicaSet`, `certmanager.k8s.io/challenges` and `certmanager.k8s.io/orders`);
- `ClusterEditor` — the same as `Editor` + can manage a limited set of `cluster-wide` objects that can be used in application tasks (`ClusterXXXMetric`, `KeepalivedInstance`, `DaemonSet`, etc.). This role is best suited for cluster operators.
- `ClusterAdmin` — the same as both `ClusterEditor` and `Admin` + can manage `cluster-wide` service objects (e.g.,  `MachineSets`, `Machines`, `OpenstackInstanceClasses`..., as well as `ClusterAuthorizationRule`, `ClusterRoleBindings` and `ClusterRole`). This role is best suited for cluster administrators. **Note** that since `ClusterAdmin` can edit `ClusterRoleBindings`, he can **broaden his privileges within the cluster**;
- `SuperAdmin` — can perform any actions with any objects (note that `namespaceSelector` and `limitNamespaces` restrictions remain valid).

{% alert level="warning" %}
Currently, the multi-tenancy mode (namespace-based authorization) is implemented according to a temporary scheme and **isn't guaranteed to be entirely safe and secure**!
{% endalert %}

If a [`ClusterAuthorizationRule`](cr.html#clusterauthorizationrule) Custom Resource contains the `namespaceSelector` field, neither `limitNamespaces` nor `allowAccessToSystemNamespaces`are taken into consideration.

The `allowAccessToSystemNamespaces`, `namespaceSelector` and `limitNamespaces` options in the custom resource will no longer be applied if the authorization system's webhook is unavailable for some reason. As a result, users will have access to all namespaces. After the webhook availability is restored, the options will become relevant again.

### Default access list for each role

Each next role inherits permissions from the previous roles. A role block shows only the permissions added by that role.

The list below includes:

- standard permissions from the current role-based model (k8s permissions);
- permissions created by Deckhouse’s built-in modules.

It does not include permissions for [modules from source](/products/kubernetes-platform/documentation/v1/architecture/module-development/run/#module-source).

When enabled in a cluster, modules from source create permissions for the resources they provide. When a module from source is disabled, the permissions it created are removed.

To view the permissions created by source modules, use the [command](#get_rules).

`verbs` aliases:
<!-- start user-authz roles placeholder -->
* read - `get`, `list`, `watch`
* read-write - `get`, `list`, `watch`, `create`, `delete`, `deletecollection`, `patch`, `update`
* write - `create`, `delete`, `deletecollection`, `patch`, `update`

{{site.data.i18n.common.role[page.lang] | capitalize }} `User`:

```text
read:
    - acme.cert-manager.io/orders
    - apiextensions.k8s.io/customresourcedefinitions
    - apps/daemonsets
    - apps/deployments
    - apps/replicasets
    - apps/statefulsets
    - autoscaling.k8s.io/verticalpodautoscalercheckpoints
    - autoscaling.k8s.io/verticalpodautoscalers
    - autoscaling/horizontalpodautoscalers
    - batch/cronjobs
    - batch/jobs
    - cert-manager.io/certificaterequests
    - cert-manager.io/certificates
    - cert-manager.io/challenges
    - cert-manager.io/clusterissuers
    - cert-manager.io/issuers
    - cilium.io/ciliumclusterwidenetworkpolicies
    - cilium.io/ciliumnetworkpolicies
    - config.gatekeeper.sh/configs
    - configmaps
    - connection.gatekeeper.sh/connections
    - constraints.gatekeeper.sh/*
    - deckhouse.io/applicationpackages
    - deckhouse.io/applicationpackageversions
    - deckhouse.io/applications
    - deckhouse.io/awsinstanceclasses
    - deckhouse.io/azureinstanceclasses
    - deckhouse.io/clusterdaemonsetmetrics
    - deckhouse.io/clusterdeploymentmetrics
    - deckhouse.io/clusteringressmetrics
    - deckhouse.io/clusterpodmetrics
    - deckhouse.io/clusterservicemetrics
    - deckhouse.io/clusterstatefulsetmetrics
    - deckhouse.io/daemonsetmetrics
    - deckhouse.io/deckhousereleases
    - deckhouse.io/deploymentmetrics
    - deckhouse.io/deschedulers
    - deckhouse.io/dexauthenticators
    - deckhouse.io/dexclients
    - deckhouse.io/dvpinstanceclasses
    - deckhouse.io/dynamixinstanceclasses
    - deckhouse.io/gcpinstanceclasses
    - deckhouse.io/huaweicloudinstanceclasses
    - deckhouse.io/hubblemonitoringconfigs
    - deckhouse.io/ingressmetrics
    - deckhouse.io/instances
    - deckhouse.io/keepalivedinstances
    - deckhouse.io/localpathprovisioners
    - deckhouse.io/moduledocumentations
    - deckhouse.io/modulepulloverrides
    - deckhouse.io/modulereleases
    - deckhouse.io/modules
    - deckhouse.io/modulesources
    - deckhouse.io/moduleupdatepolicies
    - deckhouse.io/namespacemetrics
    - deckhouse.io/nodegroups
    - deckhouse.io/openstackinstanceclasses
    - deckhouse.io/operationpolicies
    - deckhouse.io/packagerepositories
    - deckhouse.io/packagerepositoryoperations
    - deckhouse.io/podmetrics
    - deckhouse.io/projects
    - deckhouse.io/projecttemplates
    - deckhouse.io/securitypolicies
    - deckhouse.io/securitypolicyexceptions
    - deckhouse.io/servicemetrics
    - deckhouse.io/statefulsetmetrics
    - deckhouse.io/vcdaffinityrules
    - deckhouse.io/vcdinstanceclasses
    - deckhouse.io/vsphereinstanceclasses
    - deckhouse.io/yandexinstanceclasses
    - deckhouse.io/zvirtinstanceclasses
    - discovery.k8s.io/endpointslices
    - endpoints
    - events
    - events.k8s.io/events
    - expansion.gatekeeper.sh/expansiontemplate
    - extensions.istio.io/wasmplugins
    - extensions/daemonsets
    - extensions/deployments
    - extensions/ingresses
    - extensions/replicasets
    - extensions/replicationcontrollers
    - externaldata.gatekeeper.sh/providers
    - gateway.networking.k8s.io/backendtlspolicies
    - gateway.networking.k8s.io/gatewayclasses
    - gateway.networking.k8s.io/gateways
    - gateway.networking.k8s.io/grpcroutes
    - gateway.networking.k8s.io/httproutes
    - gateway.networking.k8s.io/listenersets
    - gateway.networking.k8s.io/referencegrants
    - gateway.networking.k8s.io/tcproutes
    - gateway.networking.k8s.io/tlsroutes
    - gateway.networking.k8s.io/udproutes
    - infrastructure.cluster.x-k8s.io/deckhouseclusters
    - infrastructure.cluster.x-k8s.io/deckhousemachines
    - infrastructure.cluster.x-k8s.io/deckhousemachinetemplates
    - infrastructure.cluster.x-k8s.io/dynamixclusters
    - infrastructure.cluster.x-k8s.io/dynamixmachines
    - infrastructure.cluster.x-k8s.io/dynamixmachinetemplates
    - infrastructure.cluster.x-k8s.io/huaweicloudclusters
    - infrastructure.cluster.x-k8s.io/huaweicloudmachines
    - infrastructure.cluster.x-k8s.io/huaweicloudmachinetemplates
    - infrastructure.cluster.x-k8s.io/vcdclusters
    - infrastructure.cluster.x-k8s.io/vcdclustertemplates
    - infrastructure.cluster.x-k8s.io/vcdmachines
    - infrastructure.cluster.x-k8s.io/vcdmachinetemplates
    - infrastructure.cluster.x-k8s.io/zvirtclusters
    - infrastructure.cluster.x-k8s.io/zvirtmachines
    - infrastructure.cluster.x-k8s.io/zvirtmachinetemplates
    - limitranges
    - metrics.k8s.io/nodes
    - metrics.k8s.io/pods
    - multitenancy.deckhouse.io/availableclusterresources
    - mutations.gatekeeper.sh/assign
    - mutations.gatekeeper.sh/assignimage
    - mutations.gatekeeper.sh/assignmetadata
    - mutations.gatekeeper.sh/modifyset
    - namespaces
    - network.deckhouse.io/egressgatewaypolicies
    - network.deckhouse.io/egressgateways
    - network.deckhouse.io/metalloadbalancerclasses
    - network.deckhouse.io/servicewithhealthchecks
    - networking.istio.io/destinationrules
    - networking.istio.io/gateways
    - networking.istio.io/serviceentries
    - networking.istio.io/sidecars
    - networking.istio.io/virtualservices
    - networking.istio.io/workloadentries
    - networking.istio.io/workloadgroups
    - networking.k8s.io/ingresses
    - networking.k8s.io/networkpolicies
    - nodes
    - persistentvolumeclaims
    - persistentvolumes
    - pods
    - pods/log
    - policy/poddisruptionbudgets
    - rbac.authorization.k8s.io/rolebindings
    - rbac.authorization.k8s.io/roles
    - replicationcontrollers
    - resourcequotas
    - security.istio.io/authorizationpolicies
    - security.istio.io/peerauthentications
    - security.istio.io/requestauthentications
    - serviceaccounts
    - services
    - status.gatekeeper.sh/configpodstatuses
    - status.gatekeeper.sh/connectionpodstatuses
    - status.gatekeeper.sh/constraintpodstatuses
    - status.gatekeeper.sh/constrainttemplatepodstatuses
    - status.gatekeeper.sh/expansiontemplatepodstatuses
    - status.gatekeeper.sh/mutatorpodstatuses
    - status.gatekeeper.sh/providerpodstatuses
    - storage.k8s.io/storageclasses
    - syncset.gatekeeper.sh/syncsets
    - telemetry.istio.io/telemetries
    - templates.gatekeeper.sh/constrainttemplates
```

{{site.data.i18n.common.role[page.lang] | capitalize }} `PrivilegedUser` ({{site.data.i18n.common.includes_rules_from[page.lang]}} `User`):

```text
create:
    - pods/eviction
create,get:
    - pods/attach
    - pods/exec
delete,deletecollection:
    - pods
read:
    - secrets
```

{{site.data.i18n.common.role[page.lang] | capitalize }} `Editor` ({{site.data.i18n.common.includes_rules_from[page.lang]}} `User`, `PrivilegedUser`):

```text
read:
    - deckhouse.io/clusterlogdestinations
    - deckhouse.io/clusterloggingconfigs
    - deckhouse.io/customprometheusrules
    - deckhouse.io/grafanaadditionaldatasources
    - deckhouse.io/grafanadashboarddefinitions
read-write:
    - deckhouse.io/podloggingconfigs
write:
    - apps/deployments
    - apps/statefulsets
    - autoscaling.k8s.io/verticalpodautoscalers
    - autoscaling/horizontalpodautoscalers
    - batch/cronjobs
    - batch/jobs
    - cert-manager.io/certificates
    - cert-manager.io/issuers
    - configmaps
    - deckhouse.io/daemonsetmetrics
    - deckhouse.io/deploymentmetrics
    - deckhouse.io/dexauthenticators
    - deckhouse.io/dexclients
    - deckhouse.io/ingressmetrics
    - deckhouse.io/namespacemetrics
    - deckhouse.io/podmetrics
    - deckhouse.io/servicemetrics
    - deckhouse.io/statefulsetmetrics
    - discovery.k8s.io/endpointslices
    - endpoints
    - extensions/deployments
    - extensions/ingresses
    - gateway.networking.k8s.io/backendtlspolicies
    - gateway.networking.k8s.io/gateways
    - gateway.networking.k8s.io/grpcroutes
    - gateway.networking.k8s.io/httproutes
    - gateway.networking.k8s.io/listenersets
    - gateway.networking.k8s.io/referencegrants
    - gateway.networking.k8s.io/tcproutes
    - gateway.networking.k8s.io/tlsroutes
    - gateway.networking.k8s.io/udproutes
    - network.deckhouse.io/servicewithhealthchecks
    - networking.istio.io/destinationrules
    - networking.istio.io/gateways
    - networking.istio.io/serviceentries
    - networking.istio.io/sidecars
    - networking.istio.io/virtualservices
    - networking.istio.io/workloadentries
    - networking.istio.io/workloadgroups
    - networking.k8s.io/ingresses
    - networking.k8s.io/networkpolicies
    - persistentvolumeclaims
    - policy/poddisruptionbudgets
    - secrets
    - security.istio.io/authorizationpolicies
    - security.istio.io/peerauthentications
    - security.istio.io/requestauthentications
    - serviceaccounts
    - services
```

{{site.data.i18n.common.role[page.lang] | capitalize }} `Admin` ({{site.data.i18n.common.includes_rules_from[page.lang]}} `User`, `PrivilegedUser`, `Editor`):

```text
create,patch,update:
    - pods
delete,deletecollection:
    - acme.cert-manager.io/orders
    - apps/replicasets
    - cert-manager.io/certificaterequests
    - cert-manager.io/challenges
    - extensions/replicasets
read:
    - 'deckhouse.io/moduleconfigs (resourceNames: deckhouse)'
read-write:
    - deckhouse.io/authorizationrules
write:
    - autoscaling.k8s.io/verticalpodautoscalercheckpoints
    - deckhouse.io/applicationpackages
    - deckhouse.io/applicationpackageversions
    - deckhouse.io/applications
    - deckhouse.io/deckhousereleases
    - deckhouse.io/moduleconfigs
    - deckhouse.io/moduledocumentations
    - deckhouse.io/modulepulloverrides
    - deckhouse.io/modulereleases
    - deckhouse.io/modules
    - deckhouse.io/modulesources
    - deckhouse.io/moduleupdatepolicies
    - deckhouse.io/packagerepositories
    - deckhouse.io/packagerepositoryoperations
    - deckhouse.io/securitypolicyexceptions
    - extensions.istio.io/wasmplugins
    - rbac.authorization.k8s.io/rolebindings
    - rbac.authorization.k8s.io/roles
    - telemetry.istio.io/telemetries
```

{{site.data.i18n.common.role[page.lang] | capitalize }} `ClusterEditor` ({{site.data.i18n.common.includes_rules_from[page.lang]}} `User`, `PrivilegedUser`, `Editor`):

```text
delete,deletecollection:
    - acme.cert-manager.io/orders
    - cert-manager.io/certificaterequests
    - cert-manager.io/challenges
patch,update:
    - nodes
read:
    - deckhouse.io/ingressistiocontrollers
    - deckhouse.io/ingressnginxcontrollers/status
    - deckhouse.io/istiofederations
    - deckhouse.io/istiomulticlusters
    - 'deckhouse.io/moduleconfigs (resourceNames: deckhouse)'
    - install.istio.io/istiooperators
    - multitenancy.deckhouse.io/grantableclusterresourcedefinitions
    - multitenancy.deckhouse.io/grantableclusterresourcereferences
    - rbac.authorization.k8s.io/clusterrolebindings
    - rbac.authorization.k8s.io/clusterroles
    - sailoperator.io/istiocnis
    - sailoperator.io/istiorevisions
    - sailoperator.io/istiorevisiontags
    - sailoperator.io/istios
    - sailoperator.io/ztunnels
read-write:
    - apps.kruise.io/daemonsets
    - deckhouse.io/downtimes
    - deckhouse.io/ingressnginxcontrollers
    - deckhouse.io/nodegroupconfigurations
    - deckhouse.io/staticinstances
    - deckhouse.io/upmeterremotewrites
    - multitenancy.deckhouse.io/clusterresourcegrantpolicies
write:
    - apiextensions.k8s.io/customresourcedefinitions
    - apps/daemonsets
    - autoscaling.k8s.io/verticalpodautoscalercheckpoints
    - cert-manager.io/clusterissuers
    - deckhouse.io/applicationpackages
    - deckhouse.io/applicationpackageversions
    - deckhouse.io/applications
    - deckhouse.io/clusterdaemonsetmetrics
    - deckhouse.io/clusterdeploymentmetrics
    - deckhouse.io/clusteringressmetrics
    - deckhouse.io/clusterlogdestinations
    - deckhouse.io/clusterloggingconfigs
    - deckhouse.io/clusterpodmetrics
    - deckhouse.io/clusterservicemetrics
    - deckhouse.io/clusterstatefulsetmetrics
    - deckhouse.io/customprometheusrules
    - deckhouse.io/deckhousereleases
    - deckhouse.io/grafanaadditionaldatasources
    - deckhouse.io/grafanadashboarddefinitions
    - deckhouse.io/hubblemonitoringconfigs
    - deckhouse.io/instances
    - deckhouse.io/keepalivedinstances
    - deckhouse.io/moduleconfigs
    - deckhouse.io/moduledocumentations
    - deckhouse.io/modulepulloverrides
    - deckhouse.io/modulereleases
    - deckhouse.io/modules
    - deckhouse.io/modulesources
    - deckhouse.io/moduleupdatepolicies
    - deckhouse.io/nodegroups
    - deckhouse.io/packagerepositories
    - deckhouse.io/packagerepositoryoperations
    - deckhouse.io/securitypolicyexceptions
    - extensions.istio.io/wasmplugins
    - extensions/daemonsets
    - gateway.networking.k8s.io/gatewayclasses
    - network.deckhouse.io/egressgatewaypolicies
    - network.deckhouse.io/egressgateways
    - storage.k8s.io/storageclasses
    - telemetry.istio.io/telemetries
```

{{site.data.i18n.common.role[page.lang] | capitalize }} `ClusterAdmin` ({{site.data.i18n.common.includes_rules_from[page.lang]}} `User`, `PrivilegedUser`, `Editor`, `Admin`, `ClusterEditor`):

```text
delete,deletecollection,get,list,patch,update,watch:
    - machine.sapcloud.io/alicloudmachineclasses
    - machine.sapcloud.io/awsmachineclasses
    - machine.sapcloud.io/azuremachineclasses
    - machine.sapcloud.io/gcpmachineclasses
    - machine.sapcloud.io/machinedeployments
    - machine.sapcloud.io/machines
    - machine.sapcloud.io/machinesets
    - machine.sapcloud.io/openstackmachineclasses
    - machine.sapcloud.io/packetmachineclasses
    - machine.sapcloud.io/vspheremachineclasses
    - machine.sapcloud.io/yandexmachineclasses
get,list,patch,update,watch:
    - control-plane.deckhouse.io/controlplanenodes
list:
    - dex.coreos.com/offlinesessionses
    - dex.coreos.com/passwords
patch,update:
    - deckhouse.io/vcdaffinityrules
    - infrastructure.cluster.x-k8s.io/deckhouseclusters
    - infrastructure.cluster.x-k8s.io/deckhousemachines
    - infrastructure.cluster.x-k8s.io/deckhousemachinetemplates
    - infrastructure.cluster.x-k8s.io/dynamixclusters
    - infrastructure.cluster.x-k8s.io/dynamixmachines
    - infrastructure.cluster.x-k8s.io/dynamixmachinetemplates
    - infrastructure.cluster.x-k8s.io/huaweicloudclusters
    - infrastructure.cluster.x-k8s.io/huaweicloudmachines
    - infrastructure.cluster.x-k8s.io/huaweicloudmachinetemplates
    - infrastructure.cluster.x-k8s.io/vcdclusters
    - infrastructure.cluster.x-k8s.io/vcdclustertemplates
    - infrastructure.cluster.x-k8s.io/vcdmachines
    - infrastructure.cluster.x-k8s.io/vcdmachinetemplates
    - infrastructure.cluster.x-k8s.io/zvirtclusters
    - infrastructure.cluster.x-k8s.io/zvirtmachines
    - infrastructure.cluster.x-k8s.io/zvirtmachinetemplates
    - machine.sapcloud.io/machinedeployments/scale
proxy:
    - nodes
read:
    - cluster.x-k8s.io/machinedrainrules
    - control-plane.deckhouse.io/controlplaneoperations
    - infrastructure.cluster.x-k8s.io/deckhousecontrolplanes
    - infrastructure.cluster.x-k8s.io/staticclusters
    - infrastructure.cluster.x-k8s.io/staticmachines
    - nfd.k8s-sigs.io/nodefeaturegroups
    - nfd.k8s-sigs.io/nodefeaturerules
    - nfd.k8s-sigs.io/nodefeatures
read-write:
    - cluster.x-k8s.io/clusters
    - cluster.x-k8s.io/machinedeployments
    - cluster.x-k8s.io/machinehealthchecks
    - cluster.x-k8s.io/machinepools
    - cluster.x-k8s.io/machines
    - cluster.x-k8s.io/machinesets
    - deckhouse.io/clusterauthorizationrules
    - deckhouse.io/dexproviders
    - deckhouse.io/groups
    - deckhouse.io/nodeusers
    - deckhouse.io/sshcredentials
    - deckhouse.io/useroperations
    - deckhouse.io/users
    - infrastructure.cluster.x-k8s.io/staticmachinetemplates
    - nodes/configz
    - nodes/healthz
    - nodes/log
    - nodes/metrics
    - nodes/pods
    - nodes/proxy
    - nodes/stats
write:
    - cilium.io/ciliumclusterwidenetworkpolicies
    - cilium.io/ciliumnetworkpolicies
    - cluster.x-k8s.io/machinedeployments/scale
    - config.gatekeeper.sh/configs
    - connection.gatekeeper.sh/connections
    - constraints.gatekeeper.sh/*
    - deckhouse.io/awsinstanceclasses
    - deckhouse.io/azureinstanceclasses
    - deckhouse.io/deschedulers
    - deckhouse.io/dvpinstanceclasses
    - deckhouse.io/dynamixinstanceclasses
    - deckhouse.io/gcpinstanceclasses
    - deckhouse.io/huaweicloudinstanceclasses
    - deckhouse.io/ingressistiocontrollers
    - deckhouse.io/istiofederations
    - deckhouse.io/istiomulticlusters
    - deckhouse.io/localpathprovisioners
    - deckhouse.io/openstackinstanceclasses
    - deckhouse.io/operationpolicies
    - deckhouse.io/projects
    - deckhouse.io/projecttemplates
    - deckhouse.io/securitypolicies
    - deckhouse.io/vcdinstanceclasses
    - deckhouse.io/vsphereinstanceclasses
    - deckhouse.io/yandexinstanceclasses
    - deckhouse.io/zvirtinstanceclasses
    - expansion.gatekeeper.sh/expansiontemplate
    - externaldata.gatekeeper.sh/providers
    - install.istio.io/istiooperators
    - limitranges
    - mutations.gatekeeper.sh/assign
    - mutations.gatekeeper.sh/assignimage
    - mutations.gatekeeper.sh/assignmetadata
    - mutations.gatekeeper.sh/modifyset
    - namespaces
    - network.deckhouse.io/metalloadbalancerclasses
    - rbac.authorization.k8s.io/clusterrolebindings
    - rbac.authorization.k8s.io/clusterroles
    - resourcequotas
    - sailoperator.io/istiocnis
    - sailoperator.io/istiorevisions
    - sailoperator.io/istiorevisiontags
    - sailoperator.io/istios
    - sailoperator.io/ztunnels
    - status.gatekeeper.sh/configpodstatuses
    - status.gatekeeper.sh/connectionpodstatuses
    - status.gatekeeper.sh/constraintpodstatuses
    - status.gatekeeper.sh/constrainttemplatepodstatuses
    - status.gatekeeper.sh/expansiontemplatepodstatuses
    - status.gatekeeper.sh/mutatorpodstatuses
    - status.gatekeeper.sh/providerpodstatuses
    - syncset.gatekeeper.sh/syncsets
    - templates.gatekeeper.sh/constrainttemplates
```
<!-- end user-authz roles placeholder -->

{: #get_rules .anchored}

You can get additional list of access rules for module role from cluster ([existing user defined rules](usage.html#customizing-rights-of-high-level-roles) and non-default rules from other deckhouse modules):

```bash
D8_ROLE_NAME=Editor
kubectl get clusterrole -A -o jsonpath="{range .items[?(@.metadata.annotations.user-authz\.deckhouse\.io/access-level=='$D8_ROLE_NAME')]}{.rules}{'\n'}{end}" | jq -s add
```
