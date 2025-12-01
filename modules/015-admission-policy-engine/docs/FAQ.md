---
title: "The admission-policy-engine module: FAQ"
---

## How do I configure alternative security policy management solutions?

For DKP to work correctly, extended privileges are required to run and operate system component payloads. If you are using some alternative security policy management solution (e. g., Kyverno) instead of the admission-policy-engine module, you have to configure exceptions for the following namespaces:
- `kube-system`;
- all namespaces with the `d8-*` prefix (e.g., `d8-system`).

## How do I extend Pod Security Standards policies?

> Pod Security Standards respond to the `security.deckhouse.io/pod-policy: restricted` or `security.deckhouse.io/pod-policy: baseline` label.

To extend the Pod Security Standards policy by adding your checks to existing checks, you need to:
- Create a constraint template for the check (a `ConstraintTemplate` resource).
- Bind it to the `restricted` or `baseline` policy.

Example of the `ConstraintTemplate` for checking a  repository URL of a container image:

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

Example of binding a check to the `restricted` policy:

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

The example demonstrates the configuration of checking the repository address in the `image` field for all Pods created in the namespace having the `security.deckhouse.io/pod-policy : restricted`  label. A Pod will not be created if the address in the `image` field of the Pod does not start with `mycompany.registry.com`.

The [Gatekeeper documentation](https://open-policy-agent.github.io/gatekeeper/website/docs/howto) may find more info about templates and policy language.

Find more examples of checks for policy extension in the [Gatekeeper Library](https://github.com/open-policy-agent/gatekeeper-library/tree/master/src/general).

## How to allow some Pod Security Standards policies without disabling whole list?

To apply only the required security policies without turning off the entire built-in set:

1. Add the `security.deckhouse.io/pod-policy: privileged` label to your namespace in order to disable built-in policies.
1. Create a SecurityPolicy resource that matches the [baseline](https://kubernetes.io/docs/concepts/security/pod-security-standards/#baseline) or [restricted](https://kubernetes.io/docs/concepts/security/pod-security-standards/#restricted) policy while also editing the list of `policies` elements as you see fit.
1. Add a label to your namespace that matches the `namespaceSelector` in the SecurityPolicy resource. In the examples below, the label is `security-policy.deckhouse.io/baseline-enabled: "true"` or `security-policy.deckhouse.io/restricted-enabled: "true"`.

SecurityPolicy that matches baseline standard:

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
          security-policy.deckhouse.io/baseline-enabled: "true"
```

`SecurityPolicy` that matches restricted standard:

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
          security-policy.deckhouse.io/restricted-enabled: "true"
```

## What if there are multiple policies (operational or security) that are applied to the same object?

In that case the object's specification have to fulfil all the requirements imposed by the policies.

For example, consider the following two security policies:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: SecurityPolicy
metadata:
  name: foo
spec:
  enforcementAction: Deny
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          name: test
  policies:
    readOnlyRootFilesystem: true
    requiredDropCapabilities:
    - MKNOD
---
apiVersion: deckhouse.io/v1alpha1
kind: SecurityPolicy
metadata:
  name: bar
spec:
  enforcementAction: Deny
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          name: test
  policies:
    requiredDropCapabilities:
    - NET_BIND_SERVICE
```

Then, in order to fulfill the requirements of the above security policies, the following settings must be set in a container specification:

```yaml
    securityContext:
      capabilities:
        drop:
          - MKNOD
          - NET_BIND_SERVICE
      readOnlyRootFilesystem: true
```

## Verification of image signatures

{% alert level="warning" %}This feature is available in the following editions: SE+, EE.{% endalert %}

The module implements a function for checking the signatures of container images signed using [Cosign](https://docs.sigstore.dev/cosign/key_management/signing_with_self-managed_keys/#:~:text=To%20generate%20a%20key%20pair,prompted%20to%20provide%20a%20password.&text=Alternatively%2C%20you%20can%20use%20the,%2C%20ECDSA%2C%20and%20ED25519%20keys). Checking the signatures of container images allows you to ensure their integrity (that the image has not been modified since its creation) and authenticity (that the image was created by a trusted source). You can enable container image signature verification in the cluster using the [policies.verifyImageSignatures](cr.html#securitypolicy-v1alpha1-spec-policies-verifyimagesignatures) parameter of the SecurityPolicy resource.

{% offtopic title="How to sign an image..." %}
Steps to sign an image:
- Generate keys: `cosign generate-key-pair`
- Sign the image: `cosign sign --key <key> <image>`

For more information on working with Cosign, you can check the [documentation](https://docs.sigstore.dev/cosign/key_management).
{% endofftopic %}

Example of SecurityPolicy for configuring the signature verification of container images:

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

Policies do not affect the creation of pods whose container image addresses do not match those described in the `reference` parameter. If the address of any Pod container image matches those described in the `reference` policies, and the image is not signed or the signature does not correspond to the keys specified in the policy, the creation of the pod will be prohibited.

Example of an error output when creating a Pod with a container image that has not passed the signature verification:

```console
[verify-image-signatures] Image signature verification failed: nginx:1.17.2
```

## How to block deleting a node without a label

> Note. DELETE operations are handled by Gatekeeper by default.

You can create your own Gatekeeper policy to block Node deletion unless a special label is present. The example below uses `oldObject` to check labels on the Node being deleted:

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

## How to deny kubectl exec и kubectl attach to specific Pods?

> The `admission-policy-engine` module's webhook routes `CONNECT` requests for `pods/exec` and `pods/attach` through Gatekeeper. This allows custom policies to allow or deny `kubectl exec` and `kubectl attach` operations.

### Built-in policy for heritage: deckhouse Pods

To protect system components managed by Deckhouse, the `admission-policy-engine` module includes a built-in policy `D8DenyExecHeritage` that forbids running `kubectl exec` and `kubectl attach` operations to all Pods with the `heritage: deckhouse` label.

This policy doesn't apply to the following users who are allowed to run `kubectl exec` and `kubectl attach` operations to Pods labeled with `heritage: deckhouse`:

- `system:sudouser`;
- service accounts from `d8-*` namespaces (`system:serviceaccount:d8-*`);
- service accounts from `kube-*` namespaces (`system:serviceaccount:kube-*`).

### Custom policy example

You can create your own Gatekeeper policy to deny `kubectl exec` and `kubectl attach` operations in specific namespaces. In the following example,  `input.review.operation` and `input.review.resource.resource` are used to check for `CONNECT` operations:

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

        is_connect { input.review.operation == "CONNECT" }
        is_exec_or_attach { input.review.resource.resource == "pods/exec" }
        is_exec_or_attach { input.review.resource.resource == "pods/attach" }

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

Key data and checks for CONNECT validation:

- Use `input.review.operation == "CONNECT"` to check for `CONNECT` operations.
- Use `input.review.resource.resource` to check for `pods/exec` or `pods/attach` subresources.
- User information is available in `input.review.userInfo.username` and `input.review.userInfo.groups`.
- The namespace is available in `input.review.namespace`.
