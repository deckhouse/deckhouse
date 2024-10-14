---
title: "The admission-policy-engine module"
description: Deckhouse admission-policy-engine module enforces the security policies in a Kubernetes cluster according to the Kubernetes Pod Security Standards.
---

This module enforces the security policies in the cluster according to the Kubernetes [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/) using the [Gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/docs/) solution.

The Pod Security Standards define three different policies to broadly cover the security spectrum. These policies are cumulative and range from highly-permissive to highly-restrictive.

{% alert level="info" %}
The module does not apply policies to system namespaces.
{% endalert %}

List of policies available for use:
- `Privileged` — Unrestricted policy. Provides the widest possible permission level;
- `Baseline` — Minimally restrictive policy which prevents known privilege escalations. Allows for the default (minimally specified) Pod configuration;
- `Restricted` — Heavily restricted policy. Follows the most current Pod hardening best practices.

You can read more about each policy variety in the [Kubernetes documentation](https://kubernetes.io/docs/concepts/security/pod-security-standards/#profile-details).

The type of cluster policy to use by default is determined based on the following criteria:
- If a Deckhouse version **lower than v1.55** is being installed, the `Privileged` default policy is applied to all non-system namespaces;
- If a Deckhouse version starting with **v1.55** is being installed, the `Baseline` default policy is applied to all non-system namespaces;

**Note** that upgrading Deckhouse in a cluster to v1.55 does not automatically result in a default policy change.

Default policy can be overridden either globally ([in the module settings](configuration.html#parameters-podsecuritystandards-defaultpolicy)) or on a per-namespace basis (using the `security.deckhouse.io/pod-policy=<POLICY_NAME>` label for the corresponding namespace).

Example of the command to set the `Restricted` policy for all Pods in the `my-namespace` Namespace.

```bash
kubectl label ns my-namespace security.deckhouse.io/pod-policy=restricted
```

By default, Pod Security Standards policies have their enforcement actions set to "Deny" which means any workload pods not compliant to the selected policy won't be able to run. This behavior can be adjusted either for the whole cluster or per namespace. For setting PSS enforcement action cluster-wide check [configuration](configuration.html#parameters-podsecuritystandards-enforcementaction). In case you want to override default enforcement action for a namespace, set label `security.deckhouse.io/pod-policy-action =<POLICY_ACTION>` to the corresponding namespace. The list of possible enforcement actions consists of the following values: "dryrun", "warn", "deny".

Below is an example of setting the "warn" PSS policy mode for all pods in the `my-namespace` namespace:

```bash
kubectl label ns my-namespace security.deckhouse.io/pod-policy-action=warn
```

The policies define by the module can be expanded. Examples of policy extensions can be found in the [FAQ](faq.html).

### Operation policies

The module provides a set of operating policies and best practices for the secure operation of your applications.
We recommend you deploy the following minimum set of operating policies:

```yaml
---
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

To apply the policy, it will be sufficient to set the label `operation-policy.deckhouse.io/enabled: "true"` on the desired namespace.
The above policy is generic and recommended by Deckhouse team. Similarly, you can configure your own policy with the necessary settings.

### Security policies

The module allows defining security policies for making sure the workload running in the cluster meets certain security requirements.

An example of a security policy:

```yaml
---
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

To apply the policy, it will be sufficient to set the label `enforce: "mypolicy"` on the desired namespace.

### Modifying Kubernetes resources

The module also allows you to use the Gatekeeper's Custom Resources to easily modify objects in the cluster, such as
- `AssignMetadata` — defines changes to the metadata section of a resource.
- `Assign` —  any change outside the metadata section.
- `ModifySet` —  adds or removes entries from a list, such as the arguments to a container.

Example:

```yaml
apiVersion: mutations.gatekeeper.sh/v1
kind: AssignMetadata
metadata:
  name: demo-annotation-owner
spec:
  match:
    scope: Namespaced
    namespaces: ["default"]
    kinds:
      - apiGroups: [""]
        kinds: ["Pod"]
  location: "metadata.annotations.foo"
  parameters:
    assign:
      value: "bar"
```

You can read more about the available options in the [gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/docs/mutation/) documentation.
