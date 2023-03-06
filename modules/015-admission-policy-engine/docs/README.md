---
title: "The admission-policy-engine module"
---

This module enforces the security policies in the cluster according to the Kubernetes [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/) using the [Gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/docs/) solution.

The Pod Security Standards define three different policies to broadly cover the security spectrum. These policies are cumulative and range from highly-permissive to highly-restrictive:
- `Privileged` — Unrestricted policy. Provides the widest possible permission level (used by default).
- `Baseline` — Minimally restrictive policy which prevents known privilege escalations. Allows for the default (minimally specified) Pod configuration.
- `Restricted` — Heavily restricted policy. Follows the most current Pod hardening best practices.

You can read more about each policy variety in the [Kubernetes documentation](https://kubernetes.io/docs/concepts/security/pod-security-standards/).

To apply a policy set the label `security.deckhouse.io/pod-policy =<POLICY_NAME>` to the corresponding namespace.

Example of the command to set the `Restricted` policy for all Pods in the `my-namespace` Namespace.

```bash
kubectl label ns my-namespace security.deckhouse.io/pod-policy=restricted
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

### Modifying Kubernetes resources

The module also allows you to use the Gatekeeper's Custom Resources to easily modify objects in the cluster, such as
- `AssignMetadata` — defines changes to the metadata section of a resource.
- `Assign` —  any change outside the metadata section.
- `ModifySet` —  adds or removes entries from a list, such as the arguments to a container.

Example:

```yaml
ApiVersion: mutations.gatekeeper.sh/v1
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
assignment:
value: "bar"
```

You can read more about the available options in the [gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/docs/mutation/) documentation.
