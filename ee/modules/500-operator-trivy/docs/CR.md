---
title: "The operator-trivy module: Custom Resources"
---

The `operator-trivy` module uses a set of custom resources developed by the [Trivy Operator project](https://aquasecurity.github.io/trivy-operator/) from Aqua Security to represent vulnerability scan results, configuration audits, and cluster compliance checks.

Below is a description of the key CRDs created by the operator, including examples and links to the official documentation.

## Object-level security

### VulnerabilityReport

[`VulnerabilityReport`](https://aquasecurity.github.io/trivy-operator/v0.22.0/docs/crds/vulnerability-report/) is a resource that contains a report on vulnerabilities found in a container image used in a Kubernetes workload.  

The report includes a list of known vulnerabilities in OS packages and application dependencies, grouped by severity levels (`Critical`, `High`, `Medium`, etc.).

For each container in a multi-container workload, `operator-trivy` creates a separate `VulnerabilityReport` in the corresponding namespace.  
The link to the Kubernetes object is established via the `ownerReference` field.

Resource names follow the pattern: `<workload type>-<workload name>-<container name>`

Example:

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
  ownerReferences:
    - apiVersion: apps/v1
      kind: ReplicaSet
      name: nginx-6d4cf56db6
      uid: aa345200-cf24-443a-8f11-ddb438ff8659
      controller: true
      blockOwnerDeletion: false
report:
  artifact:
    repository: library/nginx
    tag: '1.16'
  os:
    family: debian
    name: '10.3'
  registry:
    server: index.docker.io
  scanner:
    name: Trivy
    vendor: Aqua Security
    version: 0.35.0
  summary:
    criticalCount: 2
    highCount: 0
    mediumCount: 0
    lowCount: 0
    unknownCount: 0
  vulnerabilities:
    - vulnerabilityID: CVE-2019-20367
      resource: libbsd0
      installedVersion: 0.9.1-2
      fixedVersion: 0.9.1-2+deb10u1
      severity: CRITICAL
      score: 9.1
      target: library/nginx:1.21.6
      primaryLink: https://avd.aquasec.com/nvd/cve-2019-20367
    - vulnerabilityID: CVE-2018-25009
      resource: libwebp6
      installedVersion: 0.6.1-2
      fixedVersion: ''
      severity: CRITICAL
      score: 9.1
      target: library/nginx:1.16
      title: 'libwebp: out-of-bounds read in WebPMuxCreateInternal'
      primaryLink: https://avd.aquasec.com/nvd/cve-2018-25009
```

### ConfigAuditReport

[`ConfigAuditReport`](https://aquasecurity.github.io/trivy-operator/v0.22.0/docs/crds/configaudit-report/) is a resource that contains the results of a configuration audit of a Kubernetes object using tools such as Trivy.  

The report includes a list of configuration issues grouped by categories (e.g., `Security`) and severity levels (`Critical`, `High`, etc.).

Examples of checks include:

- running a container as a non-root user;
- defining resource requests and limits for containers;
- configuring network access (`hostNetwork`, `hostPID`, etc.);
- setting security flags to prevent privilege escalation.

`ConfigAuditReport` can be created for any namespaced resource, including:

- workloads (`Pod`, `Deployment`, `StatefulSet`, etc.);
- auxiliary resources (`Service`, `ConfigMap`, `Role`, `RoleBinding`, etc.).

Each report is linked to the audited object via `ownerReference` and stored in the same namespace.  
Resource names follow the pattern: `<workload type>-<workload name>`

Example:

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
  ownerReferences:
    - apiVersion: apps/v1
      kind: ReplicaSet
      name: nginx-6d4cf56db6
report:
  scanner:
    name: Trivy 
    vendor: Aqua Security
    version: '0.22.0'
  summary:
    criticalCount: 2
    highCount: 0
    mediumCount: 0
    lowCount: 9
  checks:
    - checkID: hostPIDSet
      severity: CRITICAL
      messages: ["Host PID is not configured"]
      success: true
    - checkID: notReadOnlyRootFilesystem
      severity: LOW
      messages: ["Filesystem should be read only"]
      success: false
      scope:
        type: Container
        value: nginx
```

### ExposedSecretReport

[`ExposedSecretReport`](https://aquasecurity.github.io/trivy-operator/v0.22.0/docs/crds/exposedsecret-report/) is a report on potential secrets discovered in a container image used in a Kubernetes workload.

The report lists strings that contain sensitive data (e.g., tokens, keys, passwords) found in files within the image. Each finding includes a category, rule ID, severity level, and file path.

For each container in a multi-container workload, `operator-trivy` creates a separate `ExposedSecretReport` in the workload’s namespace.  
The report is linked to the workload via the `ownerReference`.

Resource names follow the pattern: `<workload type>-<workload name>-<container name>`

Example:

```yaml
apiVersion: aquasecurity.github.io/v1alpha1
kind: ExposedSecretReport
metadata:
  name: replicaset-app-67b77f5965-app
  namespace: default
  labels:
    trivy-operator.container.name: app
    trivy-operator.resource.kind: ReplicaSet
    trivy-operator.resource.name: app-67b77f5965
    trivy-operator.resource.namespace: default
  ownerReferences:
    - apiVersion: apps/v1
      kind: ReplicaSet
      name: app-67b77f5965
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
      ruleID: stripe-access-token
      severity: HIGH
      target: "/app/config/secret.yaml"
      match: "publishable_key: *****"
      title: Stripe
    - category: Stripe
      ruleID: stripe-access-token
      severity: HIGH
      target: "/app/config/secret.yaml"
      match: "secret_key: *****"
      title: Stripe
  summary:
    criticalCount: 0
    highCount: 2
    mediumCount: 0
    lowCount: 0
  updateTimestamp: "2022-06-29T14:29:37Z"
```

### SbomReport

[`SbomReport`](https://aquasecurity.github.io/trivy-operator/v0.22.0/docs/crds/sbom-report/) is a report containing the SBOM (Software Bill of Materials) for a container image used in a Kubernetes workload.

It lists all software components, including OS packages and application dependencies found in the container.  
This information is useful for analyzing image contents, performing security audits, and ensuring compliance with vendor requirements.

For a multi-container workload, `trivy-operator` creates a separate `SbomReport` for each container.  
The report is stored in the same namespace as the workload and linked via the `ownerReference`.

Resource names follow the pattern: `<workload type>-<workload name>-<container name>`

Example:

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

## Cluster-level security

### RbacAssessmentReport

[`RbacAssessmentReport`](https://aquasecurity.github.io/trivy-operator/v0.22.0/docs/crds/rbacassessment-report/) is a report based on the analysis of RBAC (Role-Based Access Control) settings in the Kubernetes cluster.

It includes results of checks performed by configuration audit tools such as Trivy.  
Examples of checks include identifying roles that:

- grant excessive privileges (e.g., full access to secrets across all API groups);
- violate the principle of least privilege.

Each report is associated with a specific `Role` or `ClusterRole` and is stored in the same namespace as the audited object.

Resource names follow the pattern: `<Role|ClusterRole>-<role name>`

Example:

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

[`ClusterComplianceReport`](https://aquasecurity.github.io/trivy-operator/v0.22.0/docs/crds/clustercompliance-report/) is a cluster-wide resource that contains a summary report on cluster compliance with security requirements.

Currently, it supports compliance checks against the [CIS Kubernetes Benchmark](https://www.cisecurity.org/benchmark/kubernetes) — a set of best practices for secure Kubernetes configuration.

Report structure:

- `spec.compliance.controls` — defines the compliance checks.
- `status` — contains the results of the checks defined in `spec`. Results are based on reports from various security scanners.

> The results of this report can also be viewed in the Grafana dashboard `Security / CIS Kubernetes Benchmark`.

Example:

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
