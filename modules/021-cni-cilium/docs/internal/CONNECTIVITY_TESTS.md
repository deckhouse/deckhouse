# Cilium connectivity tests

## How to run

```bash
cilium connectivity test -n d8-cni-cilium --single-node --hubble=false --agent-daemonset-name=agent --agent-pod-selector app=agent --assume-cilium-version 1.12.7
```

## Openstack

### bpfLBMode=Hybrid + tunnelMode=Disabled

Cilium settings:

```yaml
bpfLBMode: Hybrid
tunnelMode: Disabled
```

Problems with using this mode:

- It is impossible to access nodes (curl) from the Pod network using node internal IP addresses (except the node where the Pod is located).

An example of the results of the connectivity tests:

```text
📋 Test Report
❌ 1/32 tests failed (10/246 actions), 0 tests skipped, 0 scenarios skipped:
Test [no-policies-extra]:
  ❌ no-policies-extra/pod-to-remote-nodeport/curl-0: cilium-test/client-784c67ffc4-njnxv (10.111.6.185) -> cilium-test/echo-same-node (echo-same-node:8080)
  ❌ no-policies-extra/pod-to-remote-nodeport/curl-1: cilium-test/client-784c67ffc4-njnxv (10.111.6.185) -> cilium-test/echo-same-node (echo-same-node:8080)
  ❌ no-policies-extra/pod-to-remote-nodeport/curl-2: cilium-test/client-784c67ffc4-njnxv (10.111.6.185) -> cilium-test/echo-same-node (echo-same-node:8080)
  ❌ no-policies-extra/pod-to-remote-nodeport/curl-3: cilium-test/client-784c67ffc4-njnxv (10.111.6.185) -> cilium-test/echo-same-node (echo-same-node:8080)
  ❌ no-policies-extra/pod-to-remote-nodeport/curl-4: cilium-test/client-784c67ffc4-njnxv (10.111.6.185) -> cilium-test/echo-same-node (echo-same-node:8080)
  ❌ no-policies-extra/pod-to-remote-nodeport/curl-5: cilium-test/client2-67754cb6fb-h8zh6 (10.111.6.72) -> cilium-test/echo-same-node (echo-same-node:8080)
  ❌ no-policies-extra/pod-to-remote-nodeport/curl-6: cilium-test/client2-67754cb6fb-h8zh6 (10.111.6.72) -> cilium-test/echo-same-node (echo-same-node:8080)
  ❌ no-policies-extra/pod-to-remote-nodeport/curl-7: cilium-test/client2-67754cb6fb-h8zh6 (10.111.6.72) -> cilium-test/echo-same-node (echo-same-node:8080)
  ❌ no-policies-extra/pod-to-remote-nodeport/curl-8: cilium-test/client2-67754cb6fb-h8zh6 (10.111.6.72) -> cilium-test/echo-same-node (echo-same-node:8080)
  ❌ no-policies-extra/pod-to-remote-nodeport/curl-9: cilium-test/client2-67754cb6fb-h8zh6 (10.111.6.72) -> cilium-test/echo-same-node (echo-same-node:8080)
```

Cilium settings:

```yaml
bpfLBMode: SNAT
```

An example of the results of the connectivity tests:

```text
✅ All 32 tests (246 actions) successful, 0 tests skipped, 0 scenarios skipped.
```

## Yandex

### bpfLBMode=Hybrid + tunnelMode=Disabled

Cilium settings:

```yaml
bpfLBMode: Hybrid
tunnelMode: Disabled
```

Problems with using this mode:

- It is impossible to access nodes (curl, ping) from the Pod network using the internal IP addresses of nodes (except the node where the Pod is located).

An example of the results of the connectivity tests:

Connection tests do not run because the internal node IP is not available from the Pod.

```text
⌛ [k8s-dev-cluster] Waiting for NodePort 10.233.32.10:30860 (cilium-test/echo-same-node) to become ready...
connectivity test failed: timeout reached waiting for NodePort 10.233.32.10:30860 (cilium-test/echo-same-node) (last error: context deadline exceeded)
```

### bpfLBMode=SNAT + tunnelMode=Disabled

Cilium settings:

```yaml
bpfLBMode: SNAT
tunnelMode: Disabled
```

Problems with using this mode:

- It is impossible to access nodes (curl, ping) from the Pod network using external IP addresses of nodes (except the node where the Pod is located).

An example of the results of the connectivity tests:

```text
📋 Test Report
❌ 4/32 tests failed (16/195 actions), 0 tests skipped, 0 scenarios skipped:
Test [no-policies]:
  ❌ no-policies/pod-to-host/ping-1: cilium-test/client-784c67ffc4-m4tth (10.111.60.213) -> 84.201.158.188 (84.201.158.188:0)
  ❌ no-policies/pod-to-host/ping-3: cilium-test/client-784c67ffc4-m4tth (10.111.60.213) -> 51.250.79.66 (51.250.79.66:0)
  ❌ no-policies/pod-to-host/ping-7: cilium-test/client2-67754cb6fb-bjvrk (10.111.60.94) -> 84.201.158.188 (84.201.158.188:0)
  ❌ no-policies/pod-to-host/ping-9: cilium-test/client2-67754cb6fb-bjvrk (10.111.60.94) -> 51.250.79.66 (51.250.79.66:0)
Test [no-policies-extra]:
  ❌ no-policies-extra/pod-to-remote-nodeport/curl-0: cilium-test/client-784c67ffc4-m4tth (10.111.60.213) -> cilium-test/echo-same-node (echo-same-node:8080)
  ❌ no-policies-extra/pod-to-remote-nodeport/curl-1: cilium-test/client-784c67ffc4-m4tth (10.111.60.213) -> cilium-test/echo-same-node (echo-same-node:8080)
  ❌ no-policies-extra/pod-to-remote-nodeport/curl-2: cilium-test/client2-67754cb6fb-bjvrk (10.111.60.94) -> cilium-test/echo-same-node (echo-same-node:8080)
  ❌ no-policies-extra/pod-to-remote-nodeport/curl-3: cilium-test/client2-67754cb6fb-bjvrk (10.111.60.94) -> cilium-test/echo-same-node (echo-same-node:8080)
Test [allow-all-except-world]:
  ❌ allow-all-except-world/pod-to-host/ping-1: cilium-test/client-784c67ffc4-m4tth (10.111.60.213) -> 84.201.158.188 (84.201.158.188:0)
  ❌ allow-all-except-world/pod-to-host/ping-3: cilium-test/client-784c67ffc4-m4tth (10.111.60.213) -> 51.250.79.66 (51.250.79.66:0)
  ❌ allow-all-except-world/pod-to-host/ping-7: cilium-test/client2-67754cb6fb-bjvrk (10.111.60.94) -> 84.201.158.188 (84.201.158.188:0)
  ❌ allow-all-except-world/pod-to-host/ping-9: cilium-test/client2-67754cb6fb-bjvrk (10.111.60.94) -> 51.250.79.66 (51.250.79.66:0)
Test [host-entity]:
  ❌ host-entity/pod-to-host/ping-1: cilium-test/client-784c67ffc4-m4tth (10.111.60.213) -> 84.201.158.188 (84.201.158.188:0)
  ❌ host-entity/pod-to-host/ping-3: cilium-test/client-784c67ffc4-m4tth (10.111.60.213) -> 51.250.79.66 (51.250.79.66:0)
  ❌ host-entity/pod-to-host/ping-7: cilium-test/client2-67754cb6fb-bjvrk (10.111.60.94) -> 84.201.158.188 (84.201.158.188:0)
  ❌ host-entity/pod-to-host/ping-9: cilium-test/client2-67754cb6fb-bjvrk (10.111.60.94) -> 51.250.79.66 (51.250.79.66:0)
connectivity test failed: 4 tests failed
```

How to fix it:

```bash
kubectl patch -n d8-cni-cilium cm cilium-config --patch='{"data":{"enable-bpf-masquerade":"false","enable-ipv4-egress-gateway":"false", "install-no-conntrack-iptables-rules":"false"}}'
```

An example of the results of the connectivity tests after patch:

```text
✅ All 32 tests (195 actions) successful, 0 tests skipped, 0 scenarios skipped.
```

### bpfLBMode=SNAT + tunnelMode=VXLAN

Cilium settings:

```yaml
bpfLBMode: SNAT
tunnelMode: VXLAN
```

Problems with using this mode:

- It is impossible to access nodes (curl, ping) from the Pod network using external IP addresses of nodes (except the node where the Pod is located).

An example of the results of the connectivity tests:

```text
📋 Test Report
❌ 4/32 tests failed (20/273 actions), 0 tests skipped, 0 scenarios skipped:
Test [no-policies]:
  ❌ no-policies/pod-to-host/ping-1: cilium-test/client-784c67ffc4-6jkkz (10.111.60.83) -> 84.201.158.188 (84.201.158.188:0)
  ❌ no-policies/pod-to-host/ping-3: cilium-test/client-784c67ffc4-6jkkz (10.111.60.83) -> 51.250.79.66 (51.250.79.66:0)
  ❌ no-policies/pod-to-host/ping-7: cilium-test/client2-67754cb6fb-nvfjp (10.111.60.133) -> 84.201.158.188 (84.201.158.188:0)
  ❌ no-policies/pod-to-host/ping-9: cilium-test/client2-67754cb6fb-nvfjp (10.111.60.133) -> 51.250.79.66 (51.250.79.66:0)
Test [no-policies-extra]:
  ❌ no-policies-extra/pod-to-remote-nodeport/curl-0: cilium-test/client-784c67ffc4-6jkkz (10.111.60.83) -> cilium-test/echo-other-node (echo-other-node:8080)
  ❌ no-policies-extra/pod-to-remote-nodeport/curl-1: cilium-test/client-784c67ffc4-6jkkz (10.111.60.83) -> cilium-test/echo-other-node (echo-other-node:8080)
  ❌ no-policies-extra/pod-to-remote-nodeport/curl-2: cilium-test/client-784c67ffc4-6jkkz (10.111.60.83) -> cilium-test/echo-same-node (echo-same-node:8080)
  ❌ no-policies-extra/pod-to-remote-nodeport/curl-3: cilium-test/client-784c67ffc4-6jkkz (10.111.60.83) -> cilium-test/echo-same-node (echo-same-node:8080)
  ❌ no-policies-extra/pod-to-remote-nodeport/curl-4: cilium-test/client2-67754cb6fb-nvfjp (10.111.60.133) -> cilium-test/echo-other-node (echo-other-node:8080)
  ❌ no-policies-extra/pod-to-remote-nodeport/curl-5: cilium-test/client2-67754cb6fb-nvfjp (10.111.60.133) -> cilium-test/echo-other-node (echo-other-node:8080)
  ❌ no-policies-extra/pod-to-remote-nodeport/curl-6: cilium-test/client2-67754cb6fb-nvfjp (10.111.60.133) -> cilium-test/echo-same-node (echo-same-node:8080)
  ❌ no-policies-extra/pod-to-remote-nodeport/curl-7: cilium-test/client2-67754cb6fb-nvfjp (10.111.60.133) -> cilium-test/echo-same-node (echo-same-node:8080)
Test [allow-all-except-world]:
  ❌ allow-all-except-world/pod-to-host/ping-1: cilium-test/client-784c67ffc4-6jkkz (10.111.60.83) -> 84.201.158.188 (84.201.158.188:0)
  ❌ allow-all-except-world/pod-to-host/ping-3: cilium-test/client-784c67ffc4-6jkkz (10.111.60.83) -> 51.250.79.66 (51.250.79.66:0)
  ❌ allow-all-except-world/pod-to-host/ping-7: cilium-test/client2-67754cb6fb-nvfjp (10.111.60.133) -> 51.250.79.66 (51.250.79.66:0)
  ❌ allow-all-except-world/pod-to-host/ping-11: cilium-test/client2-67754cb6fb-nvfjp (10.111.60.133) -> 84.201.158.188 (84.201.158.188:0)
Test [host-entity]:
  ❌ host-entity/pod-to-host/ping-3: cilium-test/client-784c67ffc4-6jkkz (10.111.60.83) -> 84.201.158.188 (84.201.158.188:0)
  ❌ host-entity/pod-to-host/ping-5: cilium-test/client-784c67ffc4-6jkkz (10.111.60.83) -> 51.250.79.66 (51.250.79.66:0)
  ❌ host-entity/pod-to-host/ping-7: cilium-test/client2-67754cb6fb-nvfjp (10.111.60.133) -> 51.250.79.66 (51.250.79.66:0)
  ❌ host-entity/pod-to-host/ping-11: cilium-test/client2-67754cb6fb-nvfjp (10.111.60.133) -> 84.201.158.188 (84.201.158.188:0)

```

### bpfLBMode=Hybrid + tunnelMode=VXLAN

Cilium settings:

```yaml
bpfLBMode: Hybrid
tunnelMode: VXLAN
```

Problems with using this mode:

- It is impossible to access nodes (ping, curl) from the Pod network using external IP addresses of nodes (except the node where the Pod is located).

An example of the results of the connectivity tests:

```text
📋 Test Report
❌ 5/32 tests failed (21/273 actions), 0 tests skipped, 0 scenarios skipped:
Test [no-policies]:
  ❌ no-policies/pod-to-host/ping-1: cilium-test/client-784c67ffc4-6jkkz (10.111.60.83) -> 84.201.158.188 (84.201.158.188:0)
  ❌ no-policies/pod-to-host/ping-3: cilium-test/client-784c67ffc4-6jkkz (10.111.60.83) -> 51.250.79.66 (51.250.79.66:0)
  ❌ no-policies/pod-to-host/ping-7: cilium-test/client2-67754cb6fb-nvfjp (10.111.60.133) -> 84.201.158.188 (84.201.158.188:0)
  ❌ no-policies/pod-to-host/ping-9: cilium-test/client2-67754cb6fb-nvfjp (10.111.60.133) -> 51.250.79.66 (51.250.79.66:0)
Test [no-policies-extra]:
  ❌ no-policies-extra/pod-to-remote-nodeport/curl-0: cilium-test/client2-67754cb6fb-nvfjp (10.111.60.133) -> cilium-test/echo-other-node (echo-other-node:8080)
  ❌ no-policies-extra/pod-to-remote-nodeport/curl-1: cilium-test/client2-67754cb6fb-nvfjp (10.111.60.133) -> cilium-test/echo-other-node (echo-other-node:8080)
  ❌ no-policies-extra/pod-to-remote-nodeport/curl-2: cilium-test/client2-67754cb6fb-nvfjp (10.111.60.133) -> cilium-test/echo-same-node (echo-same-node:8080)
  ❌ no-policies-extra/pod-to-remote-nodeport/curl-3: cilium-test/client2-67754cb6fb-nvfjp (10.111.60.133) -> cilium-test/echo-same-node (echo-same-node:8080)
  ❌ no-policies-extra/pod-to-remote-nodeport/curl-4: cilium-test/client-784c67ffc4-6jkkz (10.111.60.83) -> cilium-test/echo-other-node (echo-other-node:8080)
  ❌ no-policies-extra/pod-to-remote-nodeport/curl-5: cilium-test/client-784c67ffc4-6jkkz (10.111.60.83) -> cilium-test/echo-other-node (echo-other-node:8080)
  ❌ no-policies-extra/pod-to-remote-nodeport/curl-6: cilium-test/client-784c67ffc4-6jkkz (10.111.60.83) -> cilium-test/echo-same-node (echo-same-node:8080)
  ❌ no-policies-extra/pod-to-remote-nodeport/curl-7: cilium-test/client-784c67ffc4-6jkkz (10.111.60.83) -> cilium-test/echo-same-node (echo-same-node:8080)
Test [allow-all-except-world]:
  ❌ allow-all-except-world/pod-to-host/ping-1: cilium-test/client-784c67ffc4-6jkkz (10.111.60.83) -> 84.201.158.188 (84.201.158.188:0)
  ❌ allow-all-except-world/pod-to-host/ping-3: cilium-test/client-784c67ffc4-6jkkz (10.111.60.83) -> 51.250.79.66 (51.250.79.66:0)
  ❌ allow-all-except-world/pod-to-host/ping-7: cilium-test/client2-67754cb6fb-nvfjp (10.111.60.133) -> 84.201.158.188 (84.201.158.188:0)
  ❌ allow-all-except-world/pod-to-host/ping-9: cilium-test/client2-67754cb6fb-nvfjp (10.111.60.133) -> 51.250.79.66 (51.250.79.66:0)
Test [host-entity]:
  ❌ host-entity/pod-to-host/ping-1: cilium-test/client-784c67ffc4-6jkkz (10.111.60.83) -> 84.201.158.188 (84.201.158.188:0)
  ❌ host-entity/pod-to-host/ping-3: cilium-test/client-784c67ffc4-6jkkz (10.111.60.83) -> 51.250.79.66 (51.250.79.66:0)
  ❌ host-entity/pod-to-host/ping-7: cilium-test/client2-67754cb6fb-nvfjp (10.111.60.133) -> 51.250.79.66 (51.250.79.66:0)
  ❌ host-entity/pod-to-host/ping-11: cilium-test/client2-67754cb6fb-nvfjp (10.111.60.133) -> 84.201.158.188 (84.201.158.188:0)
Test [client-ingress-from-other-client-icmp-deny]:
  ❌ client-ingress-from-other-client-icmp-deny/pod-to-pod/curl-2: cilium-test/client2-67754cb6fb-nvfjp (10.111.60.133) -> cilium-test/echo-other-node-5488b85f95-s7cgl (10.111.2.199:8080)
connectivity test failed: 5 tests failed
```
