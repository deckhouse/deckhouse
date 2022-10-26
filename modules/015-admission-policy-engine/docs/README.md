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
