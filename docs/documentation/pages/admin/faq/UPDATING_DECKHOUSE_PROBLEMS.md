---
title: What to do if there are problems updating DKP?
permalink: en/faq-common/updating-dkp-problems.html
---

## Deckhouse Kubernetes Platform update fails, one or more Deckhouse pods are in an unworkable state

If the Deckhouse Kubernetes Platform update fails, one or more Deckhouse pods in the `d8-system` namespace are in an unworkable state. Perform the following steps:

1. Check the Deckhouse logs using the command:

   ```shell
   d8 k -n d8-system logs -f -l app=deckhouse | jq -Rr 'fromjson? | .msg'
   ```

   If there are any problems, information about them will be included in the output. When analyzing logs, pay special attention to warnings (`WARNING`) and error messages (`ERROR`).

1. Check Deckhouse events using the command:

   ```shell
   d8 k -n d8-system describe po -l app=deckhouse | awk '
   /^Name:/ { 
       pod = $2; 
       print "=== " pod " ==="; 
       in_events = 0 
   }
   /Events:/ { 
       in_events = 1; 
       next 
   }
   in_events && /^$/ { 
       in_events = 0; 
       print "---" 
   }
   in_events && !/^Events:/ { 
       print $0 
   }
   ' | sed '/^---$/N;/^\n$/D'
   ```

   Pod events contain key information about problems (e.g., planning errors, image loading errors, etc.). When analyzing events, pay special attention to those with the `Warning` type.

   Example output:

   ```console
   Type     Reason     Age                      From     Message
   ----     ------     ----                     ----     -------
   Warning  Unhealthy  4m44s (x1918 over 154m)  kubelet  Readiness probe failed: HTTP probe failed with statuscode: 500
   ```

## DKP update is stuck in the Release is suspended status

The status `Release is suspended` indicates that it has been postponed and is currently unavailable (not recommended) for installation. In this case, it is recommended to remain on the latest available release or on the one currently installed (it will have the status `Deployed`).

To view the list of releases, use the command:

```shell
d8 k get deckhousereleases.deckhouse.io
```

Example output:

```console
NAME       PHASE        TRANSITIONTIME   MESSAGE
v1.69.13   Skipped      3h46m
v1.69.14   Skipped      3h46m
v1.69.15   Skipped      3h46m
v1.69.16   Superseded   160m
v1.70.12   Suspended    49d              Release is suspended
v1.70.13   Skipped      36d
v1.70.14   Skipped      34d
v1.70.15   Skipped      28d
v1.70.16   Skipped      19d
v1.70.17   Deployed     160m
v1.71.3    Suspended    14d              Release is suspended
```
