---
title: "The deckhouse module: usage"
search: "release.deckhouse.io/approved"
---

## Setting up the update mode

You can manage DKP updates in the following ways:

- Using the [`settings.update`](configuration.html#parameters-update) parameter of the ModuleConfig `deckhouse` resource.
- Using the [`disruptions`](/modules/node-manager/cr.html#nodegroup-v1-spec-disruptions) parameters section of the NodeGroup resource.

### Update windows configuration

You can configure the time windows when Deckhouse will automatically install updates in the following ways:

- For general update management, in the [`update.windows`](configuration.html#parameters-update-windows) parameter of the `deckhouse` ModuleConfig resource.
- For managing disruptive updates, in the [`disruptions.automatic.windows`](/modules/node-manager/cr.html#nodegroup-v1-spec-disruptions-automatic-windows) and [`disruptions.rollingUpdate.windows`](/modules/node-manager/cr.html#nodegroup-v1-spec-disruptions-rollingupdate-windows) parameters of the NodeGroup resource.

An example of setting up two daily update windows — from 8 a.m. to 10 a.m. and from 8 p.m. to 10 p.m. (UTC):

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    releaseChannel: EarlyAccess
    update:
      windows:
        - from: "8:00"
          to: "10:00"
        - from: "20:00"
          to: "22:00"
```

You can also set up updates on certain days, for example, on Tuesdays and Saturdays from 6 p.m. to 7:30 p.m. (UTC):

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    releaseChannel: Stable
    update:
      windows:
        - from: "18:00"
          to: "19:30"
          days:
            - Tue
            - Sat
```

<div id="manual-disruption-update-confirmation"></div>

### Manual update confirmation

Manual confirmation of Deckhouse version updates is provided in the following cases:

- The Deckhouse update confirmation mode is enabled.

  This means that the parameter [settings.update.mode](configuration.html#parameters-update-mode) in the ModuleConfig `deckhouse` is set to `Manual` (confirmation for both patch and minor versions of Deckhouse) or `AutoPatch` (confirmation for the minor version of Deckhouse).
  Run the following command to confirm the update (use the corresponding Deckhouse version):

  ```shell
  d8 k patch DeckhouseRelease <VERSION> --type=merge -p='{"approved": true}'
  ```

- If automatic application of disruptive updates is disabled for a node group.

  This means that the corresponding NodeGroup has the parameter [`spec.disruptions.approvalMode`](/modules/node-manager/cr.html#nodegroup-v1-spec-disruptions-approvalmode) set to `Manual`.

  For updating **each** node in such a group, the node must have `update.node.deckhouse.io/disruption-approved=` annotation.
  Example:

  ```shell
  d8 k annotate node ${NODE_1} update.node.deckhouse.io/disruption-approved=
  ```

### Deckhouse update notifications

In the `Auto` update mode, you can [configure](configuration.html#parameters-update-notification) a webhook call to receive a notification about an upcoming minor Deckhouse version update.

In addition, notifications are generated not only for Deckhouse updates but also for updates of any modules, including their individual updates.
In some cases, the system may initiate the sending of multiple notifications at once (10–20 notifications) at approximately 15-second intervals.

{% alert %}
Notifications are available only in the `Auto` update mode; in the `Manual` mode they are not generated.
{% endalert %}

{% alert %}
Specifying a webhook is optional: if the `update.notification.webhook` parameter is not set but the `update.notification.minimalNotificationTime` parameter is specified, the update will still be postponed for the specified period. In this case, the appearance of a [DeckhouseRelease](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#deckhouserelease) resource in the cluster with the name of the new version can be considered the notification of its availability.
{% endalert %}

Notifications are sent only once for a specific update. If something goes wrong (for example, the webhook receives incorrect data), they will not be resent automatically. To resend the notification, you must delete the corresponding [DeckhouseRelease](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#deckhouserelease) resource.

Example of notification configuration:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    update:
      releaseChannel: Stable
      mode: Auto
      notification:
        webhook: https://release-webhook.mydomain.com
```

After a new minor Deckhouse version appears on the selected release channel, but before it is applied in the cluster, a [POST request](configuration.html#parameters-update-notification-webhook) will be sent to the configured webhook address.

The [minimalNotificationTime](configuration.html#parameters-update-notification-minimalnotificationtime) parameter allows you to postpone the update installation for the specified period, providing time to react to the notification while respecting update windows. If the webhook is unavailable, each failed attempt to send the notification will postpone the update by the same duration, which may lead to the update being deferred indefinitely.

{% alert level="warning" %}
If your webhook returns any status code out of 2xx range, DKP retries sending the notification up to five times with exponential backoff. If all attempts fail, the release is blocked until the webhook becomes available again.
{% endalert %}

For easier error handling and debugging, when returning error codes the webhook should return a JSON response with the following structure:

- `code` — optional internal error code for programmatic handling;
- `message` — a human-readable description of what went wrong.

If the webhook returns a successful HTTP status (2xx), DKP treats the notification as successful regardless of the response body.

Example:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    update:
      releaseChannel: Stable
      mode: Auto
      notification:
        webhook: https://release-webhook.mydomain.com
        minimalNotificationTime: 8h
```

{% offtopic title="Minimal Go webhook example..." %}

```go
package main

import (
  "encoding/json"
  "fmt"
  "log"
  "net/http"
)

// Payload structure Deckhouse sends in POST body.
type WebhookData struct {
  Subject       string            `json:"subject"`
  Version       string            `json:"version"`
  Requirements  map[string]string `json:"requirements,omitempty"`
  ChangelogLink string            `json:"changelogLink,omitempty"`
  ApplyTime     string            `json:"applyTime,omitempty"`
  Message       string            `json:"message"`
}

// Response structure that Deckhouse expects from webhook on error
type ResponseError struct {
  Code    string `json:"code,omitempty"`
  Message string `json:"message"`
}

func handler(w http.ResponseWriter, r *http.Request) {
  if r.Method != http.MethodPost {
    w.WriteHeader(http.StatusMethodNotAllowed)
    return
  }
  defer r.Body.Close()

  var data WebhookData
  if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
    log.Printf("failed to decode payload: %v", err)
    w.WriteHeader(http.StatusInternalServerError)
    return
  }

  // Print payload fields
  log.Printf("subject=%s version=%s applyTime=%s changelog=%s requirements=%v",
    data.Subject, data.Version, data.ApplyTime, data.ChangelogLink, data.Requirements)
  log.Printf("message=%s", data.Message)

  // Example conditional logic: fail intentionally for testing
  if data.Version == "v0.0.0-fail" {
    // Return structured error response with error status code
    errorResp := ResponseError{
      Code:    "TEST_FAILURE",
      Message: "intentional failure for testing",
    }

    w.WriteHeader(http.StatusBadRequest)
    json.NewEncoder(w).Encode(errorResp)
    return
  }

  // Return success response with 2xx status code
  w.WriteHeader(http.StatusOK)
  w.Write([]byte("Notification processed successfully"))
}

func main() {
  mux := http.NewServeMux()
  mux.HandleFunc("/webhook", handler)

  addr := ":8080"
  fmt.Printf("listening on %s, POST to http://localhost%s/webhook\n", addr, addr)
  if err := http.ListenAndServe(addr, mux); err != nil {
    log.Fatal(err)
  }
}
```

{% endofftopic %}

## Reserving the hostnames of the platform web interfaces

Deckhouse publishes its own web interfaces under hostnames rendered from the [`publicDomainTemplate`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate) global parameter: the API, the web interface, Grafana, Dex, the kubeconfig generator and others.
Those hostnames are reserved cluster-wide.
An Ingress, HTTPRoute, GRPCRoute, TLSRoute, ListenerSet or Gateway created outside a namespace labelled `heritage: deckhouse` cannot claim one, and such a request is rejected with a message naming the hostname:

```console
Hostname console.example.com is reserved for Deckhouse platform services
and cannot be claimed outside a system namespace
```

Only creation and modification are checked.
An object that already claims a reserved hostname keeps serving traffic until it is modified.

### What is reserved

The reserved set is controlled by the [`settings.reservedPublicHosts.mode`](configuration.html#parameters-reservedpublichosts-mode) parameter of the ModuleConfig `deckhouse` resource:

- `Template` (the default) — every hostname the template can render.
  With the template `%s.example.com` that is any single-label name under `example.com`: both `console.example.com`, which the platform publishes, and `shop.example.com`, which no module serves yet.
  A module never has to ask to be covered, and a hostname is reserved before anything starts serving it.
- `List` — only the hostnames of the services Deckhouse knows it publishes.
  A narrower reservation, but a public domain that Deckhouse learns to publish later stays unreserved until the list catches up.

Under `Template` the wildcard form of the domain, `*.example.com`, is reserved as well: a workload holding it would shadow every platform hostname at once.
The wildcard is only reserved where the template's `%s` is the whole first label — with `kube-%s.example.com` neither `kube-*.example.com` nor `*.example.com` is reserved.

A hostname one label deeper is never affected, so ecosystem applications are out of scope: the [`applications.publicDomainTemplate`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-applications-publicdomaintemplate) global parameter puts the application name and its namespace in two different labels.

### Giving a hostname back to a workload

Under `Template` mode the reservation covers more than the platform serves, so a workload that legitimately needs such a hostname has to be named.
There are three ways to do it:

1. Free a single hostname with the [`settings.reservedPublicHosts.excludedServices`](configuration.html#parameters-reservedpublichosts-excludedservices) parameter.
   It takes the service name, the part the template substitutes, not the whole hostname: `grafana` frees `grafana.example.com`.
1. Exempt a whole namespace with the `security.deckhouse.io/reserved-hosts-bypass: "true"` label.
   The label is set on the namespace, not on the object, so a tenant confined to their own namespaces cannot set it.
1. Return to the previous reservation with `mode: List`.

An example that frees the hostname of Grafana and additionally reserves one the template cannot render:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    reservedPublicHosts:
      mode: Template
      excludedServices:
        - grafana
      additionalHosts:
        - admin.corp.example.org
```

The [`settings.reservedPublicHosts.additionalHosts`](configuration.html#parameters-reservedpublichosts-additionalhosts) parameter always reserves, whatever `excludedServices` says, so the two parameters cannot contradict each other.

{% alert level="warning" %}
The wildcard form cannot be freed with `excludedServices`, which takes a service name.
A workload that serves `*.example.com` needs the namespace label or `mode: List`.
{% endalert %}

### Enabling the reservation in a cluster that already has workloads

When the reservation starts to apply, the hostnames workloads already serve are recorded once and stay allowed.
Without that, their next modification would be rejected.

The record is kept in the `d8-reserved-public-hosts` ConfigMap of the `d8-system` namespace, in the `grandfatheredHosts` key, separately from the hostnames freed by hand.
It is taken once, on the first convergence where `Template` mode is in force, and never retaken: otherwise the reservation could be defeated by claiming a hostname and waiting for the next snapshot to legitimise it.

To stop allowing a recorded hostname once the workload behind it is gone, remove the entry from the key.
An entry is brought to the spelling the reservation compares — lowercase and without a trailing dot — so editing the key by hand cannot stop the module from converging.
An entry that is not a hostname even then, a pasted URL for example, is dropped from the record, and the object claiming it is rejected on its next modification.

The wildcard form is the exception: it is not recorded, because allowing it would leave every hostname the template renders shadowed, including the ones Deckhouse publishes later.
A workload serving the wildcard over the platform domain needs the namespace label or `mode: List`, set before the upgrade.

### Finding out what is reserved and why

The same ConfigMap answers what the reservation covers in this cluster:

| Key | Contents |
| --- | --- |
| `mode` | The reservation in force. Reads `List` if the value of `publicDomainTemplate` never went through its schema |
| `hostPattern` | The set of hostnames the template can render, as a regular expression. Empty under `List` |
| `hosts` | The hostnames matched exactly: `additionalHosts` and the wildcard form |
| `allowedHosts` | What is allowed back out of the pattern: the hostnames freed by `excludedServices` and the recorded ones |
| `excludedHosts` | The hostnames `excludedServices` freed |
| `grandfatheredHosts` | The hostnames recorded when the reservation started to apply |
| `platformHosts` | The hostnames of the services Deckhouse publishes today. Informational under both modes |
| `unknownExcludedServices` | The names from `excludedServices` that no module publishes. The place to look when a hostname stayed reserved: the name is probably misspelled |

To read it, run:

```shell
d8 k -n d8-system get configmap d8-reserved-public-hosts -o yaml
```

## Collect debug info

Read [the FAQ](faq.html#how-to-collect-debug-info) to learn more about collecting debug information.
