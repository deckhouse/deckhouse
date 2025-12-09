---
title: Security policies
permalink: en/admin/configuration/security/policies.html
description: "Configure security policies in Deckhouse Kubernetes Platform using Gatekeeper and Pod Security Standards. Policy enforcement, compliance, and cluster security management."
---

Deckhouse Kubernetes Platform (DKP) lets you manage application security in the cluster using a set of policies
that follow the [Kubernetes Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/) model
and can be extended with DKP's built-in mechanisms.

DKP implements security policies using [Gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/docs/).

## Applying Pod Security Standards

DKP supports three security policy levels:

- `privileged`: An unrestricted policy with the broadest possible permissions.
- `baseline`: A minimally restrictive policy that prevents the most well-known and common privilege escalation techniques.
  Allows the use of a standard (minimally specified) Pod configuration.
- `restricted`: A highly restrictive policy with the strictest requirements for Pods.

### Default policy

The default policy is determined as follows:

- In DKP versions prior to v1.55, the default policy is `privileged`.
- Starting from DKP v1.55, the default policy is `baseline`.

{% alert level="info" %}
When upgrading DKP to v1.55 or later, the default policy will not change automatically.
{% endalert %}

### Assigning a policy

You can assign a policy in the following ways:

- Globally, using the [`settings.podSecurityStandards.defaultPolicy`](/modules/admission-policy-engine/configuration.html#parameters-podsecuritystandards-defaultpolicy) parameter of the [`admission-policy-engine`](/modules/admission-policy-engine/) module.
- Per namespace, using the `security.deckhouse.io/pod-policy=<POLICY_NAME>` label.

  Example command to assign the `restricted` policy to all Pods in the `my-namespace` namespace:

  ```shell
  d8 k label ns my-namespace security.deckhouse.io/pod-policy=restricted
  ```

### Enforcement modes

Allowed policy enforcement modes:

- `deny`: Blocks actions from being executed.
- `dryrun`: Does not affect execution and used for debugging.
  Event information can be viewed in Grafana or in the console using `kubectl`.
- `warn`: Works like `dryrun` but also displays a warning with the reason the action would have been denied in `deny` mode.

By default, Pod Security Standards policies in DKP are enforced in `deny` mode.
In this mode, application Pods that do not comply with the policies cannot be run in the cluster.

As with policy assignment, enforcement mode can be set:

- Globally, using the [`settings.podSecurityStandards.enforcementAction`](/modules/admission-policy-engine/configuration.html#parameters-podsecuritystandards-enforcementaction) parameter of the [`admission-policy-engine`](/modules/admission-policy-engine/) module.
- Per namespace, using the `security.deckhouse.io/pod-policy-action=<POLICY_ACTION>` label.

  Example command to set the `warn` mode for all Pods in the `my-namespace` namespace:

  ```shell
  d8 k label ns my-namespace security.deckhouse.io/pod-policy-action=warn
  ```

### Extending a policy

You can extend the `baseline` and `restricted` policies using Gatekeeper templates
by adding extra checks to the existing ones.

To extend a policy:

1. Create a validation template using a ConstraintTemplate resource.
1. Apply the template to the `baseline` or `restricted` policy.

Example template for validating the container image repository address:

```yaml
apiVersion: templates.gatekeeper.sh/v1
kind: ConstraintTemplate
metadata:
  name: k8sallowedrepos
spec:
  crd:
    spec:
      names:
        kind: K8sAllowedRepos
      validation:
        openAPIV3Schema:
          type: object
          properties:
            repos:
              type: array
              items:
                type: string
  targets:
    - target: admission.k8s.gatekeeper.sh
      rego: |
        package d8.pod_security_standards.extended

        violation[{"msg": msg}] {
          container := input.review.object.spec.containers[_]
          satisfied := [good | repo = input.parameters.repos[_] ; good = startswith(container.image, repo)]
          not any(satisfied)
          msg := sprintf("container <%v> has an invalid image repo <%v>, allowed repos are %v", [container.name, container.image, input.parameters.repos])
        }

        violation[{"msg": msg}] {
          container := input.review.object.spec.initContainers[_]
          satisfied := [good | repo = input.parameters.repos[_] ; good = startswith(container.image, repo)]
          not any(satisfied)
          msg := sprintf("container <%v> has an invalid image repo <%v>, allowed repos are %v", [container.name, container.image, input.parameters.repos])
        }
```

Example of applying the template to the `restricted` policy:

```yaml
apiVersion: constraints.gatekeeper.sh/v1beta1
kind: K8sAllowedRepos
metadata:
  name: prod-repo
spec:
  match:
    kinds:
      - apiGroups: [""]
        kinds: ["Pod"]
    namespaceSelector:
      matchLabels:
        security.deckhouse.io/pod-policy: restricted
  parameters:
    repos:
      - "mycompany.registry.com"
```

In this example, the repository address in the `image` field of all Pods in namespaces
labeled `security.deckhouse.io/pod-policy: restricted` is checked.
If the `image` address of a created Pod does not start with `mycompany.registry.com`, the Pod will not be created.

Helpful resources for creating extended policies:

- [Custom policy examples](#custom-gatekeeper-policy-examples).
- [Gatekeeper documentation](https://open-policy-agent.github.io/gatekeeper/website/docs/howto/) contains information
  on templates and policy language.
- [Gatekeeper library](https://github.com/open-policy-agent/gatekeeper-library/tree/master/src/general) contains examples
  of validation templates.

## Operational policies

DKP provides a mechanism for creating operational policies using the [OperationPolicy](/modules/admission-policy-engine/cr.html#operationpolicy) resource.
Operational policies define requirements for cluster objects such as allowed repositories, required resources, probes, and more.

The DKP development team recommends applying the following minimal operational policy:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: OperationPolicy
metadata:
  name: common
spec:
  policies:
    allowedRepos:
      - myrepo.example.com
      - registry.deckhouse.io
    requiredResources:
      limits:
        - memory
      requests:
        - cpu
        - memory
    disallowedImageTags:
      - latest
    requiredProbes:
      - livenessProbe
      - readinessProbe
    maxRevisionHistoryLimit: 3
    imagePullPolicy: Always
    priorityClassNames:
    - production-high
    - production-low
    checkHostNetworkDNSPolicy: true
    checkContainerDuplicates: true
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          operation-policy.deckhouse.io/enabled: "true"
```

This policy defines basic operational requirements for cluster objects,
including allowed container registries, required resources and probes, restrictions on using images with the `latest` tag,
allowed priority classes, and other settings that improve application security and stability.

To assign this operational policy, add the `operation-policy.deckhouse.io/enabled=true` label to the target namespace:

```shell
d8 k label ns my-namespace operation-policy.deckhouse.io/enabled=true
```

## Security policies

Using the [SecurityPolicy](/modules/admission-policy-engine/cr.html#securitypolicy) resource,
you can create security policies that define container behavior restrictions in the cluster,
such as host network access, privileges, AppArmor usage, and more.

Example security policy:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: SecurityPolicy
metadata:
  name: mypolicy
spec:
  enforcementAction: Deny
  policies:
    allowHostIPC: true
    allowHostNetwork: true
    allowHostPID: false
    allowPrivileged: false
    allowPrivilegeEscalation: false
    allowedFlexVolumes:
    - driver: vmware
    allowedHostPorts:
    - max: 4000
      min: 2000
    allowedProcMount: Unmasked
    allowedAppArmor:
    - unconfined
    allowedUnsafeSysctls:
    - kernel.*
    allowedVolumes:
    - hostPath
    - projected
    fsGroup:
      ranges:
      - max: 200
        min: 100
      rule: MustRunAs
    readOnlyRootFilesystem: true
    requiredDropCapabilities:
    - ALL
    runAsGroup:
      ranges:
      - max: 500
        min: 300
      rule: RunAsAny
    runAsUser:
      ranges:
      - max: 200
        min: 100
      rule: MustRunAs
    seccompProfiles:
      allowedLocalhostFiles:
      - my_profile.json
      allowedProfiles:
      - Localhost
    supplementalGroups:
      ranges:
      - max: 133
        min: 129
      rule: MustRunAs
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          enforce: mypolicy
```

To assign this security policy, add the `enforce: "mypolicy"` label to the target namespace.

### Partial policy enforcement

To enforce specific security policies without disabling the entire predefined set, follow these steps:

1. Add the `security.deckhouse.io/pod-policy: privileged` label to the target namespace to disable the built-in policy set.
1. Create a [SecurityPolicy](/modules/admission-policy-engine/cr.html#securitypolicy) resource
   that matches the `baseline` or `restricted` level.
   In the `policies` section, specify only the security settings you need.
1. Add an extra label to the namespace matching the `namespaceSelector` in the SecurityPolicy.

Example SecurityPolicy configuration for the `baseline` level:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: SecurityPolicy
metadata:
  name: baseline
spec:
  enforcementAction: Deny
  policies:
    allowHostIPC: false
    allowHostNetwork: false
    allowHostPID: false
    allowPrivilegeEscalation: true
    allowPrivileged: false
    allowedAppArmor:
      - runtime/default
      - localhost/*
    allowedCapabilities:
      - AUDIT_WRITE
      - CHOWN
      - DAC_OVERRIDE
      - FOWNER
      - FSETID
      - KILL
      - MKNOD
      - NET_BIND_SERVICE
      - SETFCAP
      - SETGID
      - SETPCAP
      - SETUID
      - SYS_CHROOT
    allowedHostPaths: []
    allowedHostPorts:
      - max: 0
        min: 0
    allowedProcMount: Default
    allowedUnsafeSysctls:
      - kernel.shm_rmid_forced
      - net.ipv4.ip_local_port_range
      - net.ipv4.ip_unprivileged_port_start
      - net.ipv4.tcp_syncookies
      - net.ipv4.ping_group_range
      - net.ipv4.ip_local_reserved_ports
      - net.ipv4.tcp_keepalive_time
      - net.ipv4.tcp_fin_timeout
      - net.ipv4.tcp_keepalive_intvl
      - net.ipv4.tcp_keepalive_probes
    seLinux:
      - type: ""
      - type: container_t
      - type: container_init_t
      - type: container_kvm_t
      - type: container_engine_t
    seccompProfiles:
      allowedProfiles:
        - RuntimeDefault
        - Localhost
        - undefined
        - ''
      allowedLocalhostFiles:
        - '*'
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          operation-policy.deckhouse.io/baseline-enabled: "true"
```

Example SecurityPolicy configuration for the `restricted` level:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: SecurityPolicy
metadata:
  name: restricted
spec:
  enforcementAction: Deny
  policies:
    allowHostIPC: false
    allowHostNetwork: false
    allowHostPID: false
    allowPrivilegeEscalation: false
    allowPrivileged: false
    allowedAppArmor:
      - runtime/default
      - localhost/*
    allowedCapabilities:
      - NET_BIND_SERVICE
    allowedHostPaths: []
    allowedHostPorts:
      - max: 0
        min: 0
    allowedProcMount: Default
    allowedUnsafeSysctls:
      - kernel.shm_rmid_forced
      - net.ipv4.ip_local_port_range
      - net.ipv4.ip_unprivileged_port_start
      - net.ipv4.tcp_syncookies
      - net.ipv4.ping_group_range
      - net.ipv4.ip_local_reserved_ports
      - net.ipv4.tcp_keepalive_time
      - net.ipv4.tcp_fin_timeout
      - net.ipv4.tcp_keepalive_intvl
      - net.ipv4.tcp_keepalive_probes
    allowedVolumes:
      - configMap
      - csi
      - downwardAPI
      - emptyDir
      - ephemeral
      - persistentVolumeClaim
      - projected
      - secret
    requiredDropCapabilities:
      - ALL
    runAsUser:
      rule: MustRunAsNonRoot
    seLinux:
      - type: ""
      - type: container_t
      - type: container_init_t
      - type: container_kvm_t
      - type: container_engine_t
    seccompProfiles:
      allowedProfiles:
        - RuntimeDefault
        - Localhost
      allowedLocalhostFiles:
        - '*'
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          operation-policy.deckhouse.io/restricted-enabled: "true"
```

## Gatekeeper custom resources

Gatekeeper offers advanced capabilities for modifying Kubernetes resources using mutation policies.
These policies are defined through the following custom resources:

- [AssignMetadata](/modules/admission-policy-engine/gatekeeper-cr.html#assignmetadata): Modifies the `metadata` section of a resource.
- [Assign](/modules/admission-policy-engine/gatekeeper-cr.html#assign): Modifies fields other than `metadata`.
- [ModifySet](/modules/admission-policy-engine/gatekeeper-cr.html#modifyset): Adds or removes values from a list,
  such as container run arguments.
- [AssignImage](/modules/admission-policy-engine/gatekeeper-cr.html#assignimage): Modifies the `image` field of a resource.

For more on modifying Kubernetes resources using mutation policies, refer to the [Gatekeeper documentation](https://open-policy-agent.github.io/gatekeeper/website/docs/mutation/).

## Custom Gatekeeper policy examples

Here you can find examples of Gatekeeper policies that let you extend the standard cluster security mechanism.

### Preventing deletion of a node without a specified label

{% alert level="info" %}
The `DELETE` operations are handled by Gatekeeper by default.
{% endalert %}

You can create a Gatekeeper policy to prevent a node deletion unless it has a special label assigned.

In the following example, the `oldObject` field is used to check labels on the node being deleted:

```yaml
apiVersion: templates.gatekeeper.sh/v1
kind: ConstraintTemplate
metadata:
  name: d8customnodedeleteguard
spec:
  crd:
    spec:
      names:
        kind: D8CustomNodeDeleteGuard
      validation:
        openAPIV3Schema:
          type: object
          properties:
            requiredLabelKey:
              type: string
            requiredLabelValue:
              type: string
  targets:
    - target: admission.k8s.gatekeeper.sh
      rego: |
        package d8.custom

        is_delete { input.review.operation == "DELETE" }
        is_node { input.review.kind.kind == "Node" }

        has_required_label {
          key := input.parameters.requiredLabelKey
          val := input.parameters.requiredLabelValue
          obj := input.review.oldObject
          obj.metadata.labels[key] == val
        }

        violation[{"msg": msg}] {
          is_delete
          is_node
          not has_required_label
          msg := sprintf("Node deletion is blocked. Add label %q=%q to proceed.", [input.parameters.requiredLabelKey, input.parameters.requiredLabelValue])
        }
---
apiVersion: constraints.gatekeeper.sh/v1beta1
kind: D8CustomNodeDeleteGuard
metadata:
  name: require-node-delete-label
spec:
  enforcementAction: warn
  match:
    kinds:
      - apiGroups: [""]
        kinds: ["Node"]
  parameters:
    requiredLabelKey: "admission.deckhouse.io/allow-delete"
    requiredLabelValue: "true"
```

### Preventing kubectl exec and kubectl attach operations to specific Pods

The `admission-policy-engine` module's webhook routes `CONNECT` requests for `pods/exec` and `pods/attach` through Gatekeeper.
This allows custom policies to allow or deny `kubectl exec` and `kubectl attach` operations.

#### Built-in policy for heritage: deckhouse Pods

To protect system components managed by Deckhouse,
the `admission-policy-engine` module includes a built-in policy `D8DenyExecHeritage`
that forbids running `kubectl exec` and `kubectl attach` operations to all Pods with the `heritage: deckhouse` label.

This policy doesn't apply to the following users
who are allowed to run `kubectl exec` and `kubectl attach` operations to Pods labeled with `heritage: deckhouse`:

- `system:sudouser`
- service accounts from `d8-*` namespaces (`system:serviceaccount:d8-*`)
- service accounts from `kube-*` namespaces (`system:serviceaccount:kube-*`)

#### Custom policy example

You can create your own Gatekeeper policy to deny `kubectl exec` and `kubectl attach` operations in specific namespaces.
In the following example, `input.review.operation` and `input.review.resource.resource` are used to check for `CONNECT` operations:

```yaml
apiVersion: templates.gatekeeper.sh/v1
kind: ConstraintTemplate
metadata:
  name: d8customdenyexec
spec:
  crd:
    spec:
      names:
        kind: D8CustomDenyExec
      validation:
        openAPIV3Schema:
          type: object
          properties:
            forbiddenNamespaces:
              type: array
              items:
                type: string
  targets:
    - target: admission.k8s.gatekeeper.sh
      rego: |
        package d8.custom

        is_connect {
          input.review.operation == "CONNECT"
        }

        # requestSubResource is preferred, but fall back to subResource for older APIs
        subresource_is(sub) {
          sr := object.get(input.review, "requestSubResource", input.review.subResource)
          sr == sub
        }

        is_exec_or_attach {
          input.review.resource.resource == "pods"
          subresource_is("exec")
        }

        is_exec_or_attach {
          input.review.resource.resource == "pods"
          subresource_is("attach")
        }

        is_forbidden_namespace {
          ns := input.review.namespace
          ns == input.parameters.forbiddenNamespaces[_]
        }

        violation[{"msg": msg}] {
          is_connect
          is_exec_or_attach
          is_forbidden_namespace
          msg := sprintf("Exec/attach is forbidden in namespace %q", [input.review.namespace])
        }
---
apiVersion: constraints.gatekeeper.sh/v1beta1
kind: D8CustomDenyExec
metadata:
  name: deny-exec-in-namespaces
spec:
  enforcementAction: deny
  match:
    kinds:
      - apiGroups: ["*"]
        kinds: ["*"]
    scope: Namespaced
  parameters:
    forbiddenNamespaces:
      - production
      - staging
```

Key data and checks for `CONNECT` validation:

- Use `input.review.operation == "CONNECT"` to check for `CONNECT` operations.
- User information is available in `input.review.userInfo.username` and `input.review.userInfo.groups`.
- The namespace is available in `input.review.namespace`.

## Image signature verification

{% alert level="warning" %}
Available in DKP Enterprise Edition only.
{% endalert %}

DKP supports container image signature verification using [Cosign](https://docs.sigstore.dev/cosign/key_management/signing_with_self-managed_keys/).
Verification ensures image integrity and authenticity.

To sign an image with Cosign, do the following:

1. Generate a key pair:

   ```shell
   cosign generate-key-pair
   ```

1. Sign the image:

   ```shell
   cosign sign --key <KEY> <IMAGE>
   ```

To enable container image signature verification in a DKP cluster,
use the [`policies.verifyImageSignatures`](/modules/admission-policy-engine/cr.html#securitypolicy-v1alpha1-spec-policies-verifyimagesignatures) parameter
of the SecurityPolicy resource.

Example SecurityPolicy configuration for verifying container image signatures:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: SecurityPolicy
metadata:
  name: verify-image-signatures
spec:
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          kubernetes.io/metadata.name: default
  policies:
    verifyImageSignatures:
      - reference: docker.io/myrepo/*
        publicKeys:
        - |-
          -----BEGIN PUBLIC KEY-----
          MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8nXRh950IZbRj8Ra/N9sbqOPZrfM
          5/KAQN0/KjHcorm/J5yctVd7iEcnessRQjU917hmKO6JWVGHpDguIyakZA==
          -----END PUBLIC KEY-----
      - reference: company.registry.com/*
        dockerCfg: zxc==
        publicKeys:
        - |-
          -----BEGIN PUBLIC KEY-----
          MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8nXRh950IZbRj8Ra/N9sbqOPZrfM
          5/KAQN0/KjHcorm/J5yctVd7iEcnessRQjU917hmKO6JWVGHpDguIyakZA==
          -----END PUBLIC KEY-----
```

With this policy, if a container image address matches the value of the `reference` parameter
and the image is unsigned or the signature does not match the specified keys, Pod creation will be denied.

Example error output when creating a Pod with an unverified container image:

```console
[verify-image-signatures] Image signature verification failed: nginx:1.17.2
```

## Using alternative security policy management tools

If you use an alternative solution for security policy management in a DKP cluster
(for example, [Kyverno](https://kyverno.io/docs/introduction/)), configure exceptions for the following namespaces:

- `kube-system`
- all namespaces with the `d8-*` prefix (for example, `d8-system`)

Without these exceptions, policies may block or disrupt system component operations.
