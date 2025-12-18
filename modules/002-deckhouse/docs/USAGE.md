---
title: "The deckhouse module: usage"
---

## Setting up the update mode

You can manage DKP updates in the following ways:

- Using the [settings.update](configuration.html#parameters-update) ModuleConfig `deckhouse` parameter;
- Using the [disruptions](../node-manager/cr.html#nodegroup-v1-spec-disruptions) NodeGroup parameters section.

### Update windows configuration

You can configure the time windows when Deckhouse will automatically install updates in the following ways:

- in the [update.windows](configuration.html#parameters-update-windows) parameter of the `deckhouse` ModuleConfig for overall update management;
- in the [disruptions.automatic.windows](../node-manager/cr.html#nodegroup-v1-spec-disruptions-automatic-windows) and [disruptions.rollingUpdate.windows](../node-manager/cr.html#nodegroup-v1-spec-disruptions-rollingupdate-windows) parameters of NodeGroup, for managing disruptive updates.

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

  This means that the corresponding NodeGroup has the parameter [spec.disruptions.approvalMode](../node-manager/cr.html#nodegroup-v1-spec-disruptions-approvalmode) set to `Manual`.

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

## Collect debug info

Read [the FAQ](faq.html#how-to-collect-debug-info) to learn more about collecting debug information.
