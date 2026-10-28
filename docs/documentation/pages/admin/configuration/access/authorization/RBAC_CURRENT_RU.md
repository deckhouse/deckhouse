---
title: "Упрощённая модель авторизации"
permalink: ru/admin/configuration/access/authorization/rbac-current.html
description: "Настройка упрощённой модели RBAC-авторизации в Deckhouse Platform. Настройка модуля user-authz, управление ClusterRole и конфигурация ролевого доступа."
lang: ru
---

Для реализации упрощённой ролевой модели в кластере должен быть включён модуль [`user-authz`](/modules/user-authz/).
Модуль создаёт набор кластерных ролей (ClusterRole), подходящий для большинства задач по управлению доступом пользователей и групп.

{% alert level="warning" %}
В модуле реализованы две ролевые модели: [гранулярная](rbac-experimental.html) (рекомендуется к использованию) и упрощённая (эта страница), построенная на ресурсах ClusterAuthorizationRule и AuthorizationRule — её поддержка будет прекращена в будущих релизах.

Модели не совместимы по ресурсам — автоматическая конвертация невозможна, — но могут использоваться одновременно: права из обеих моделей суммируются.
{% endalert %}

Особенности упрощённой ролевой модели:

- Реализует role-based-подсистему сквозной авторизации, расширяя функционал стандартного механизма RBAC.
- Права доступа настраиваются кастомными ресурсами [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule) и [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule) поверх RBAC Kubernetes.
- Управление доступом к инструментам масштабирования (параметр `allowScale` ресурса [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule-v1-spec-allowscale) или [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule-v1alpha1-spec-allowscale)).
- Управление доступом к форвардингу портов (параметр `portForwarding` ресурса [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule-v1-spec-portforwarding) или [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule-v1alpha1-spec-portforwarding)).
- Управление списком разрешённых неймспейсов в формате labelSelector (параметр `namespaceSelector` ресурса [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule-v1-spec-namespaceselector)).
- Ограничение по неймспейсам (режим мультитенантности, `namespaceSelector`, `limitNamespaces`) доступно в редакциях DP Ultimate/EE, Certified Lite, Certified Pro, BE, SE, SE+ и CSE.

## Высокоуровневые роли, используемые для реализации модели

Для реализации упрощённой ролевой модели с помощью модуля [`user-authz`](/modules/user-authz/), кроме использования RBAC, можно использовать удобный набор высокоуровневых ролей:

| Роль             | Примеры доступных действий                                                                                                              | Ограничения                                  |
|------------------|-----------------------------------------------------------------------------------------------------------------------------------------|---------------------------------------------|
| **User**         | Просмотр подов, логов, Deployment                                                                                                   | Нет доступа к секретам, портам, контейнерам |
| **PrivilegedUser** | Вход в контейнеры (`kubectl exec`), чтение секретов, удаление подов                                                                     | Не может изменять Deployment/Service        |
| **Editor**       | Создание/удаление Deployment, Service, ConfigMap                                                                                        | Нет доступа к ReplicaSet, ClusterRoles  |
| **Admin**        | Создание и изменение подов, выпуск токенов ServiceAccount'ов (`serviceaccounts/token`), управление объектами Role, RoleBinding и AuthorizationRule, удаление служебных объектов (например, ReplicaSet, challenges и orders группы `acme.cert-manager.io`) | Нет доступа к ресурсам на уровне кластера         |
| **ClusterEditor** | Управление ограниченным набором cluster-wide-объектов для прикладных задач: DaemonSet, StorageClass, CustomResourceDefinition, GatewayClass и объекты, которые модули добавляют на этот уровень (например, KeepalivedInstance) | Не может управлять неймспейсами, квотами, ClusterRole (доступно только чтение) и ClusterRoleBinding |
| **ClusterAdmin** | Управление служебными cluster-wide-объектами: неймспейсы, квоты, NetworkPolicy, подресурсы узлов для proxy, ClusterAuthorizationRule, ClusterRole и ClusterRoleBinding | Может создавать объекты ClusterAuthorizationRule с уровнем не выше `ClusterAdmin` и объекты ClusterRoleBinding только на роли, права которых у него уже есть |
| **SuperAdmin**   | Любые действия (включая `*` в RBAC), но с учётом `limitNamespaces`                                                                      | Ограничения только через политики кластера  |

{% alert level="warning" %}
Режим мультитенантности ограничивает доступ только к ресурсам в неймспейсах. Субъекты с уровнем доступа `ClusterAdmin` или `SuperAdmin` по-прежнему могут создавать кластерные биндинги. Вебхук авторизации не проверяет компоненты control plane, kubelet и ServiceAccount'ы неймспейса `kube-system` и неймспейсов, имена которых начинаются с `d8-`, поэтому ограничения по неймспейсам к ним не применяются.
{% endalert %}

Параметры `limitNamespaces` и `allowAccessToSystemNamespaces` ресурса [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule) устарели. Чтобы ограничить правило определёнными неймспейсами, используйте [`namespaceSelector`](/modules/user-authz/cr.html#clusterauthorizationrule-v1-spec-namespaceselector). Чтобы выдать доступ ко всем неймспейсам, включая системные, используйте [`namespaceSelector.matchAny: true`](/modules/user-authz/cr.html#clusterauthorizationrule-v1-spec-namespaceselector-matchany). Если задан `namespaceSelector`, устаревшие параметры не учитываются.

В режиме мультитенантности, если вебхук авторизации станет недоступен, запросы будут **запрещаться**, а не разрешаться: вебхук стоит перед RBAC и настроен с `failurePolicy: Deny`, поэтому запрос, на который он не смог ответить, до RBAC не доходит. Исключение — идентичности, перечисленные в `matchConditions` ресурса AuthorizationConfiguration: компоненты control plane, `kubernetes-api-proxy`, kubelet, ServiceAccount'ы неймспейсов `kube-system` и `d8-*`, а также анонимные проверки работоспособности kube-apiserver (`/livez`, `/readyz`, `/healthz`) вебхук не проверяет, поэтому кластер продолжает работать, а Deckhouse может восстановить вебхук.

Некоторые компоненты платформы размещают свои объекты в пользовательских неймспейсах (например, поды Dex-аутентификатора) и помечают их лейблом `deckhouse.io/system-resource: "true"`. Из уровней упрощённой модели изменять и удалять такие объекты, а также выполнять `exec`, `attach` и `port-forward` в такие поды позволяет только `SuperAdmin`: субъекты правила с этим уровнем получают роль `user-authz:super-admin` и считаются администраторами кластера. Подробнее — в разделе [«Ограничения уровня admin и права superadmin»](rbac-experimental.html#ограничения-уровня-admin-и-права-superadmin).

## Список доступа для каждой высокоуровневой роли по умолчанию

{% alert level="info" %}
Этот список — статический снимок: фактический набор правил зависит от включённых в кластере модулей DP.
{% endalert %}

Сокращения для `verbs`:

- read — `get`, `list`, `watch`;
- read-write — `get`, `list`, `watch`, `create`, `delete`, `deletecollection`, `patch`, `update`;
- write — `create`, `delete`, `deletecollection`, `patch`, `update`.

{{site.data.i18n.common.role[page.lang] | capitalize }} `User`:

```text
read:
    - acme.cert-manager.io/challenges
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
    - deckhouse.io/clusterprojectrolebindings
    - deckhouse.io/deschedulers
    - deckhouse.io/dexauthenticators
    - deckhouse.io/dexclients
    - deckhouse.io/dvpinstanceclasses
    - deckhouse.io/dynamixinstanceclasses
    - deckhouse.io/gcpinstanceclasses
    - deckhouse.io/huaweicloudinstanceclasses
    - deckhouse.io/hubblemonitoringconfigs
    - deckhouse.io/instances
    - deckhouse.io/keepalivedinstances
    - deckhouse.io/localpathprovisioners
    - deckhouse.io/nodeextensionrequests
    - deckhouse.io/nodegroups
    - deckhouse.io/nodeoperations
    - deckhouse.io/openstackinstanceclasses
    - deckhouse.io/operationpolicies
    - deckhouse.io/projectnamespaces
    - deckhouse.io/projectrolebindings
    - deckhouse.io/projecttemplates
    - deckhouse.io/securitypolicies
    - deckhouse.io/securitypolicyexceptions
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
    - network.deckhouse.io/metalloadbalancerbgppeers
    - network.deckhouse.io/metalloadbalancerclasses
    - network.deckhouse.io/metalloadbalancerconfigurations
    - network.deckhouse.io/metalloadbalancerpools
    - network.deckhouse.io/servicewithhealthchecks
    - network.deckhouse.io/waypointinstances
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
get,patch:
    - pods/resize
write:
    - apps/deployments
    - apps/statefulsets
    - autoscaling.k8s.io/verticalpodautoscalers
    - autoscaling/horizontalpodautoscalers
    - batch/cronjobs
    - batch/jobs
    - cert-manager.io/certificates
    - configmaps
    - deckhouse.io/dexauthenticators
    - deckhouse.io/dexclients
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
    - network.deckhouse.io/waypointinstances
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
create:
    - serviceaccounts/token
create,patch,update:
    - pods
delete,deletecollection:
    - acme.cert-manager.io/challenges
    - acme.cert-manager.io/orders
    - apps/replicasets
    - cert-manager.io/certificaterequests
    - extensions/replicasets
read-write:
    - deckhouse.io/authorizationrules
write:
    - autoscaling.k8s.io/verticalpodautoscalercheckpoints
    - cert-manager.io/issuers
    - deckhouse.io/applications
    - extensions.istio.io/wasmplugins
    - rbac.authorization.k8s.io/rolebindings
    - rbac.authorization.k8s.io/roles
    - telemetry.istio.io/telemetries
```

{{site.data.i18n.common.role[page.lang] | capitalize }} `ClusterEditor` ({{site.data.i18n.common.includes_rules_from[page.lang]}} `User`, `PrivilegedUser`, `Editor`):

```text
create,delete,patch:
    - deckhouse.io/nodeoperations
delete,deletecollection:
    - acme.cert-manager.io/challenges
    - acme.cert-manager.io/orders
    - cert-manager.io/certificaterequests
get,list:
    - templates.internal.deckhouse.io/nodeconfigtemplates
patch,update:
    - nodes
read:
    - deckhouse.io/containerdintegritypolicies
    - deckhouse.io/ingressistiocontrollers
    - deckhouse.io/istiofederations
    - deckhouse.io/istiomulticlusters
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
    - deckhouse.io/nodegroupconfigurations
    - deckhouse.io/staticinstances
    - multitenancy.deckhouse.io/clusterresourcegrantpolicies
write:
    - apiextensions.k8s.io/customresourcedefinitions
    - apps/daemonsets
    - autoscaling.k8s.io/verticalpodautoscalercheckpoints
    - cert-manager.io/clusterissuers
    - cert-manager.io/issuers
    - deckhouse.io/applications
    - deckhouse.io/hubblemonitoringconfigs
    - deckhouse.io/instances
    - deckhouse.io/keepalivedinstances
    - deckhouse.io/nodeextensionrequests
    - deckhouse.io/nodegroups
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
create:
    - authorization.deckhouse.io/bulksubjectaccessreviews
    - authorization.deckhouse.io/roleaccessreports
    - authorization.deckhouse.io/subjectaccessreports
    - deckhouse.io/dexauthenticators/allow-access-to-kubernetes
    - deckhouse.io/dexclients/allow-access-to-kubernetes
get,list,patch,update,watch:
    - control-plane.deckhouse.io/controlplanenodes
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
proxy:
    - nodes
read:
    - control-plane.deckhouse.io/controlplaneoperations
    - deckhouse.io/nodestaticpodrequests
    - nfd.k8s-sigs.io/nodefeaturegroups
    - nfd.k8s-sigs.io/nodefeaturerules
    - nfd.k8s-sigs.io/nodefeatures
read-write:
    - deckhouse.io/clusterauthorizationrules
    - deckhouse.io/deckhousereleases
    - deckhouse.io/dexproviderchecks
    - deckhouse.io/dexproviders
    - deckhouse.io/groups
    - deckhouse.io/moduleconfigs
    - deckhouse.io/moduledocumentations
    - deckhouse.io/modulepulloverrides
    - deckhouse.io/modulereleases
    - deckhouse.io/modules
    - deckhouse.io/modulesources
    - deckhouse.io/moduleupdatepolicies
    - deckhouse.io/nodeusers
    - deckhouse.io/packagerepositories
    - deckhouse.io/packagerepositoryoperations
    - deckhouse.io/sshcredentials
    - deckhouse.io/useraccounts
    - deckhouse.io/useroperations
    - deckhouse.io/users
    - nodes/configz
    - nodes/healthz
    - nodes/log
    - nodes/metrics
    - nodes/pods
    - nodes/proxy
    - nodes/stats
update:
    - namespaces/finalize
write:
    - cilium.io/ciliumclusterwidenetworkpolicies
    - cilium.io/ciliumnetworkpolicies
    - config.gatekeeper.sh/configs
    - connection.gatekeeper.sh/connections
    - constraints.gatekeeper.sh/*
    - deckhouse.io/applicationpackages
    - deckhouse.io/applicationpackageversions
    - deckhouse.io/awsinstanceclasses
    - deckhouse.io/azureinstanceclasses
    - deckhouse.io/clusterprojectrolebindings
    - deckhouse.io/containerdintegritypolicies
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
    - deckhouse.io/projectnamespaces
    - deckhouse.io/projectrolebindings
    - deckhouse.io/projects
    - deckhouse.io/projecttemplates
    - deckhouse.io/securitypolicies
    - deckhouse.io/securitypolicyexceptions
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
    - network.deckhouse.io/metalloadbalancerbgppeers
    - network.deckhouse.io/metalloadbalancerclasses
    - network.deckhouse.io/metalloadbalancerconfigurations
    - network.deckhouse.io/metalloadbalancerpools
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

Чтобы получить дополнительный список правил доступа для роли модуля из кластера ([правила, добавленные администратором](#расширение-прав-доступа-для-высокоуровневых-ролей), и нестандартные правила других модулей Deckhouse Platform), выполните команду:

```bash
D8_ROLE_NAME=Editor
d8 k get clusterrole -o jsonpath="{range .items[?(@.metadata.annotations.user-authz\.deckhouse\.io/access-level=='$D8_ROLE_NAME')]}{.rules}{'\n'}{end}" | jq -s add
```

## Пример AuthorizationRule

Используйте [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule) для установки правил доступа для пользователей внутри определённого неймспейса.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: AuthorizationRule
metadata:
  name: test-rule
spec:
  accessLevel: Admin
  subjects:
  - kind: User
    name: admin@example.com
```

## Пример ClusterAuthorizationRule

[ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule) можно использовать для установки правил доступа для пользователей как на уровне всего кластера, так и на уровне определённых неймспейсов.

```yaml
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: test-rule
spec:
  subjects:
  - kind: User
    name: some@example.com
  - kind: ServiceAccount
    name: gitlab-runner-deploy
    namespace: ci
  - kind: Group
    name: some-group-name
  accessLevel: PrivilegedUser
  portForwarding: true
  # Опция доступна только при включённом параметре enableMultiTenancy.
  namespaceSelector:
    labelSelector:
      matchExpressions:
      - key: stage
        operator: In
        values:
        - test
        - review
      matchLabels:
        team: frontend
```

## Применение правил

Объекты ClusterAuthorizationRule и AuthorizationRule выдают доступ через обычные объекты ClusterRoleBinding и RoleBinding. Компонент `user-authz-controller` создаёт эти привязки для каждого правила, синхронизирует их с правилом и записывает результат в `status` правила. Чтобы посмотреть состояние правил, выполните:

```shell
d8 k get clusterauthorizationrules
d8 k get authorizationrules -A
```

Колонка `READY` показывает, соответствуют ли привязки правилу, колонка `BINDINGS` — сколько привязок у правила. Правило с неподдерживаемым `accessLevel` получает `Ready=False` с причиной `InvalidSpec`. Остальные причины и метрики контроллера описаны в [FAQ модуля `user-authz`](/modules/user-authz/faq.html#как-проверить-что-контроллер-поддерживает-биндинги-в-актуальном-состоянии).

Если одному субъекту соответствует несколько объектов ClusterAuthorizationRule, разрешённые неймспейсы объединяются, а эффективным `accessLevel` становится самый высокий из уровней подходящих правил. Пример — в [FAQ модуля `user-authz`](/modules/user-authz/faq.html#что-если-два-clusterauthorizationrules-подходят-для-одного-пользователя).

Создание, изменение и удаление ClusterAuthorizationRule считаются назначением ролей: запрос допускается, только если запрашивающий может назначить роли этого правила. Изменение проверяется и по прежним, и по новым правам, поэтому, чтобы понизить `SuperAdmin` до `User`, нужно иметь право назначать `SuperAdmin`. Кто какие уровни может назначать, описано в разделе [«Кто может назначать роли»](rbac-experimental.html#кто-может-назначать-роли).

Субъект типа `User` или `Group` сопоставляется по имени с идентичностью из токена, а локальная идентичность не отличается от полученной от внешнего провайдера аутентификации. Поэтому создание [User](/modules/user-authn/cr.html#user) со значением `spec.email` или [Group](/modules/user-authn/cr.html#group) со значением `spec.name`, совпадающим с субъектом существующего ClusterAuthorizationRule или AuthorizationRule, запрещено по умолчанию. Ограничение проверяется при создании User или Group и при изменении их `spec.email` или `spec.name`. Если совпадение намеренное, например, правило написано заранее, установите на User или Group аннотацию `user-authz.deckhouse.io/allow-authorization-rule-collision: "true"`. Аннотация только подтверждает совпадение имени и не позволяет запрашивающему назначить роли, которые он не может назначить иначе.

## Расширение прав доступа для высокоуровневых ролей

Если требуется добавить права для определённой [высокоуровневой роли](../authorization/rbac-current.html#высокоуровневые-роли-используемые-для-реализации-модели), создайте ClusterRole с аннотацией `user-authz.deckhouse.io/access-level: <AccessLevel>`.

Аннотация принимает значения `User`, `PrivilegedUser`, `Editor`, `Admin`, `ClusterEditor` и `ClusterAdmin`. Другие значения, включая `SuperAdmin`, игнорируются. В имени агрегированной роли уровень записывается через дефис, например `user-authz:cluster-editor:custom`. Правила уровня также входят в агрегированные роли всех более высоких уровней.

Пример:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  annotations:
    user-authz.deckhouse.io/access-level: Editor
  name: user-editor
rules:
- apiGroups:
  - kuma.io
  resources:
  - trafficroutes
  - trafficroutes/finalizers
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
- apiGroups:
  - flagger.app
  resources:
  - canaries
  - canaries/status
  - metrictemplates
  - metrictemplates/status
  - alertproviders
  - alertproviders/status
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
```
