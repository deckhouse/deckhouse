# kube-api-rewriter troubleshooting notes

## KubeVirt resource dissapears just after creation, e.g. VMI

**Symptom:** Only KVVM is created after VM creation. KVVMI gets deleted immediately after creation.

**Cause:** The resource is being deleted by Kubernetes because the apiGroup and kind fields in ownerReferences are not rewritten properly and remain as the original values. A resource with the original apiGroup/kind is unknown to the cluster, which leads to the deletion of the VirtualMachineInstance.

**Detect:** This situation can be observed via auditing in "RequestResponse" mode for the disappearing resource kind, for example:

```
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
- level: RequestResponse
  resources:
  - group: "internal.virtualization.deckhouse.io"
    resources:
    - internalvirtualizationvirtualmachineinstances
      verbs:
  - create
  - update
  - patch
  - delete
  - deletecollection
- level: Metadata
  resources:
  - group: "internal.virtualization.deckhouse.io"
    verbs:
  - delete
  - update
  - patch
  - deletecollection
```


## KubeVirt resource stuck in Terminating state

**Symptom:** Delete VM, the KVVMI is deleted, but the KVVM gets stuck in Terminating state and does not get removed. The virtualization-controller keeps waiting indefinitely for the VM to disappear.

**Cause:** Wrong rewrite for PATCH request from mutating webhook in virt-api or from virt-controller. The mutating webhook in virt-api may have incorrectly applied labels or failed to remove finalizers.

**Detect:** Check the logs of the 'proxy' container ('-c proxy') for PATCH and DELETE requests related to this KVVM. You should check the logs of both virt-api and virt-controller.

**Extra:** Additionally, there may be a more complex issue that is harder to diagnose. You need to find the watch subscription for KVVM in the logs and compare rewritten labelSelector with the labels on KVVM. If the labels and selector do not match, the virt-controller loses track of the KVVM and fails to update it.


## KubeVirt resource is not updated as expected

**Symptom:** Original kubevirt.io labels or a finalizer appear on the KVVM.

**Cause:** Wrong rewrite of UPDATE/PATCH requests from virt-controller.

**Detect:** Enable Debug logging for the proxy and monitor how the UPDATE/PATCH requests are rewritten.
The logs show the diff between the request from virt-controller and the request after rewriting.
The difficulty here is that the diff only shows changes and does not display fields without changes.
Therefore, if metadata.labels do not appear in the diff but labels are expected to be rewritten, it indicates issues with label rewriting.

In the future, we might find a way to highlight this more clearly.


## Continuous reconcile with no success in virt-operator

**Symptom:** Indefinite reconcile of kubevirts/config resource in virt-operator. No message "All KubeVirts resources are ready".

**Cause:** Wrong rewrites.

**Detect:** Check virt-operator logs, analyze what should be installed and what rewrites are made by proxy.


## Errors in virt-operator logs: virt-api webhook denies requests

**Cause:** Wrong rewrites of AdmissionReview requests from the API server.

**Detect:** Analyze deploy/virt-api logs.

```
kubectl -n d8-virtualization logs deploy/virt-api -c virt-api
kubectl -n d8-virtualization logs deploy/virt-api -c proxy
```


## Error "409 confict" in virt-operator/virt-controller logs

It is a normal situation when different controllers update one resource.
It should stop after repeated reconciles.


## Error "Status 422 Unprocessable Entity"

**Cause 1:** Wrong rewrites of JSON patch leads to wrong value for "op":"test".

**Detect:** Grep logs for this error. It may appear in virt-operator and virt-controller.
Search for PATCH rewrites. Analyze diff and resource state in cluster to detect problems in rewrite process.

**Cause 2:** The object in the cluster contains both original and rewritten labels. 
After restoration, it results in two sets of original labels being combined into one.
Consequently, the subsequent PATCH after renaming no longer contains the original labels,
and the op:test operation cannot succeed.

This issue was resolved by adding a prefix to the original labels during restoration.
When renaming, the prefix is removed, thus correctly restoring and renaming both
the original and renamed labels, annotations, and finalizers.

For example, this can occur with a Node when both the original KubeVirt and our KubeVirt are running simultaneously.

**Detect:** Check debug logs for adding and removing safe prefix.


## Error: virt-operator trying to send message larger than max

**Symptom:** rpc error: code = ResourceExhausted desc = trying to send message larger than max (2950060 vs. 2097152)

**Cause:** This happens when virt-operator can't update CRD and trying to dump its full content to the kubevirts/config status. CRD full dump is huge, so error is occured.

**Detect:** Check rewrites in proxy logs and analyze why CRD is not updated.


## Error: Precondition failed: UID in precondition: NNN, UID in object meta: MMM

**Symptom:**

```
"msg": "...Precondition failed: UID in precondition: 7412b2e0-b13e-4865-9726-4e362a946ed4, UID in object meta: afa1dc70-1c22-477f-8655-5bafaffe8594"
```

**Cause:** It is a "cache poisoning" introduced by wrong rewrites.

The issue lies in the duplication of CRDs after restoring the CRDList.

virt-operator/virt-controller populate their resource cache by performing a List operation and subscribing to the CRD watch, and the restored resources arrive in the cache. If both the original and renamed resources exist in the cluster, the cache ends up containing two CRDs with the same name but different UIDs. Which resource will be retained in the cache is not determined.

Simultaneously, virt-operator/virt-controller performs a Get operation for the CRD that needs to be updated.

Next, the virt-operator/virt-controller changes the retrieved resource and merge it with the resource from the cache. If cache has wrong UID, this merge will lead to an error.

This situation fixed by adding Excludes to rules definitions. "Excludes" field defines what should be excluded from lists returned in discovery and GET responses.

**Detect and fix:** Check rewrites for resource by debug logs. Add resource to Excludes if needed.


## Error: Precondition failed: UID in precondition: NNN, UID in object meta: <empty>

**Symptom:**

```
"msg": "... Precondition failed: UID in precondition: 185e2029-2afa-47d4-92dd-be454a6d7ffb, UID in object meta:"
```

**Cause:** A variation of the previous problem: controller can GET resource, but resource is not in cache managed by the watcher.

Wrong rewrite of the labelSelector in watch request URI can lead to this situation.

**Detect:** Check debug logs, analyze URI rewrite for watch requests. Analyze labels on watched resources in cluster.

## Useful commands

### Enable debug logs

```
kubectl patch --type merge -p '{"spec":{"settings":{"logLevel":"debug"}}}' mc virtualization
```

### Display logs from proxy

```
kubectl -n d8-virtualization logs deploy/virt-operator -c proxy | less
```

### Script to apply audit policy in deckhouse

```shell
#!/bin/bash

POLICY_BASE64=$(base64 -w0 audit-policy.yaml)

cat <<EOF | POLICY_BASE64=$POLICY_BASE64 envsubst | kubectl -n kube-system apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: audit-policy
  namespace: kube-system
data:
  audit-policy.yaml: $POLICY_BASE64
EOF
```
