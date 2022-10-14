---
title: "The admission-policy-engine module"
---
This module enforces the security policies in the cluster using the [gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/docs/) solution.

## Pod Security Standards

The Pod Security Standards define three different policies to broadly cover the security spectrum. These policies are cumulative and range from highly-permissive to highly-restrictive.
- Privileged — Unrestricted policy. Provides the widest possible permission level (used by default).
- Baseline — Minimally restrictive policy which prevents known privilege escalations. Allows for the default (minimally specified) Pod configuration.
- Restricted — Heavily restricted policy. Follows the most current Pod hardening best practices.

You can read more about each policy variety [here](https://kubernetes.io/docs/concepts/security/pod-security-standards/).

Add a label to the target namespace to apply a policy, e.g.:
- `security.deckhouse.io/pod-policy=baseline`
- `security.deckhouse.io/pod-policy=restricted`

Here's how you can do so with kubectl:

```bash
kubectl label ns my-namespace security.deckhouse.io/pod-policy=restricted
```

The above command sets the `Restricted` policy for all Pods in the `my-namespace` Namespace.


