---
title: "Модуль operator-trivy: Custom Resources (от aquasecurity.github.io)"
---

## Безопасность ресурсов кластера

### VulnerabilityReport

[Reference](https://aquasecurity.github.io/trivy-operator/v0.22.0/docs/crds/vulnerability-report/)

`VulnerabilityReport` представляет собой список последних уязвимостей, обнаруженных в образе контейнера заданной рабочей нагрузки Kubernetes. 
Он состоит из списка уязвимостей пакетов ОС и приложений с краткой сводкой уязвимостей, сгруппированных по уровню серьёзности. Для многоконтейнерной рабочей нагрузки trivy-operator создаёт несколько экземпляров VulnerabilityReport в пространстве имён рабочей нагрузки, при этом ссылка владельца указывает на эту рабочую нагрузку.   
Каждый отчёт следует соглашению об именовании: `<вид рабочей нагрузки>-<имя рабочей нагрузки>-<имя контейнера>`

Пример:
```yaml
apiVersion: aquasecurity.github.io/v1alpha1
kind: VulnerabilityReport
metadata:
  name: replicaset-nginx-6d4cf56db6-nginx
  namespace: default
  labels:
    trivy-operator.container.name: nginx
    trivy-operator.resource.kind: ReplicaSet
    trivy-operator.resource.name: nginx-6d4cf56db6
    trivy-operator.resource.namespace: default
    resource-spec-hash: 7cb64cb677
  uid: 8aa1a7cb-a319-4b93-850d-5a67827dfbbf
  ownerReferences:
    - apiVersion: apps/v1
      blockOwnerDeletion: false
      controller: true
      kind: ReplicaSet
      name: nginx-6d4cf56db6
      uid: aa345200-cf24-443a-8f11-ddb438ff8659
report:
  artifact:
    repository: library/nginx
    tag: '1.16'
  os:
    family: debian
    name:   '10.3'
  registry:
    server: index.docker.io
  scanner:
    name: Trivy
    vendor: Aqua Security
    version: 0.35.0
  summary:
    criticalCount: 2
    highCount: 0
    lowCount: 0
    mediumCount: 0
    unknownCount: 0
  vulnerabilities:
    - fixedVersion: 0.9.1-2+deb10u1
      installedVersion: 0.9.1-2
      links: []
      primaryLink: 'https://avd.aquasec.com/nvd/cve-2019-20367'
      resource: libbsd0
      score: 9.1
      severity: CRITICAL
      target: library/nginx:1.21.6
      title: ''
      vulnerabilityID: CVE-2019-20367
    - fixedVersion: ''
      installedVersion: 0.6.1-2
      links: []
      primaryLink: 'https://avd.aquasec.com/nvd/cve-2018-25009'
      resource: libwebp6
      score: 9.1
      severity: CRITICAL
      target: library/nginx:1.16
      title: 'libwebp: out-of-bounds read in WebPMuxCreateInternal'
      vulnerabilityID: CVE-2018-25009
```
### ConfigAuditReport

[Reference](https://aquasecurity.github.io/trivy-operator/v0.22.0/docs/crds/configaudit-report/)

`ConfigAuditReport` представляет собой проверку конфигурации объекта Kubernetes, выполняемую инструментами аудита конфигурации, такими как Trivy.  
Примерами проверок являются настройка образа контейнера для запуска от имени пользователя без прав root или проверка наличия у контейнера запросов на ресурсы и ограничений. Проверки могут относиться к рабочим нагрузкам Kubernetes и другим объектам Kubernetes в пространстве имён, таким как Services, ConfigMaps, Roles и RoleBindings.

Каждый отчёт принадлежит базовому объекту Kubernetes и хранится в том же пространстве имён, следуя соглашению об именовании `<вид_рабочей_нагрузки>-<имя_рабочей_нагрузки>`.

Пример:
```yaml
apiVersion: aquasecurity.github.io/v1alpha1
kind: ConfigAuditReport
metadata:
  name: replicaset-nginx-6d4cf56db6
  namespace: default
  labels:
    trivy-operator.resource.kind: ReplicaSet
    trivy-operator.resource.name: nginx-6d4cf56db6
    trivy-operator.resource.namespace: default
    plugin-config-hash: 7f65d98b75
    resource-spec-hash: 7cb64cb677
  uid: d5cf8847-c96d-4534-beb9-514a34230302
  ownerReferences:
    - apiVersion: apps/v1
      blockOwnerDeletion: false
      controller: true
      kind: ReplicaSet
      name: nginx-6d4cf56db6
      uid: aa345200-cf24-443a-8f11-ddb438ff8659
report:
  updateTimestamp: '2021-05-20T12:38:10Z'
  scanner:
    name: Trivy 
    vendor: Aqua Security
    version: '0.22.0'
  summary:
    criticalCount: 2
    highCount: 0
    lowCount: 9
    mediumCount: 0
  checks:
    - category: Security
      checkID: hostPIDSet
      messages:
        - Host PID is not configured
      severity: CRITICAL
      success: true
    - category: Security
      checkID: hostIPCSet
      messages:
        - Host IPC is not configured
      severity: CRITICAL
      success: true
    - category: Security
      checkID: hostNetworkSet
      messages:
        - Host network is not configured
      severity: LOW
      success: true
    - category: Security
      checkID: notReadOnlyRootFilesystem
      messages:
        - Filesystem should be read only
      scope:
        type: Container
        value: nginx
      severity: LOW
      success: false
    - category: Security
      checkID: privilegeEscalationAllowed
      messages:
        - Privilege escalation should not be allowed
      scope:
        type: Container
        value: nginx
      severity: CRITICAL
      success: false
```
### ExposedSecretReport

[Reference](https://aquasecurity.github.io/trivy-operator/v0.22.0/docs/crds/exposedsecret-report/)  

`ExposedSecretReport` представляет секреты, найденные в образе контейнера заданной рабочей нагрузки Kubernetes.  
Он состоит из списка раскрытых секретов с краткой информацией, сгруппированной по уровню важности.  
Для многоконтейнерной рабочей нагрузки оператор Trivy создаст несколько экземпляров `ExposedSecretsReports` в пространстве имен рабочей нагрузки со ссылкой на владельца, заданной на эту рабочую нагрузку. Каждый отчет следует следующему соглашению об именовании: `<вид рабочей нагрузки>-<имя рабочей нагрузки>-<имя контейнера>`.

Пример: 
```yaml
apiVersion: aquasecurity.github.io/v1alpha1
kind: ExposedSecretReport
metadata:
  creationTimestamp: "2022-06-29T14:25:54Z"
  generation: 2
  labels:
    resource-spec-hash: 8495697ff5
    trivy-operator.container.name: app
    trivy-operator.resource.kind: ReplicaSet
    trivy-operator.resource.name: app-67b77f5965
    trivy-operator.resource.namespace: default
  name: replicaset-app-67b77f5965-app
  namespace: default
  ownerReferences:
  - apiVersion: apps/v1
    blockOwnerDeletion: false
    controller: true
    kind: ReplicaSet
    name: app-67b77f5965
    uid: 04a744fe-1126-42d5-bb8b-0917bdb51a28
  resourceVersion: "1420"
  uid: 2b2697bb-d528-4d4d-8312-a74dcab6ac65
report:
  artifact:
    repository: myimagewithsecret
    tag: v0.22.0
  registry:
    server: index.docker.io
  scanner:
    name: Trivy
    vendor: Aqua Security
    version: 0.35.0
  secrets:
  - category: Stripe
    match: 'publishable_key: *****'
    ruleID: stripe-access-token
    severity: HIGH
    target: "/app/config/secret.yaml"
    title: Stripe
  - category: Stripe
    match: 'secret_key: *****'
    ruleID: stripe-access-token
    severity: HIGH
    target: "/app/config/secret.yaml"
    title: Stripe
  summary:
    criticalCount: 0
    highCount: 2
    lowCount: 0
    mediumCount: 0
  updateTimestamp: "2022-06-29T14:29:37Z"
```

### SbomReport

[Reference](https://aquasecurity.github.io/trivy-operator/v0.22.0/docs/crds/sbom-report/)

Экземпляр SbomReport представляет собой последний SBOM (Software Bill of Materials), найденный в образе контейнера заданной рабочей нагрузки Kubernetes.  
Он состоит из списка пакетов ОС и спецификаций приложений со сводкой компонентов и зависимостей. Для многоконтейнерной рабочей нагрузки trivy-operator создает несколько экземпляров SbomReports в пространстве имен рабочей нагрузки, при этом ссылка владельца указывает на эту рабочую нагрузку.  
Каждый отчет следует соглашению об именовании: `<вид рабочей нагрузки>-<имя рабочей нагрузки>-<имя контейнера>.`

Пример:
```yaml
apiVersion: aquasecurity.github.io/v1alpha1
kind: SbomReport
metadata:
  creationTimestamp: "2023-07-10T09:37:21Z"
  generation: 1
  labels:
    resource-spec-hash: 796669cd5d
    trivy-operator.container.name: kube-apiserver
    trivy-operator.resource.kind: Pod
    trivy-operator.resource.name: kube-apiserver-kind-control-plane
    trivy-operator.resource.namespace: kube-system
  name: pod-kube-apiserver-kind-control-plane-kube-apiserver
  namespace: kube-system
  ownerReferences:
  - apiVersion: v1
    blockOwnerDeletion: false
    controller: true
    kind: Pod
    name: kube-apiserver-kind-control-plane
    uid: 732b4aa7-91f8-40a3-8b21-9627a98a910b
  resourceVersion: "6148"
  uid: 2a5000fe-b97e-46d0-9de7-62fb5fbc6555
report:
  artifact:
    repository: kube-apiserver
    tag: v1.21.1
  components:
    bomFormat: CycloneDX
    components:
    - bom-ref: 9464f5f9-750d-4ea0-8705-c8d067b25b29
      name: debian
      properties:
      - name: aquasecurity:trivy:Class
        value: os-pkgs
      - name: aquasecurity:trivy:Type
        value: debian
      supplier: {}
      type: operating-system
      version: "10.9"
    - bom-ref: pkg:deb/debian/base-files@10.3+deb10u9?arch=amd64&distro=debian-10.9
      licenses:
      - expression: GPL-3.0
        license: {}
      name: base-files
      properties:
      - name: aquasecurity:trivy:LayerDiffID
        value: sha256:417cb9b79adeec55f58b890dc9831e252e3523d8de5fd28b4ee2abb151b7dc8b
      - name: aquasecurity:trivy:LayerDigest
        value: sha256:5dea5ec2316d4a067b946b15c3c4f140b4f2ad607e73e9bc41b673ee5ebb99a3
      - name: aquasecurity:trivy:PkgID
        value: base-files@10.3+deb10u9
      - name: aquasecurity:trivy:PkgType
        value: debian
      - name: aquasecurity:trivy:SrcName
        value: base-files
      - name: aquasecurity:trivy:SrcVersion
        value: 10.3+deb10u9
      purl: pkg:deb/debian/base-files@10.3+deb10u9?arch=amd64&distro=debian-10.9
      supplier:
        name: Santiago Vila <sanvila@debian.org>
      type: library
      version: 10.3+deb10u9
    - bom-ref: pkg:deb/debian/netbase@5.6?arch=all&distro=debian-10.9
      licenses:
      - expression: GPL-2.0
        license: {}
      name: netbase
      properties:
      - name: aquasecurity:trivy:LayerDiffID
        value: sha256:417cb9b79adeec55f58b890dc9831e252e3523d8de5fd28b4ee2abb151b7dc8b
      - name: aquasecurity:trivy:LayerDigest
        value: sha256:5dea5ec2316d4a067b946b15c3c4f140b4f2ad607e73e9bc41b673ee5ebb99a3
      - name: aquasecurity:trivy:PkgID
        value: netbase@5.6
      - name: aquasecurity:trivy:PkgType
        value: debian
      - name: aquasecurity:trivy:SrcName
        value: netbase
      - name: aquasecurity:trivy:SrcVersion
        value: "5.6"
      purl: pkg:deb/debian/netbase@5.6?arch=all&distro=debian-10.9
      supplier:
        name: Marco d'Itri <md@linux.it>
      type: library
      version: "5.6"
    - bom-ref: pkg:deb/debian/tzdata@2021a-0+deb10u1?arch=all&distro=debian-10.9
      name: tzdata
      properties:
      - name: aquasecurity:trivy:LayerDiffID
        value: sha256:417cb9b79adeec55f58b890dc9831e252e3523d8de5fd28b4ee2abb151b7dc8b
      - name: aquasecurity:trivy:LayerDigest
        value: sha256:5dea5ec2316d4a067b946b15c3c4f140b4f2ad607e73e9bc41b673ee5ebb99a3
      - name: aquasecurity:trivy:PkgID
        value: tzdata@2021a-0+deb10u1
      - name: aquasecurity:trivy:PkgType
        value: debian
      - name: aquasecurity:trivy:SrcName
        value: tzdata
      - name: aquasecurity:trivy:SrcRelease
        value: 0+deb10u1
      - name: aquasecurity:trivy:SrcVersion
        value: 2021a
      purl: pkg:deb/debian/tzdata@2021a-0+deb10u1?arch=all&distro=debian-10.9
      supplier:
        name: GNU Libc Maintainers <debian-glibc@lists.debian.org>
      type: library
      version: 2021a-0+deb10u1
    dependencies:
    - dependsOn:
      - pkg:deb/debian/base-files@10.3+deb10u9?arch=amd64&distro=debian-10.9
      - pkg:deb/debian/netbase@5.6?arch=all&distro=debian-10.9
      - pkg:deb/debian/tzdata@2021a-0+deb10u1?arch=all&distro=debian-10.9
      ref: 9464f5f9-750d-4ea0-8705-c8d067b25b29
    - dependsOn: []
      ref: pkg:deb/debian/base-files@10.3+deb10u9?arch=amd64&distro=debian-10.9
    - dependsOn: []
      ref: pkg:deb/debian/netbase@5.6?arch=all&distro=debian-10.9
    - dependsOn: []
      ref: pkg:deb/debian/tzdata@2021a-0+deb10u1?arch=all&distro=debian-10.9
    - dependsOn:
      - 9464f5f9-750d-4ea0-8705-c8d067b25b29
      ref: pkg:oci/kube-apiserver@sha256:53a13cd1588391888c5a8ac4cef13d3ee6d229cd904038936731af7131d193a9?repository_url=k8s.gcr.io%2Fkube-apiserver&arch=amd64
    metadata:
      component:
        bom-ref: pkg:oci/kube-apiserver@sha256:53a13cd1588391888c5a8ac4cef13d3ee6d229cd904038936731af7131d193a9?repository_url=k8s.gcr.io%2Fkube-apiserver&arch=amd64
        name: k8s.gcr.io/kube-apiserver:v1.21.1
        properties:
        - name: aquasecurity:trivy:DiffID
          value: sha256:417cb9b79adeec55f58b890dc9831e252e3523d8de5fd28b4ee2abb151b7dc8b,sha256:b50131762317bbe47def2d426d5c78a353a08b966d36bed4a04aee99dde4e12b,sha256:1e6ed7621dee7e03dd779486ed469a65af6fb13071d13bd3a89c079683e3b1f0
        - name: aquasecurity:trivy:ImageID
          value: sha256:771ffcf9ca634e37cbd3202fd86bd7e2df48ecba4067d1992541bfa00e88a9bb
        - name: aquasecurity:trivy:RepoDigest
          value: k8s.gcr.io/kube-apiserver@sha256:53a13cd1588391888c5a8ac4cef13d3ee6d229cd904038936731af7131d193a9
        - name: aquasecurity:trivy:RepoTag
          value: k8s.gcr.io/kube-apiserver:v1.21.1
        - name: aquasecurity:trivy:SchemaVersion
          value: "2"
        purl: pkg:oci/kube-apiserver@sha256:53a13cd1588391888c5a8ac4cef13d3ee6d229cd904038936731af7131d193a9?repository_url=k8s.gcr.io%2Fkube-apiserver&arch=amd64
        supplier: {}
        type: container
      timestamp: "2023-07-10T09:37:21+00:00"
      tools:
      - name: trivy
        vendor: aquasecurity
    serialNumber: urn:uuid:50dbce86-28c5-4caf-9d08-a4aadf23233e
    specVersion: 1.4
    version: 1
  registry:
    server: k8s.gcr.io
  scanner:
    name: Trivy
    vendor: Aqua Security
    version: 0.52.2
  summary:
    componentsCount: 5
    dependenciesCount: 5
  updateTimestamp: "2023-07-10T09:37:21Z
```
## Безопасность кластера

### RbacAssessmentReport

[Reference](https://aquasecurity.github.io/trivy-operator/v0.22.0/docs/crds/rbacassessment-report/)

`RbacAssessmentReport` представляет проверки, выполненные инструментами аудита конфигурации по оценке Kubernetes RBAC. Например, проверяет, что заданная роль не предоставляет доступ к секретам всхе групп

Каждый отчёт принадлежит базовому объекту Kubernetes и хранится в том же пространстве имён, следуя соглашению об именовании `<Role>-<role-name>`.

```yaml
apiVersion: aquasecurity.github.io/v1alpha1
kind: RbacAssessmentReport
metadata:
  name: role-868458b9d6
  namespace: kube-system
report:
  checks:
    - category: Kubernetes Security Check
      checkID: KSV051
      description: Check whether role permits creating role bindings and associating
        to privileged role/clusterrole
      messages:
        - ""
      severity: HIGH
      success: true
      title: Do not allow role binding creation and association with privileged role/clusterrole
    - category: Kubernetes Security Check
      checkID: KSV056
      description: The ability to control which pods get service traffic directed to
        them allows for interception attacks. Controlling network policy allows for
        bypassing lateral movement restrictions.
      messages:
        - ""
      severity: HIGH
      success: true
      title: Do not allow management of networking resources
    - category: Kubernetes Security Check
      checkID: KSV041
      description: Check whether role permits managing secrets
      messages:
        - Role permits management of secret(s)
      severity: CRITICAL
      success: false
      title: Do not allow management of secrets
    - category: Kubernetes Security Check
      checkID: KSV047
      description: Check whether role permits privilege escalation from node proxy
      messages:
        - ""
      severity: HIGH
      success: true
      title: Do not allow privilege escalation from node proxy
    - category: Kubernetes Security Check
      checkID: KSV045
      description: Check whether role permits wildcard verb on specific resources
      messages:
        - ""
      severity: CRITICAL
      success: true
      title: No wildcard verb roles
    - category: Kubernetes Security Check
      checkID: KSV054
      description: Check whether role permits attaching to shell on pods
      messages:
        - ""
      severity: HIGH
      success: true
      title: Do not allow attaching to shell on pods
    - category: Kubernetes Security Check
      checkID: KSV044
      description: Check whether role permits wildcard verb on wildcard resource
      messages:
        - ""
      severity: CRITICAL
      success: true
      title: No wildcard verb and resource roles
    - category: Kubernetes Security Check
      checkID: KSV050
      description: An effective level of access equivalent to cluster-admin should not
        be provided.
      messages:
        - ""
      severity: CRITICAL
      success: true
      title: Do not allow management of RBAC resources
    - category: Kubernetes Security Check
      checkID: KSV046
      description: Check whether role permits specific verb on wildcard resources
      messages:
        - ""
      severity: CRITICAL
      success: true
      title: No wildcard resource roles
    - category: Kubernetes Security Check
      checkID: KSV055
      description: Check whether role permits allowing users in a rolebinding to add
        other users to their rolebindings
      messages:
        - ""
      severity: LOW
      success: true
      title: Do not allow users in a rolebinding to add other users to their rolebindings
    - category: Kubernetes Security Check
      checkID: KSV052
      description: Check whether role permits creating role ClusterRoleBindings and
        association with privileged cluster role
      messages:
        - ""
      severity: HIGH
      success: true
      title: Do not allow role to create ClusterRoleBindings and association with privileged
        role
    - category: Kubernetes Security Check
      checkID: KSV053
      description: Check whether role permits getting shell on pods
      messages:
        - ""
      severity: HIGH
      success: true
      title: Do not allow getting shell on pods
    - category: Kubernetes Security Check
      checkID: KSV042
      description: Used to cover attacker’s tracks, but most clusters ship logs quickly
        off-cluster.
      messages:
        - ""
      severity: MEDIUM
      success: true
      title: Do not allow deletion of pod logs
    - category: Kubernetes Security Check
      checkID: KSV049
      description: Some workloads leverage configmaps to store sensitive data or configuration
        parameters that affect runtime behavior that can be modified by an attacker
        or combined with another issue to potentially lead to compromise.
      messages:
        - ""
      severity: MEDIUM
      success: true
      title: Do not allow management of configmaps
    - category: Kubernetes Security Check
      checkID: KSV043
      description: Check whether role permits impersonating privileged groups
      messages:
        - ""
      severity: CRITICAL
      success: true
      title: Do not allow impersonation of privileged groups
    - category: Kubernetes Security Check
      checkID: KSV048
      description: Check whether role permits update/create of a malicious pod
      messages:
        - ""
      severity: HIGH
      success: true
      title: Do not allow update/create of a malicious pod
  scanner:
    name: Trivy
    vendor: Aqua Security
    version: '0.22.0'
  summary:
    criticalCount: 1
    highCount: 0
    lowCount: 0
    mediumCount: 0
  updateTimestamp: null
```


### ClusterComplianceReport

[Reference](https://aquasecurity.github.io/trivy-operator/v0.22.0/docs/crds/clustercompliance-report/)


`ClusterComplianceReport` — это cluster-wide ресурс, содержащий последние результаты проверок соответствия кластера требованиям информационной безопасности. 
На данный момент поддержано  проверка соответствия требованиям [CIS Kubernetes Benchmark](https://www.cisecurity.org/benchmark/kubernetes).

Структура отчета:

`spec.compliance.controls`: представляет описание проверки
`status`: представляет результаты проверок (согласно `spec`), извлеченные из отчетов сканеров безопасности.

> Данный отчет также можно посмотреть в Grafana Dashboard `Security/CIS Kubernetes Benchmark`

Пример отчета (фрагмент):

```yaml 
apiVersion: aquasecurity.github.io/v1alpha1
kind: ClusterComplianceReport
metadata:
  name: cis
spec:
  compliance:
    controls:
    - checks:
      - id: AVD-KCV-0048
      commands:
      - id: CMD-0001
      description: Ensure that the API server pod specification file has permissions
        of 600 or more restrictive
      id: 1.1.1
      name: Ensure that the API server pod specification file permissions are set
        to 600 or more restrictive
      severity: HIGH
    - checks:
      - id: AVD-KCV-0049
      commands:
      - id: CMD-0002
      description: Ensure that the API server pod specification file ownership is
        set to root:root
      id: 1.1.2
      name: Ensure that the API server pod specification file ownership is set to
        root:root
      severity: HIGH
...
  summary:
    failCount: 9
    passCount: 107
  updateTimestamp: "2025-07-29T06:00:00Z"
```

<!-- ### InfraAssessmentReport

[Reference](https://aquasecurity.github.io/trivy-operator/v0.22.0/docs/crds/infraassessment-report/)
Этих объектов почему-то нет
-->

<!-- ### ClusterVulnerabilityReport
Этого объекта тоже нет

[Reference](https://aquasecurity.github.io/trivy-operator/v0.22.0/docs/crds/clustervulnerability-report/)

`ClusterVulnerabilityReport` содержит информацию о последних уязвимостях, обнаруженных в плоскости управления и компонентах узлов кластера Kubernetes. Он содержит список уязвимостей плоскости управления и компонентов узлов с кратким описанием уязвимостей, сгруппированных по уровню серьёзности. ClusterVulnerabilityReports основаны на CVE из рекомендаций по уязвимостям K8s. -->