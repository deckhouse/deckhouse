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

DKP publishes its own web interfaces under hostnames rendered from the [`publicDomainTemplate`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate) global parameter: the API, the web interface, Grafana, Dex, the kubeconfig generator and others.
Those hostnames are reserved cluster-wide.
An Ingress, HTTPRoute, GRPCRoute, TLSRoute, ListenerSet or Gateway resource created outside a namespace labeled `heritage: deckhouse` cannot claim such a name, and such a request is rejected with a message naming the hostname:

```console
Hostname console.example.com is reserved for Deckhouse platform services
and cannot be claimed outside a system namespace
```

Only resource creation and modification are checked.
An object that already claims a reserved hostname keeps serving traffic until it is modified.

### Reserved hostnames

The reserved set is controlled by the `settings.reservedPublicHosts.mode` parameter of the ModuleConfig `deckhouse`:

- `Template` (by default): Every hostname the template can render are reserved.
- `List`: Only the hostnames of the services DKP knows it publishes.

For details on each reservation method, refer to the [parameter description](configuration.html#parameters-reservedpublichosts-mode).

### Excluding a name from reservation

Under `Template` mode, the reservation covers not only the hostnames used by DKP but also others matching the template. A workload that requires such a hostname can be excluded from reservation in one of the following ways:

- Free a single hostname with the [`settings.reservedPublicHosts.excludedServices`](configuration.html#parameters-reservedpublichosts-excludedservices) parameter.

  The parameter includes not the entire hostname, only the service name (the value included in the template). For example, for the `%s.example.com` template, the `grafana` value excludes `grafana.example.com` from reservation.

- Exclude a whole namespace with the `security.deckhouse.io/reserved-hosts-bypass: "true"` label.

  The label is set on the namespace, not on the object, so a tenant who is allowed to manage resources only in their own namespaces can't set it.

- Return to the previous reservation by setting `mode: List`.

The following is an example that frees `grafana.example.com` from reservation and additionally reserves `admin.corp.example.org` that doesn't match the template:

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

The [`settings.reservedPublicHosts.additionalHosts`](configuration.html#parameters-reservedpublichosts-additionalhosts) parameter lets you reserve additional hostnames regardless of the `excludedServices` value.

{% alert level="warning" %}
The `excludedServices` parameter accepts service names only, therefore it can't be used to exclude a wildcard name from reservation.

To allow a workload to use a wildcard name (such as `*.example.com`), disable the checking for its namespace using the above-mentioned label or use the `List` mode.
{% endalert %}

### Excluding the shared Gateway

The Gateway to which DKP module ListenerSet resources are attached is automatically excluded from reservation, regardless of the hostnames specified in its listeners.

The following Gateway is considered the shared Gateway:

- The Gateway specified in the global [`gatewayAPIGateway`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-gatewayapigateway) parameter.
- If the parameter is not set, the Gateway specified in the `default-gateway` ConfigMap in the `d8-alb` namespace.

The exclusion applies by name and namespace and only to the Gateway resource. It does not apply to other resources using the same hostnames, such as Ingress, HTTPRoute, or another Gateway.

This exclusion is necessary because such a Gateway typically has a listener with a wildcard hostname and wildcard certificate and is often located in a dedicated namespace without the `heritage: deckhouse` label.
Without the exclusion, the Gateway could not be modified after hostname reservation is enabled, even though the attached DKP routes continue to depend on it.

Any other Gateway with a `*.example.com` listener is rejected, even if that hostname was already in use when reservation was enabled.
To use such a Gateway, add the `security.deckhouse.io/reserved-hosts-bypass: "true"` label to its namespace or enable `List` mode.

If the shared Gateway is neither specified in the `gatewayAPIGateway` parameter nor defined through the `default-gateway` ConfigMap, no automatic exclusion is applied.

### Reservation with existing applications in the cluster

When hostname reservation takes effect, hostnames already used by applications are automatically added to the list of allowed hostnames. Otherwise, the next modification of the corresponding resources would be rejected.

The list is stored in the `grandfatheredHosts` key of the `d8-reserved-public-hosts` ConfigMap in the `d8-system` namespace, separately from hostnames manually excluded from reservation.

The list is created only when `Template` mode is enabled for the first time and is not updated afterward. This prevents reservation from being bypassed: an application cannot claim a reserved hostname after reservation has been enabled and then have that hostname allowed through subsequent regeneration of the list.

If an application that used a hostname allowed this way is removed and the hostname is no longer needed, remove the corresponding entry from `grandfatheredHosts`.

Before being stored, hostnames are converted to lowercase and any trailing dot is removed. Invalid values, such as a URL instead of a hostname, are ignored. If a resource uses the corresponding reserved hostname, its next modification will be rejected.

Wildcard hostnames, such as `*.example.com`, are not added to `grandfatheredHosts`. Otherwise, an application could use any hostname matching the template, including hostnames of services that DKP may publish in the future.
To allow an application to use a wildcard hostname in the platform domain, add the `security.deckhouse.io/reserved-hosts-bypass: "true"` label to its namespace or enable `List` mode.

This restriction does not apply to the Gateway to which DKP is attached. This Gateway is automatically excluded from the check.

Switching from `Template` mode to `List` mode and back does not regenerate `grandfatheredHosts`. Therefore, hostnames that applications start using while `List` mode is enabled are not added to the list of allowed hostnames after switching back to `Template` mode. When the corresponding resources are modified, these hostnames are checked according to the standard reservation rules.

### Checking reservation status

Information about the current reservation status is stored in the `d8-reserved-public-hosts` ConfigMap in the `d8-system` namespace.

To view the ConfigMap, run the following command:

```shell
d8 k -n d8-system get configmap d8-reserved-public-hosts -o yaml
```

The ConfigMap contains the following keys:

| Key | Description |
| --- | --- |
| `mode` | Current reservation mode. If `publicDomainTemplate` does not match the schema, the value is `List` |
| `hostPattern` | Regular expression describing the hostnames that can be generated from the template. The value is empty in `List` mode |
| `hosts` | Hostnames matched exactly. In `Template` mode, contains the values from `additionalHosts` and the wildcard domain name; in `List` mode, contains the hostnames of platform services |
| `allowedHosts` | Hostnames excluded from matching against `hostPattern`: hostnames from `excludedServices` and `grandfatheredHosts` |
| `excludedHosts` | Hostnames excluded from reservation using `excludedServices` |
| `grandfatheredHosts` | Hostnames that were already used by applications when reservation was enabled and were automatically added to the list of allowed hostnames |
| `sharedGateway` | Gateway to which DKP is attached, in the `<NAMESPACE>/<NAME>` format. The value is empty if no such Gateway is configured or detected |
| `platformHosts` | Hostnames of services published by DKP. Used as a reference in both reservation modes |
| `unknownExcludedServices` | Names from `excludedServices` that are not published by any module. A value in this key most likely indicates a typo in the service name |

## Collect debug info

Read [the FAQ](faq.html#how-to-collect-debug-info) to learn more about collecting debug information.
