/*
Copyright 2021 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hooks

import (
	"context"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = FDescribe("ingress-nginx :: hooks :: safe_daemonset_update ::", func() {
	f := HookExecutionConfigInit(`{"ingressNginx":{"defaultControllerVersion": "1.1", "internal": {}}}`, "")

	Context("ff", func() {
		BeforeEach(func() {
			_ = f.KubeStateSet(`
apiVersion: v1
kind: Pod
metadata:
  creationTimestamp: "2023-03-22T14:37:45Z"
  generateName: controller-test-
  labels:
    app: controller
    controller-revision-hash: 6657cf4d79
    example.io/block-deleting: "true"
    ingress.deckhouse.io/block-deleting: "true"
    lifecycle.apps.kruise.io/state: "PreparingDelete"
    name: test
  name: controller-test-bw8sc
  namespace: d8-ingress-nginx
  ownerReferences:
  - apiVersion: apps.kruise.io/v1alpha1
    blockOwnerDeletion: true
    controller: true
    kind: DaemonSet
    name: controller-test
    uid: 32b2c2ad-b3b6-48be-a565-935715afad03
  resourceVersion: "70560011"
  uid: 1792c866-a71a-49f2-a5eb-301053979be3
spec:
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchFields:
          - key: metadata.name
            operator: In
            values:
            - ndev-worker-5e11c78a-5f688-kw6c5
  containers:
  - args:
    - /nginx-ingress-controller
    - --configmap=$(POD_NAMESPACE)/test-config
    - --v=2
    - --ingress-class=ngizdvfzxcxzcg
    - --healthz-port=10254
    - --http-port=80
    - --https-port=443
    - --update-status=true
    - --shutdown-grace-period=0
    - --controller-class=ingress-nginx.deckhouse.io/ngizdvfzxcxzcg
    - --healthz-host=127.0.0.1
    - --election-id=ingress-controller-leader-ngizdvfzxcxzcg
    env:
    - name: POD_NAME
      valueFrom:
        fieldRef:
          apiVersion: v1
          fieldPath: metadata.name
    - name: POD_NAMESPACE
      valueFrom:
        fieldRef:
          apiVersion: v1
          fieldPath: metadata.namespace
    - name: POD_IP
      value: 127.0.0.1
    image: dev-registry.deckhouse.io/sys/deckhouse-oss:7905a7e33f15c5bb55c0353de94b654baeefd8e58aae5ef555dc68f9-1674395570382
    imagePullPolicy: IfNotPresent
    lifecycle:
      preStop:
        exec:
          command:
          - /wait-shutdown
    livenessProbe:
      failureThreshold: 10
      httpGet:
        path: /controller/healthz
        port: 10354
        scheme: HTTPS
      initialDelaySeconds: 30
      periodSeconds: 10
      successThreshold: 1
      timeoutSeconds: 5
    name: controller
    ports:
    - containerPort: 80
      hostPort: 80
      protocol: TCP
    - containerPort: 443
      hostPort: 443
      protocol: TCP
    readinessProbe:
      failureThreshold: 3
      httpGet:
        path: /controller/healthz
        port: 10354
        scheme: HTTPS
      initialDelaySeconds: 10
      periodSeconds: 2
      successThreshold: 1
      timeoutSeconds: 5
    resources:
      requests:
        ephemeral-storage: 150Mi
    terminationMessagePath: /dev/termination-log
    terminationMessagePolicy: File
    volumeMounts:
    - mountPath: /var/lib/nginx/body
      name: client-body-temp-path
    - mountPath: /var/lib/nginx/fastcgi
      name: fastcgi-temp-path
    - mountPath: /var/lib/nginx/proxy
      name: proxy-temp-path
    - mountPath: /var/lib/nginx/scgi
      name: scgi-temp-path
    - mountPath: /var/lib/nginx/uwsgi
      name: uwsgi-temp-path
    - mountPath: /etc/nginx/ssl/
      name: secret-nginx-auth-tls
    - mountPath: /tmp/nginx/
      name: tmp-nginx
    - mountPath: /etc/nginx/webhook-ssl/
      name: webhook-cert
      readOnly: true
    - mountPath: /var/run/secrets/kubernetes.io/serviceaccount
      name: kube-api-access-mqc6q
      readOnly: true
  - image: dev-registry.deckhouse.io/sys/deckhouse-oss:9bab68c01b48705a0d00ac0fe05580efec66f1147e0e0daecfa761d6-1674403577777
    imagePullPolicy: IfNotPresent
    name: protobuf-exporter
    resources:
      requests:
        cpu: 10m
        ephemeral-storage: 50Mi
        memory: 20Mi
    terminationMessagePath: /dev/termination-log
    terminationMessagePolicy: File
    volumeMounts:
    - mountPath: /var/files
      name: telemetry-config-file
    - mountPath: /var/run/secrets/kubernetes.io/serviceaccount
      name: kube-api-access-mqc6q
      readOnly: true
  - args:
    - --secure-listen-address=$(KUBE_RBAC_PROXY_LISTEN_ADDRESS):10354
    - --v=2
    - --logtostderr=true
    - --stale-cache-interval=1h30m
    env:
    - name: KUBE_RBAC_PROXY_LISTEN_ADDRESS
      valueFrom:
        fieldRef:
          apiVersion: v1
          fieldPath: status.podIP
    - name: KUBE_RBAC_PROXY_CONFIG
      value: |
        excludePaths:
        - /controller/healthz
        upstreams:
        - upstream: http://127.0.0.1:10254/
          path: /controller/
          authorization:
            resourceAttributes:
              namespace: d8-ingress-nginx
              apiGroup: apps
              apiVersion: v1
              resource: daemonsets
              subresource: prometheus-controller-metrics
              name: ingress-nginx
        - upstream: http://127.0.0.1:9091/metrics
          path: /protobuf/metrics
          authorization:
            resourceAttributes:
              namespace: d8-ingress-nginx
              apiGroup: apps
              apiVersion: v1
              resource: daemonsets
              subresource: prometheus-protobuf-metrics
              name: ingress-nginx
    image: dev-registry.deckhouse.io/sys/deckhouse-oss:7978ea3cf28c53850e8f11c115aabbce7b1a7f1928e5fbc558f5dd8e-1674392281105
    imagePullPolicy: IfNotPresent
    name: kube-rbac-proxy
    ports:
    - containerPort: 10354
      hostPort: 10354
      name: https-metrics
      protocol: TCP
    resources:
      requests:
        cpu: 10m
        ephemeral-storage: 50Mi
        memory: 20Mi
    terminationMessagePath: /dev/termination-log
    terminationMessagePolicy: File
    volumeMounts:
    - mountPath: /var/run/secrets/kubernetes.io/serviceaccount
      name: kube-api-access-mqc6q
      readOnly: true
  dnsPolicy: ClusterFirstWithHostNet
  enableServiceLinks: true
  hostNetwork: true
  imagePullSecrets:
  - name: deckhouse-registry
  nodeName: ndev-worker-5e11c78a-5f688-kw6c5
  nodeSelector:
    node-role.kubernetes.io/worker: ""
  preemptionPolicy: PreemptLowerPriority
  priority: 2000000000
  priorityClassName: system-cluster-critical
  restartPolicy: Always
  schedulerName: default-scheduler
  securityContext: {}
  serviceAccount: ingress-nginx
  serviceAccountName: ingress-nginx
  terminationGracePeriodSeconds: 420
  tolerations:
  - effect: NoSchedule
    operator: Exists
  - effect: NoExecute
    key: node.kubernetes.io/not-ready
    operator: Exists
  - effect: NoExecute
    key: node.kubernetes.io/unreachable
    operator: Exists
  - effect: NoSchedule
    key: node.kubernetes.io/disk-pressure
    operator: Exists
  - effect: NoSchedule
    key: node.kubernetes.io/memory-pressure
    operator: Exists
  - effect: NoSchedule
    key: node.kubernetes.io/pid-pressure
    operator: Exists
  - effect: NoSchedule
    key: node.kubernetes.io/unschedulable
    operator: Exists
  - effect: NoSchedule
    key: node.kubernetes.io/network-unavailable
    operator: Exists
  volumes:
  - emptyDir: {}
    name: tmp-nginx
  - emptyDir: {}
    name: client-body-temp-path
  - emptyDir: {}
    name: fastcgi-temp-path
  - emptyDir: {}
    name: proxy-temp-path
  - emptyDir: {}
    name: scgi-temp-path
  - emptyDir: {}
    name: uwsgi-temp-path
  - name: secret-nginx-auth-tls
    secret:
      defaultMode: 420
      secretName: ingress-nginx-test-auth-tls
  - name: webhook-cert
    secret:
      defaultMode: 420
      secretName: ingress-admission-certificate
  - configMap:
      defaultMode: 420
      name: d8-ingress-telemetry-config
    name: telemetry-config-file
  - name: kube-api-access-mqc6q
    projected:
      defaultMode: 420
      sources:
      - serviceAccountToken:
          expirationSeconds: 3607
          path: token
      - configMap:
          items:
          - key: ca.crt
            path: ca.crt
          name: kube-root-ca.crt
      - downwardAPI:
          items:
          - fieldRef:
              apiVersion: v1
              fieldPath: metadata.namespace
            path: namespace
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2023-03-22T14:37:45Z"
    status: "True"
    type: Initialized
  - lastProbeTime: null
    lastTransitionTime: "2023-03-24T15:02:56Z"
    status: "True"
    type: Ready
  - lastProbeTime: null
    lastTransitionTime: "2023-03-24T15:02:56Z"
    status: "True"
    type: ContainersReady
  - lastProbeTime: null
    lastTransitionTime: "2023-03-22T14:37:45Z"
    status: "True"
    type: PodScheduled
  containerStatuses:
  - containerID: containerd://bf2c50336b2aa2cba32d86a3cf98d8ba880cc1935f1db91f083c23a05d74c4e5
    image: dev-registry.deckhouse.io/sys/deckhouse-oss:7905a7e33f15c5bb55c0353de94b654baeefd8e58aae5ef555dc68f9-1674395570382
    imageID: dev-registry.deckhouse.io/sys/deckhouse-oss@sha256:a9dc9f65840ce790ac2d3c719fdd68236fd12f80d7181ecc15de043b1aa1e70d
    lastState: {}
    name: controller
    ready: true
    restartCount: 0
    started: true
    state:
      running:
        startedAt: "2023-03-22T14:37:46Z"
  - containerID: containerd://cf1edf2fb87661208520035d79f2de1a84e8b829242757fbb57b0622758a4541
    image: dev-registry.deckhouse.io/sys/deckhouse-oss:7978ea3cf28c53850e8f11c115aabbce7b1a7f1928e5fbc558f5dd8e-1674392281105
    imageID: dev-registry.deckhouse.io/sys/deckhouse-oss@sha256:9788231f1b69e12aa0d01162ad8a45b990e3b8965e298fcc27b03558ee9e55fe
    lastState: {}
    name: kube-rbac-proxy
    ready: true
    restartCount: 0
    started: true
    state:
      running:
        startedAt: "2023-03-22T14:37:47Z"
  - containerID: containerd://849a8b3c984506adf2be1b52edf55fba61acfe192878f78fc53a1bf568dd24c6
    image: dev-registry.deckhouse.io/sys/deckhouse-oss:9bab68c01b48705a0d00ac0fe05580efec66f1147e0e0daecfa761d6-1674403577777
    imageID: dev-registry.deckhouse.io/sys/deckhouse-oss@sha256:6aaede02e3bc0a3b3b756a6724da5c65508a26596af9a884a9f5c4e45722b611
    lastState: {}
    name: protobuf-exporter
    ready: true
    restartCount: 0
    started: true
    state:
      running:
        startedAt: "2023-03-22T14:37:46Z"
  hostIP: 192.168.199.253
  phase: Running
  podIP: 192.168.199.253
  podIPs:
  - ip: 192.168.199.253
  qosClass: Burstable
  startTime: "2023-03-22T14:37:45Z"
---
apiVersion: v1
kind: Pod
metadata:
  creationTimestamp: "2023-03-22T14:46:07Z"
  generateName: controller-test-failover-
  labels:
    app: controller
    controller-revision-hash: 897b7cf66
    name: test-failover
  name: controller-test-failover-qq89j
  namespace: d8-ingress-nginx
  ownerReferences:
  - apiVersion: apps.kruise.io/v1alpha1
    blockOwnerDeletion: true
    controller: true
    kind: DaemonSet
    name: controller-test-failover
    uid: 7521373f-57dc-418f-8cd6-f94f29868cde
  resourceVersion: "70559989"
  uid: 4253a4fd-aca5-4602-ae48-81e78f3d74e5
spec:
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchFields:
          - key: metadata.name
            operator: In
            values:
            - ndev-worker-5e11c78a-5f688-kw6c5
  containers:
  - args:
    - /nginx-ingress-controller
    - --configmap=$(POD_NAMESPACE)/test-failover-config
    - --v=2
    - --ingress-class=ngizdvfzxchgcg
    - --healthz-port=10254
    - --http-port=80
    - --https-port=443
    - --update-status=true
    - --shutdown-grace-period=0
    - --validating-webhook=:8443
    - --validating-webhook-certificate=/etc/nginx/webhook-ssl/tls.crt
    - --validating-webhook-key=/etc/nginx/webhook-ssl/tls.key
    - --controller-class=ingress-nginx.deckhouse.io/ngizdvfzxchgcg
    - --healthz-host=127.0.0.1
    - --election-id=ingress-controller-leader-ngizdvfzxchgcg
    env:
    - name: POD_NAME
      valueFrom:
        fieldRef:
          apiVersion: v1
          fieldPath: metadata.name
    - name: POD_NAMESPACE
      valueFrom:
        fieldRef:
          apiVersion: v1
          fieldPath: metadata.namespace
    - name: POD_IP
      value: 127.0.0.1
    image: dev-registry.deckhouse.io/sys/deckhouse-oss:7905a7e33f15c5bb55c0353de94b654baeefd8e58aae5ef555dc68f9-1674395570382
    imagePullPolicy: IfNotPresent
    lifecycle:
      preStop:
        exec:
          command:
          - /wait-shutdown
    livenessProbe:
      failureThreshold: 10
      httpGet:
        path: /controller/healthz
        port: 10354
        scheme: HTTPS
      initialDelaySeconds: 30
      periodSeconds: 10
      successThreshold: 1
      timeoutSeconds: 5
    name: controller
    ports:
    - containerPort: 80
      protocol: TCP
    - containerPort: 443
      protocol: TCP
    - containerPort: 8443
      name: webhook
      protocol: TCP
    readinessProbe:
      failureThreshold: 3
      httpGet:
        path: /controller/healthz
        port: 10354
        scheme: HTTPS
      initialDelaySeconds: 10
      periodSeconds: 2
      successThreshold: 1
      timeoutSeconds: 5
    resources:
      requests:
        ephemeral-storage: 150Mi
    terminationMessagePath: /dev/termination-log
    terminationMessagePolicy: File
    volumeMounts:
    - mountPath: /var/lib/nginx/body
      name: client-body-temp-path
    - mountPath: /var/lib/nginx/fastcgi
      name: fastcgi-temp-path
    - mountPath: /var/lib/nginx/proxy
      name: proxy-temp-path
    - mountPath: /var/lib/nginx/scgi
      name: scgi-temp-path
    - mountPath: /var/lib/nginx/uwsgi
      name: uwsgi-temp-path
    - mountPath: /etc/nginx/ssl/
      name: secret-nginx-auth-tls
    - mountPath: /tmp/nginx/
      name: tmp-nginx
    - mountPath: /etc/nginx/webhook-ssl/
      name: webhook-cert
      readOnly: true
    - mountPath: /var/run/secrets/kubernetes.io/serviceaccount
      name: kube-api-access-7d6vw
      readOnly: true
  - image: dev-registry.deckhouse.io/sys/deckhouse-oss:9bab68c01b48705a0d00ac0fe05580efec66f1147e0e0daecfa761d6-1674403577777
    imagePullPolicy: IfNotPresent
    name: protobuf-exporter
    resources:
      requests:
        cpu: 10m
        ephemeral-storage: 50Mi
        memory: 20Mi
    terminationMessagePath: /dev/termination-log
    terminationMessagePolicy: File
    volumeMounts:
    - mountPath: /var/files
      name: telemetry-config-file
    - mountPath: /var/run/secrets/kubernetes.io/serviceaccount
      name: kube-api-access-7d6vw
      readOnly: true
  - args:
    - --secure-listen-address=$(KUBE_RBAC_PROXY_LISTEN_ADDRESS):10354
    - --v=2
    - --logtostderr=true
    - --stale-cache-interval=1h30m
    env:
    - name: KUBE_RBAC_PROXY_LISTEN_ADDRESS
      valueFrom:
        fieldRef:
          apiVersion: v1
          fieldPath: status.podIP
    - name: KUBE_RBAC_PROXY_CONFIG
      value: |
        excludePaths:
        - /controller/healthz
        upstreams:
        - upstream: http://127.0.0.1:10254/
          path: /controller/
          authorization:
            resourceAttributes:
              namespace: d8-ingress-nginx
              apiGroup: apps
              apiVersion: v1
              resource: daemonsets
              subresource: prometheus-controller-metrics
              name: ingress-nginx
        - upstream: http://127.0.0.1:9091/metrics
          path: /protobuf/metrics
          authorization:
            resourceAttributes:
              namespace: d8-ingress-nginx
              apiGroup: apps
              apiVersion: v1
              resource: daemonsets
              subresource: prometheus-protobuf-metrics
              name: ingress-nginx
    image: dev-registry.deckhouse.io/sys/deckhouse-oss:7978ea3cf28c53850e8f11c115aabbce7b1a7f1928e5fbc558f5dd8e-1674392281105
    imagePullPolicy: IfNotPresent
    name: kube-rbac-proxy
    ports:
    - containerPort: 10354
      name: https-metrics
      protocol: TCP
    resources:
      requests:
        cpu: 10m
        ephemeral-storage: 50Mi
        memory: 20Mi
    terminationMessagePath: /dev/termination-log
    terminationMessagePolicy: File
    volumeMounts:
    - mountPath: /var/run/secrets/kubernetes.io/serviceaccount
      name: kube-api-access-7d6vw
      readOnly: true
  dnsPolicy: ClusterFirst
  enableServiceLinks: true
  imagePullSecrets:
  - name: deckhouse-registry
  nodeName: ndev-worker-5e11c78a-5f688-kw6c5
  nodeSelector:
    node-role.kubernetes.io/worker: ""
  preemptionPolicy: PreemptLowerPriority
  priority: 2000000000
  priorityClassName: system-cluster-critical
  restartPolicy: Always
  schedulerName: default-scheduler
  securityContext: {}
  serviceAccount: ingress-nginx
  serviceAccountName: ingress-nginx
  terminationGracePeriodSeconds: 420
  tolerations:
  - effect: NoSchedule
    operator: Exists
  - effect: NoExecute
    key: node.kubernetes.io/not-ready
    operator: Exists
  - effect: NoExecute
    key: node.kubernetes.io/unreachable
    operator: Exists
  - effect: NoSchedule
    key: node.kubernetes.io/disk-pressure
    operator: Exists
  - effect: NoSchedule
    key: node.kubernetes.io/memory-pressure
    operator: Exists
  - effect: NoSchedule
    key: node.kubernetes.io/pid-pressure
    operator: Exists
  - effect: NoSchedule
    key: node.kubernetes.io/unschedulable
    operator: Exists
  volumes:
  - emptyDir: {}
    name: tmp-nginx
  - emptyDir: {}
    name: client-body-temp-path
  - emptyDir: {}
    name: fastcgi-temp-path
  - emptyDir: {}
    name: proxy-temp-path
  - emptyDir: {}
    name: scgi-temp-path
  - emptyDir: {}
    name: uwsgi-temp-path
  - name: secret-nginx-auth-tls
    secret:
      defaultMode: 420
      secretName: ingress-nginx-test-auth-tls
  - name: webhook-cert
    secret:
      defaultMode: 420
      secretName: ingress-admission-certificate
  - configMap:
      defaultMode: 420
      name: d8-ingress-telemetry-config
    name: telemetry-config-file
  - name: kube-api-access-7d6vw
    projected:
      defaultMode: 420
      sources:
      - serviceAccountToken:
          expirationSeconds: 3607
          path: token
      - configMap:
          items:
          - key: ca.crt
            path: ca.crt
          name: kube-root-ca.crt
      - downwardAPI:
          items:
          - fieldRef:
              apiVersion: v1
              fieldPath: metadata.namespace
            path: namespace
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2023-03-22T14:46:07Z"
    status: "True"
    type: Initialized
  - lastProbeTime: null
    lastTransitionTime: "2023-03-24T15:02:55Z"
    status: "True"
    type: Ready
  - lastProbeTime: null
    lastTransitionTime: "2023-03-24T15:02:55Z"
    status: "True"
    type: ContainersReady
  - lastProbeTime: null
    lastTransitionTime: "2023-03-22T14:46:07Z"
    status: "True"
    type: PodScheduled
  containerStatuses:
  - containerID: containerd://987b41446044858311900234ec11784b36db4b795f1328ba11a487c9265eff99
    image: dev-registry.deckhouse.io/sys/deckhouse-oss:7905a7e33f15c5bb55c0353de94b654baeefd8e58aae5ef555dc68f9-1674395570382
    imageID: dev-registry.deckhouse.io/sys/deckhouse-oss@sha256:a9dc9f65840ce790ac2d3c719fdd68236fd12f80d7181ecc15de043b1aa1e70d
    lastState: {}
    name: controller
    ready: true
    restartCount: 0
    started: true
    state:
      running:
        startedAt: "2023-03-22T14:46:10Z"
  - containerID: containerd://53a8f295f2eb4b678b24ea3fb61d741e655d40d1a7c4a9f7cbe8bc8065db6ae3
    image: dev-registry.deckhouse.io/sys/deckhouse-oss:7978ea3cf28c53850e8f11c115aabbce7b1a7f1928e5fbc558f5dd8e-1674392281105
    imageID: dev-registry.deckhouse.io/sys/deckhouse-oss@sha256:9788231f1b69e12aa0d01162ad8a45b990e3b8965e298fcc27b03558ee9e55fe
    lastState: {}
    name: kube-rbac-proxy
    ready: true
    restartCount: 0
    started: true
    state:
      running:
        startedAt: "2023-03-22T14:46:11Z"
  - containerID: containerd://a5adeb2219de8f356ce34ea7c306002761ae606ca32ae48af1c7f944b1f8f314
    image: dev-registry.deckhouse.io/sys/deckhouse-oss:9bab68c01b48705a0d00ac0fe05580efec66f1147e0e0daecfa761d6-1674403577777
    imageID: dev-registry.deckhouse.io/sys/deckhouse-oss@sha256:6aaede02e3bc0a3b3b756a6724da5c65508a26596af9a884a9f5c4e45722b611
    lastState: {}
    name: protobuf-exporter
    ready: true
    restartCount: 0
    started: true
    state:
      running:
        startedAt: "2023-03-22T14:46:10Z"
  hostIP: 192.168.199.253
  phase: Running
  podIP: 10.111.4.227
  podIPs:
  - ip: 10.111.4.227
  qosClass: Burstable
  startTime: "2023-03-22T14:46:07Z"
---
apiVersion: v1
kind: Pod
metadata:
  creationTimestamp: "2023-03-22T14:37:45Z"
  generateName: proxy-test-failover-
  labels:
    app: proxy-failover
    controller-revision-hash: 68886b59f6
    name: test
  name: proxy-test-failover-54nvz
  namespace: d8-ingress-nginx
  ownerReferences:
  - apiVersion: apps.kruise.io/v1alpha1
    blockOwnerDeletion: true
    controller: true
    kind: DaemonSet
    name: proxy-test-failover
    uid: 4acb05e8-1cc2-4825-8400-7d98a05db84f
  resourceVersion: "68891573"
  uid: d37e7914-cd14-4cc8-8446-350ffae7ca5b
spec:
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchFields:
          - key: metadata.name
            operator: In
            values:
            - ndev-worker-5e11c78a-5f688-kw6c5
  containers:
  - env:
    - name: CONTROLLER_NAME
      value: test
    image: dev-registry.deckhouse.io/sys/deckhouse-oss:0bcfaa4bb9c0c5fae928f1755cb658e2cadce24367241ac8be88a8c6-1676269283495
    imagePullPolicy: IfNotPresent
    lifecycle:
      preStop:
        exec:
          command:
          - /usr/sbin/nginx
          - -s
          - quit
    livenessProbe:
      failureThreshold: 3
      httpGet:
        host: 127.0.0.1
        path: /healthz
        port: 10253
        scheme: HTTP
      initialDelaySeconds: 3
      periodSeconds: 10
      successThreshold: 1
      timeoutSeconds: 1
    name: nginx
    resources:
      requests:
        cpu: 350m
        ephemeral-storage: 50Mi
        memory: 500Mi
    terminationMessagePath: /dev/termination-log
    terminationMessagePolicy: File
    volumeMounts:
    - mountPath: /var/run/secrets/kubernetes.io/serviceaccount
      name: kube-api-access-tls75
      readOnly: true
  - image: dev-registry.deckhouse.io/sys/deckhouse-oss:b3c7d8928ff06eaf8b5b3e0ae8bc326d90afe6ef4a565140d331f9a0-1676278402409
    imagePullPolicy: IfNotPresent
    name: iptables-loop
    resources:
      requests:
        cpu: 10m
        ephemeral-storage: 50Mi
        memory: 20Mi
    securityContext:
      capabilities:
        add:
        - NET_RAW
        - NET_ADMIN
    terminationMessagePath: /dev/termination-log
    terminationMessagePolicy: File
    volumeMounts:
    - mountPath: /run/xtables.lock
      name: xtables-lock
    - mountPath: /var/run/secrets/kubernetes.io/serviceaccount
      name: kube-api-access-tls75
      readOnly: true
  - args:
    - -web.listen-address=127.0.0.1:10354
    - -nginx.scrape-uri=http://127.0.0.1:10253/nginx_status
    - -nginx.ssl-verify=false
    - -nginx.retries=10
    - -nginx.retry-interval=6s
    image: dev-registry.deckhouse.io/sys/deckhouse-oss:069016a8c6f1e16721687055516efd1e1f7e6e3bee8fed0ae51960c9-1674396551022
    imagePullPolicy: IfNotPresent
    livenessProbe:
      failureThreshold: 3
      httpGet:
        host: 127.0.0.1
        path: /metrics
        port: 10354
        scheme: HTTP
      periodSeconds: 10
      successThreshold: 1
      timeoutSeconds: 1
    name: nginx-exporter
    resources:
      requests:
        cpu: 10m
        ephemeral-storage: 50Mi
        memory: 20Mi
    terminationMessagePath: /dev/termination-log
    terminationMessagePolicy: File
    volumeMounts:
    - mountPath: /var/run/secrets/kubernetes.io/serviceaccount
      name: kube-api-access-tls75
      readOnly: true
  - args:
    - --secure-listen-address=$(KUBE_RBAC_PROXY_LISTEN_ADDRESS):10355
    - --v=2
    - --logtostderr=true
    - --stale-cache-interval=1h30m
    env:
    - name: KUBE_RBAC_PROXY_LISTEN_ADDRESS
      valueFrom:
        fieldRef:
          apiVersion: v1
          fieldPath: status.podIP
    - name: KUBE_RBAC_PROXY_CONFIG
      value: |
        upstreams:
        - upstream: http://127.0.0.1:10354/metrics
          path: /metrics
          authorization:
            resourceAttributes:
              namespace: d8-ingress-nginx
              apiGroup: apps
              apiVersion: v1
              resource: daemonsets
              subresource: prometheus-metrics
              name: proxy-failover
    image: dev-registry.deckhouse.io/sys/deckhouse-oss:7978ea3cf28c53850e8f11c115aabbce7b1a7f1928e5fbc558f5dd8e-1674392281105
    imagePullPolicy: IfNotPresent
    name: kube-rbac-proxy
    ports:
    - containerPort: 10355
      hostPort: 10355
      name: https-metrics
      protocol: TCP
    resources:
      requests:
        cpu: 10m
        ephemeral-storage: 50Mi
        memory: 20Mi
    terminationMessagePath: /dev/termination-log
    terminationMessagePolicy: File
    volumeMounts:
    - mountPath: /var/run/secrets/kubernetes.io/serviceaccount
      name: kube-api-access-tls75
      readOnly: true
  dnsPolicy: ClusterFirstWithHostNet
  enableServiceLinks: true
  hostNetwork: true
  imagePullSecrets:
  - name: deckhouse-registry
  nodeName: ndev-worker-5e11c78a-5f688-kw6c5
  nodeSelector:
    node-role.kubernetes.io/worker: ""
  preemptionPolicy: PreemptLowerPriority
  priority: 2000000000
  priorityClassName: system-cluster-critical
  restartPolicy: Always
  schedulerName: default-scheduler
  securityContext: {}
  serviceAccount: ingress-nginx
  serviceAccountName: ingress-nginx
  terminationGracePeriodSeconds: 300
  tolerations:
  - effect: NoSchedule
    operator: Exists
  - effect: NoExecute
    key: node.kubernetes.io/not-ready
    operator: Exists
  - effect: NoExecute
    key: node.kubernetes.io/unreachable
    operator: Exists
  - effect: NoSchedule
    key: node.kubernetes.io/disk-pressure
    operator: Exists
  - effect: NoSchedule
    key: node.kubernetes.io/memory-pressure
    operator: Exists
  - effect: NoSchedule
    key: node.kubernetes.io/pid-pressure
    operator: Exists
  - effect: NoSchedule
    key: node.kubernetes.io/unschedulable
    operator: Exists
  - effect: NoSchedule
    key: node.kubernetes.io/network-unavailable
    operator: Exists
  volumes:
  - hostPath:
      path: /run/xtables.lock
      type: FileOrCreate
    name: xtables-lock
  - name: kube-api-access-tls75
    projected:
      defaultMode: 420
      sources:
      - serviceAccountToken:
          expirationSeconds: 3607
          path: token
      - configMap:
          items:
          - key: ca.crt
            path: ca.crt
          name: kube-root-ca.crt
      - downwardAPI:
          items:
          - fieldRef:
              apiVersion: v1
              fieldPath: metadata.namespace
            path: namespace
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2023-03-22T14:37:45Z"
    status: "True"
    type: Initialized
  - lastProbeTime: null
    lastTransitionTime: "2023-03-22T14:37:48Z"
    status: "True"
    type: Ready
  - lastProbeTime: null
    lastTransitionTime: "2023-03-22T14:37:48Z"
    status: "True"
    type: ContainersReady
  - lastProbeTime: null
    lastTransitionTime: "2023-03-22T14:37:45Z"
    status: "True"
    type: PodScheduled
  containerStatuses:
  - containerID: containerd://9ac28f078b3839f258f5096e234554f65fbfd977480cab5105a52931820408b3
    image: dev-registry.deckhouse.io/sys/deckhouse-oss:b3c7d8928ff06eaf8b5b3e0ae8bc326d90afe6ef4a565140d331f9a0-1676278402409
    imageID: dev-registry.deckhouse.io/sys/deckhouse-oss@sha256:4c52e6643f3cfcdb47d1e5d22de03c0d78ab5dcc1ee0d8c8c9d04d1cb0c4b50b
    lastState: {}
    name: iptables-loop
    ready: true
    restartCount: 0
    started: true
    state:
      running:
        startedAt: "2023-03-22T14:37:47Z"
  - containerID: containerd://08b8007efc76c0fe71793cdeaf4431af0582474ea1756587ff0dab1f6b985950
    image: dev-registry.deckhouse.io/sys/deckhouse-oss:7978ea3cf28c53850e8f11c115aabbce7b1a7f1928e5fbc558f5dd8e-1674392281105
    imageID: dev-registry.deckhouse.io/sys/deckhouse-oss@sha256:9788231f1b69e12aa0d01162ad8a45b990e3b8965e298fcc27b03558ee9e55fe
    lastState: {}
    name: kube-rbac-proxy
    ready: true
    restartCount: 0
    started: true
    state:
      running:
        startedAt: "2023-03-22T14:37:48Z"
  - containerID: containerd://ebaf0bf7ff31148e51b1adacc06a888f7bd8973c6fd5446223aa0c2ee919497d
    image: dev-registry.deckhouse.io/sys/deckhouse-oss:0bcfaa4bb9c0c5fae928f1755cb658e2cadce24367241ac8be88a8c6-1676269283495
    imageID: dev-registry.deckhouse.io/sys/deckhouse-oss@sha256:d42fbb3ccfaad074981ce21ce222afa5cff1f3cdf609ad3235d15f5bba3a24ac
    lastState: {}
    name: nginx
    ready: true
    restartCount: 0
    started: true
    state:
      running:
        startedAt: "2023-03-22T14:37:46Z"
  - containerID: containerd://19df32dc01cec686eede44a78d010724c6f3278184406524056a26307baea983
    image: dev-registry.deckhouse.io/sys/deckhouse-oss:069016a8c6f1e16721687055516efd1e1f7e6e3bee8fed0ae51960c9-1674396551022
    imageID: dev-registry.deckhouse.io/sys/deckhouse-oss@sha256:d4f9ee33d63bb837b138f3d85c02dc9afd9df1a0c7f9011ef30aeaeec75f1e79
    lastState: {}
    name: nginx-exporter
    ready: true
    restartCount: 0
    started: true
    state:
      running:
        startedAt: "2023-03-22T14:37:47Z"
  hostIP: 192.168.199.253
  phase: Running
  podIP: 192.168.199.253
  podIPs:
  - ip: 192.168.199.253
  qosClass: Burstable
  startTime: "2023-03-22T14:37:45Z"
`)

			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})

		It("foo", func() {
			Expect(f).To(ExecuteSuccessfully())
			pod := f.KubernetesResource("Pod", "d8-ingress-nginx", "controller-test-bw8sc")
			fmt.Println(pod.ToYaml())
		})
	})
})

var _ = Describe("ingress-nginx :: hooks :: safe_daemonset_update ::", func() {
	f := HookExecutionConfigInit(`{"ingressNginx":{"defaultControllerVersion": "1.1", "internal": {}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1", "IngressNginxController", true)

	dsControllerMainInitialYAML := `
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: controller-main
  namespace: d8-ingress-nginx
  labels:
    name: main
    app: controller
    ingress-nginx-safe-update: ""
  generation: 1
  annotations:
    ingress-nginx-controller.deckhouse.io/checksum: main-checksum-123
spec:
  selector:
    matchLabels:
      app: controller
      name: main
status:
  currentNumberScheduled: 2
  desiredNumberScheduled: 2
  numberAvailable: 2
  numberMisscheduled: 0
  numberReady: 2
  observedGeneration: 1
  updatedNumberScheduled: 2
`
	dsProxyMainFailoverInitialYAML := `
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: proxy-main-failover
  namespace: d8-ingress-nginx
  labels:
    name: main
    app: proxy-failover
    ingress-nginx-safe-update: ""
  generation: 1
  annotations:
    ingress-nginx-controller.deckhouse.io/checksum: main-checksum-123
spec:
  selector:
    matchLabels:
      app: proxy-failover
      name: main
status:
  currentNumberScheduled: 2
  desiredNumberScheduled: 2
  numberAvailable: 2
  numberMisscheduled: 0
  numberReady: 2
  observedGeneration: 1
  updatedNumberScheduled: 2
`
	dsControllerMainFailoverInitialYAML := `
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: controller-main-failover
  namespace: d8-ingress-nginx
  labels:
    name: main-failover
    app: controller
    ingress-nginx-failover: ""
  generation: 1
  annotations:
    ingress-nginx-controller.deckhouse.io/checksum: main-checksum-123
spec:
  selector:
    matchLabels:
      app: controller
      name: main-failover
status:
  currentNumberScheduled: 2
  desiredNumberScheduled: 2
  numberAvailable: 2
  numberMisscheduled: 0
  numberReady: 2
  observedGeneration: 1
  updatedNumberScheduled: 2
`

	pod1ControllerMainInitialYAML := `
---
apiVersion: v1
kind: Pod
metadata:
  name: controller-main-1
  namespace: d8-ingress-nginx
  labels:
    app: controller
    name: main
    pod-template-generation: "1"
status:
  conditions:
  - status: "True"
    type: Ready
`

	pod2ControllerMainInitialYAML := `
---
apiVersion: v1
kind: Pod
metadata:
  name: controller-main-2
  namespace: d8-ingress-nginx
  labels:
    app: controller
    name: main
    pod-template-generation: "1"
status:
  conditions:
  - status: "True"
    type: Ready
`
	pod1ProxyMainFailoverInitialYAML := `
---
apiVersion: v1
kind: Pod
metadata:
  name: proxy-main-failover-1
  namespace: d8-ingress-nginx
  labels:
    app: proxy-failover
    name: main
    pod-template-generation: "1"
status:
  conditions:
  - status: "True"
    type: Ready
`
	pod2ProxyMainFailoverInitialYAML := `
---
apiVersion: v1
kind: Pod
metadata:
  name: proxy-main-failover-2
  namespace: d8-ingress-nginx
  labels:
    app: proxy-failover
    name: main
    pod-template-generation: "1"
status:
  conditions:
  - status: "True"
    type: Ready
`
	pod1ControllerMainFailoverInitialYAML := `
---
apiVersion: v1
kind: Pod
metadata:
  name: controller-main-failover-1
  namespace: d8-ingress-nginx
  labels:
    app: controller
    name: main-failover
    pod-template-generation: "1"
status:
  conditions:
  - status: "True"
    type: Ready
`
	pod2ControllerMainFailoverInitialYAML := `
---
apiVersion: v1
kind: Pod
metadata:
  name: controller-main-failover-2
  namespace: d8-ingress-nginx
  labels:
    app: controller
    name: main-failover
    pod-template-generation: "1"
status:
  conditions:
  - status: "True"
    type: Ready
`

	pod2TerminatingControllerMainInitialYAML := `
---
apiVersion: v1
kind: Pod
metadata:
  deletionTimestamp: "2023-02-19T11:28:08Z"
  name: controller-main-2
  namespace: d8-ingress-nginx
  labels:
    app: controller
    name: main
    pod-template-generation: "1"
status:
  conditions:
  - status: "True"
    type: Ready
`
	var dsControllerMain *v1.DaemonSet
	var dsProxyMainFailover *v1.DaemonSet
	var dsControllerMainFailover *v1.DaemonSet
	var pod1ControllerMain *corev1.Pod
	var pod2ControllerMain *corev1.Pod
	var pod1ProxyMainFailover *corev1.Pod
	var pod2ProxyMainFailover *corev1.Pod
	var pod1ControllerMainFailover *corev1.Pod
	var pod2ControllerMainFailover *corev1.Pod
	var pod2TerminatingControllerMain *corev1.Pod

	BeforeEach(func() {
		_ = yaml.Unmarshal([]byte(dsControllerMainInitialYAML), &dsControllerMain)
		_ = yaml.Unmarshal([]byte(dsProxyMainFailoverInitialYAML), &dsProxyMainFailover)
		_ = yaml.Unmarshal([]byte(dsControllerMainFailoverInitialYAML), &dsControllerMainFailover)
		_ = yaml.Unmarshal([]byte(pod1ControllerMainInitialYAML), &pod1ControllerMain)
		_ = yaml.Unmarshal([]byte(pod2ControllerMainInitialYAML), &pod2ControllerMain)
		_ = yaml.Unmarshal([]byte(pod1ProxyMainFailoverInitialYAML), &pod1ProxyMainFailover)
		_ = yaml.Unmarshal([]byte(pod2ProxyMainFailoverInitialYAML), &pod2ProxyMainFailover)
		_ = yaml.Unmarshal([]byte(pod1ControllerMainFailoverInitialYAML), &pod1ControllerMainFailover)
		_ = yaml.Unmarshal([]byte(pod2ControllerMainFailoverInitialYAML), &pod2ControllerMainFailover)
		_ = yaml.Unmarshal([]byte(pod2TerminatingControllerMainInitialYAML), &pod2TerminatingControllerMain)
	})

	Context("all daemonsets updated", func() {
		BeforeEach(func() {
			dsControllerMainYAML, _ := yaml.Marshal(&dsControllerMain)
			dsProxyMainFailoverYAML, _ := yaml.Marshal(&dsProxyMainFailover)
			dsControllerMainFailoverYAML, _ := yaml.Marshal(&dsControllerMainFailover)
			pod1ControllerMainYAML, _ := yaml.Marshal(&pod1ControllerMain)
			pod2ControllerMainYAML, _ := yaml.Marshal(&pod2ControllerMain)
			pod1ProxyMainFailoverYAML, _ := yaml.Marshal(&pod1ProxyMainFailover)
			pod2ProxyMainFailoverYAML, _ := yaml.Marshal(&pod2ProxyMainFailover)
			pod1ControllerMainFailoverYAML, _ := yaml.Marshal(&pod1ControllerMainFailover)
			pod2ControllerMainFailoverYAML, _ := yaml.Marshal(&pod2ControllerMainFailover)

			clusterState := strings.Join([]string{string(dsControllerMainYAML), string(dsProxyMainFailoverYAML), string(dsControllerMainFailoverYAML), string(pod1ControllerMainYAML), string(pod2ControllerMainYAML), string(pod1ProxyMainFailoverYAML), string(pod2ProxyMainFailoverYAML), string(pod1ControllerMainFailoverYAML), string(pod2ControllerMainFailoverYAML)}, "---\n")

			f.BindingContexts.Set(f.KubeStateSet(clusterState))
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			_, _ = f.KubeClient().AppsV1().DaemonSets("d8-ingress-nginx").Create(context.TODO(), dsControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().AppsV1().DaemonSets("d8-ingress-nginx").Create(context.TODO(), dsProxyMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().AppsV1().DaemonSets("d8-ingress-nginx").Create(context.TODO(), dsControllerMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod1ControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod2ControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod1ProxyMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod2ProxyMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod1ControllerMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod2ControllerMainFailover, metav1.CreateOptions{})

			f.RunHook()
		})

		It("must be execute successfully without any changes", func() {
			Expect(f).To(ExecuteSuccessfully())

			pod1ControllerMainAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod1ControllerMain.Name, metav1.GetOptions{})
			pod2ControllerMainAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod2ControllerMain.Name, metav1.GetOptions{})
			pod1ProxyMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod1ProxyMainFailover.Name, metav1.GetOptions{})
			pod2ProxyMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod2ProxyMainFailover.Name, metav1.GetOptions{})
			pod1ControllerMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod1ControllerMainFailover.Name, metav1.GetOptions{})
			pod2ControllerMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod2ControllerMainFailover.Name, metav1.GetOptions{})

			Expect(pod1ControllerMainAfterRunHook).To(Equal(pod1ControllerMain))
			Expect(pod2ControllerMainAfterRunHook).To(Equal(pod2ControllerMain))
			Expect(pod1ProxyMainFailoverAfterRunHook).To(Equal(pod1ProxyMainFailover))
			Expect(pod2ProxyMainFailoverAfterRunHook).To(Equal(pod2ProxyMainFailover))
			Expect(pod1ControllerMainFailoverAfterRunHook).To(Equal(pod1ControllerMainFailover))
			Expect(pod2ControllerMainFailoverAfterRunHook).To(Equal(pod2ControllerMainFailover))
		})
	})

	Context("daemonset controller-main with terminating pod update scheduled", func() {
		BeforeEach(func() {
			dsControllerMain.Generation = 2
			dsControllerMain.Status.UpdatedNumberScheduled = 1
			pod2ControllerMain.Labels["pod-template-generation"] = "2"

			dsControllerMainYAML, _ := yaml.Marshal(&dsControllerMain)
			dsProxyMainFailoverYAML, _ := yaml.Marshal(&dsProxyMainFailover)
			dsControllerMainFailoverYAML, _ := yaml.Marshal(&dsControllerMainFailover)
			pod1ControllerMainYAML, _ := yaml.Marshal(&pod1ControllerMain)
			pod2TerminatingControllerMainYAML, _ := yaml.Marshal(&pod2TerminatingControllerMain)
			pod1ProxyMainFailoverYAML, _ := yaml.Marshal(&pod1ProxyMainFailover)
			pod2ProxyMainFailoverYAML, _ := yaml.Marshal(&pod2ProxyMainFailover)
			pod1ControllerMainFailoverYAML, _ := yaml.Marshal(&pod1ControllerMainFailover)
			pod2ControllerMainFailoverYAML, _ := yaml.Marshal(&pod2ControllerMainFailover)

			clusterState := strings.Join([]string{string(dsControllerMainYAML), string(dsProxyMainFailoverYAML), string(dsControllerMainFailoverYAML), string(pod1ControllerMainYAML), string(pod2TerminatingControllerMainYAML), string(pod1ProxyMainFailoverYAML), string(pod2ProxyMainFailoverYAML), string(pod1ControllerMainFailoverYAML), string(pod2ControllerMainFailoverYAML)}, "---\n")

			f.BindingContexts.Set(f.KubeStateSet(clusterState))
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			_, _ = f.KubeClient().AppsV1().DaemonSets("d8-ingress-nginx").Create(context.TODO(), dsControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().AppsV1().DaemonSets("d8-ingress-nginx").Create(context.TODO(), dsProxyMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().AppsV1().DaemonSets("d8-ingress-nginx").Create(context.TODO(), dsControllerMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod1ControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod2TerminatingControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod1ProxyMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod2ProxyMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod1ControllerMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod2ControllerMainFailover, metav1.CreateOptions{})

			f.RunHook()
		})

		It("pod controller-main-1 must not be deleted", func() {
			Expect(f).To(ExecuteSuccessfully())

			pod1ControllerMainAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod1ControllerMain.Name, metav1.GetOptions{})
			pod2ControllerMainAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod2ControllerMain.Name, metav1.GetOptions{})
			pod1ProxyMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod1ProxyMainFailover.Name, metav1.GetOptions{})
			pod2ProxyMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod2ProxyMainFailover.Name, metav1.GetOptions{})
			pod1ControllerMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod1ControllerMainFailover.Name, metav1.GetOptions{})
			pod2ControllerMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod2ControllerMainFailover.Name, metav1.GetOptions{})

			Expect(pod1ControllerMainAfterRunHook).To(Equal(pod1ControllerMain))
			Expect(pod2ControllerMainAfterRunHook).To(Equal(pod2TerminatingControllerMain))
			Expect(pod1ProxyMainFailoverAfterRunHook).To(Equal(pod1ProxyMainFailover))
			Expect(pod2ProxyMainFailoverAfterRunHook).To(Equal(pod2ProxyMainFailover))
			Expect(pod1ControllerMainFailoverAfterRunHook).To(Equal(pod1ControllerMainFailover))
			Expect(pod2ControllerMainFailoverAfterRunHook).To(Equal(pod2ControllerMainFailover))
		})
	})

	Context("daemonset controller-main update scheduled", func() {
		BeforeEach(func() {
			dsControllerMain.Generation = 2
			dsControllerMain.Status.UpdatedNumberScheduled = 1
			pod2ControllerMain.Labels["pod-template-generation"] = "2"

			dsControllerMainYAML, _ := yaml.Marshal(&dsControllerMain)
			dsProxyMainFailoverYAML, _ := yaml.Marshal(&dsProxyMainFailover)
			dsControllerMainFailoverYAML, _ := yaml.Marshal(&dsControllerMainFailover)
			pod1ControllerMainYAML, _ := yaml.Marshal(&pod1ControllerMain)
			pod2ControllerMainYAML, _ := yaml.Marshal(&pod2ControllerMain)
			pod1ProxyMainFailoverYAML, _ := yaml.Marshal(&pod1ProxyMainFailover)
			pod2ProxyMainFailoverYAML, _ := yaml.Marshal(&pod2ProxyMainFailover)
			pod1ControllerMainFailoverYAML, _ := yaml.Marshal(&pod1ControllerMainFailover)
			pod2ControllerMainFailoverYAML, _ := yaml.Marshal(&pod2ControllerMainFailover)

			clusterState := strings.Join([]string{string(dsControllerMainYAML), string(dsProxyMainFailoverYAML), string(dsControllerMainFailoverYAML), string(pod1ControllerMainYAML), string(pod2ControllerMainYAML), string(pod1ProxyMainFailoverYAML), string(pod2ProxyMainFailoverYAML), string(pod1ControllerMainFailoverYAML), string(pod2ControllerMainFailoverYAML)}, "---\n")

			f.BindingContexts.Set(f.KubeStateSet(clusterState))
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			_, _ = f.KubeClient().AppsV1().DaemonSets("d8-ingress-nginx").Create(context.TODO(), dsControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().AppsV1().DaemonSets("d8-ingress-nginx").Create(context.TODO(), dsProxyMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().AppsV1().DaemonSets("d8-ingress-nginx").Create(context.TODO(), dsControllerMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod1ControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod2ControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod1ProxyMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod2ProxyMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod1ControllerMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod2ControllerMainFailover, metav1.CreateOptions{})

			f.RunHook()
		})

		It("pod controller-main-1 must be deleted", func() {
			Expect(f).To(ExecuteSuccessfully())

			pod1ControllerMainAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod1ControllerMain.Name, metav1.GetOptions{})
			pod2ControllerMainAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod2ControllerMain.Name, metav1.GetOptions{})
			pod1ProxyMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod1ProxyMainFailover.Name, metav1.GetOptions{})
			pod2ProxyMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod2ProxyMainFailover.Name, metav1.GetOptions{})
			pod1ControllerMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod1ControllerMainFailover.Name, metav1.GetOptions{})
			pod2ControllerMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod2ControllerMainFailover.Name, metav1.GetOptions{})

			Expect(pod1ControllerMainAfterRunHook).ToNot(Equal(pod1ControllerMain))
			Expect(pod2ControllerMainAfterRunHook).To(Equal(pod2ControllerMain))
			Expect(pod1ProxyMainFailoverAfterRunHook).To(Equal(pod1ProxyMainFailover))
			Expect(pod2ProxyMainFailoverAfterRunHook).To(Equal(pod2ProxyMainFailover))
			Expect(pod1ControllerMainFailoverAfterRunHook).To(Equal(pod1ControllerMainFailover))
			Expect(pod2ControllerMainFailoverAfterRunHook).To(Equal(pod2ControllerMainFailover))
		})
	})

	Context("daemonset controller-main update scheduled", func() {
		BeforeEach(func() {
			dsControllerMain.Generation = 2
			dsControllerMain.Status.UpdatedNumberScheduled = 1
			pod1ControllerMain.Labels["pod-template-generation"] = "2"

			dsControllerMainYAML, _ := yaml.Marshal(&dsControllerMain)
			dsProxyMainFailoverYAML, _ := yaml.Marshal(&dsProxyMainFailover)
			dsControllerMainFailoverYAML, _ := yaml.Marshal(&dsControllerMainFailover)
			pod1ControllerMainYAML, _ := yaml.Marshal(&pod1ControllerMain)
			pod2ControllerMainYAML, _ := yaml.Marshal(&pod2ControllerMain)
			pod1ProxyMainFailoverYAML, _ := yaml.Marshal(&pod1ProxyMainFailover)
			pod2ProxyMainFailoverYAML, _ := yaml.Marshal(&pod2ProxyMainFailover)
			pod1ControllerMainFailoverYAML, _ := yaml.Marshal(&pod1ControllerMainFailover)
			pod2ControllerMainFailoverYAML, _ := yaml.Marshal(&pod2ControllerMainFailover)

			clusterState := strings.Join([]string{string(dsControllerMainYAML), string(dsProxyMainFailoverYAML), string(dsControllerMainFailoverYAML), string(pod1ControllerMainYAML), string(pod2ControllerMainYAML), string(pod1ProxyMainFailoverYAML), string(pod2ProxyMainFailoverYAML), string(pod1ControllerMainFailoverYAML), string(pod2ControllerMainFailoverYAML)}, "---\n")
			f.BindingContexts.Set(f.KubeStateSet(clusterState))
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			_, _ = f.KubeClient().AppsV1().DaemonSets("d8-ingress-nginx").Create(context.TODO(), dsControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().AppsV1().DaemonSets("d8-ingress-nginx").Create(context.TODO(), dsProxyMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().AppsV1().DaemonSets("d8-ingress-nginx").Create(context.TODO(), dsControllerMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod1ControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod2ControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod1ProxyMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod2ProxyMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod1ControllerMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod2ControllerMainFailover, metav1.CreateOptions{})

			f.RunHook()
		})

		It("pod controller-main-2 must be deleted", func() {
			Expect(f).To(ExecuteSuccessfully())

			pod1ControllerMainAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod1ControllerMain.Name, metav1.GetOptions{})
			_, pod2ControllerMainError := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod2ControllerMain.Name, metav1.GetOptions{})
			Expect(errors.IsNotFound(pod2ControllerMainError)).To(BeTrue())
			pod1ProxyMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod1ProxyMainFailover.Name, metav1.GetOptions{})
			pod2ProxyMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod2ProxyMainFailover.Name, metav1.GetOptions{})
			pod1ControllerMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod1ControllerMainFailover.Name, metav1.GetOptions{})
			pod2ControllerMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod2ControllerMainFailover.Name, metav1.GetOptions{})

			Expect(pod1ControllerMainAfterRunHook).To(Equal(pod1ControllerMain))
			Expect(pod1ProxyMainFailoverAfterRunHook).To(Equal(pod1ProxyMainFailover))
			Expect(pod2ProxyMainFailoverAfterRunHook).To(Equal(pod2ProxyMainFailover))
			Expect(pod1ControllerMainFailoverAfterRunHook).To(Equal(pod1ControllerMainFailover))
			Expect(pod2ControllerMainFailoverAfterRunHook).To(Equal(pod2ControllerMainFailover))
		})
	})

	Context("daemonset proxy-main-failover update scheduled", func() {
		BeforeEach(func() {
			dsProxyMainFailover.Generation = 2
			dsProxyMainFailover.Status.UpdatedNumberScheduled = 1
			pod2ProxyMainFailover.Labels["pod-template-generation"] = "2"

			dsControllerMainYAML, _ := yaml.Marshal(&dsControllerMain)
			dsProxyMainFailoverYAML, _ := yaml.Marshal(&dsProxyMainFailover)
			dsControllerMainFailoverYAML, _ := yaml.Marshal(&dsControllerMainFailover)
			pod1ControllerMainYAML, _ := yaml.Marshal(&pod1ControllerMain)
			pod2ControllerMainYAML, _ := yaml.Marshal(&pod2ControllerMain)
			pod1ProxyMainFailoverYAML, _ := yaml.Marshal(&pod1ProxyMainFailover)
			pod2ProxyMainFailoverYAML, _ := yaml.Marshal(&pod2ProxyMainFailover)
			pod1ControllerMainFailoverYAML, _ := yaml.Marshal(&pod1ControllerMainFailover)
			pod2ControllerMainFailoverYAML, _ := yaml.Marshal(&pod2ControllerMainFailover)

			clusterState := strings.Join([]string{string(dsControllerMainYAML), string(dsProxyMainFailoverYAML), string(dsControllerMainFailoverYAML), string(pod1ControllerMainYAML), string(pod2ControllerMainYAML), string(pod1ProxyMainFailoverYAML), string(pod2ProxyMainFailoverYAML), string(pod1ControllerMainFailoverYAML), string(pod2ControllerMainFailoverYAML)}, "---\n")

			f.BindingContexts.Set(f.KubeStateSet(clusterState))
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			_, _ = f.KubeClient().AppsV1().DaemonSets("d8-ingress-nginx").Create(context.TODO(), dsControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().AppsV1().DaemonSets("d8-ingress-nginx").Create(context.TODO(), dsProxyMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().AppsV1().DaemonSets("d8-ingress-nginx").Create(context.TODO(), dsControllerMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod1ControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod2ControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod1ProxyMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod2ProxyMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod1ControllerMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod2ControllerMainFailover, metav1.CreateOptions{})

			f.RunHook()
		})

		It("pod proxy-main-failover-1 must be deleted", func() {
			Expect(f).To(ExecuteSuccessfully())

			pod1ControllerMainAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod1ControllerMain.Name, metav1.GetOptions{})
			pod2ControllerMainAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod2ControllerMain.Name, metav1.GetOptions{})
			_, pod1ProxyMainFailoverError := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod1ProxyMainFailover.Name, metav1.GetOptions{})
			Expect(errors.IsNotFound(pod1ProxyMainFailoverError)).To(BeTrue())
			pod2ProxyMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod2ProxyMainFailover.Name, metav1.GetOptions{})
			pod1ControllerMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod1ControllerMainFailover.Name, metav1.GetOptions{})
			pod2ControllerMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod2ControllerMainFailover.Name, metav1.GetOptions{})

			Expect(pod1ControllerMainAfterRunHook).To(Equal(pod1ControllerMain))
			Expect(pod2ControllerMainAfterRunHook).To(Equal(pod2ControllerMain))
			Expect(pod2ProxyMainFailoverAfterRunHook).To(Equal(pod2ProxyMainFailover))
			Expect(pod1ControllerMainFailoverAfterRunHook).To(Equal(pod1ControllerMainFailover))
			Expect(pod2ControllerMainFailoverAfterRunHook).To(Equal(pod2ControllerMainFailover))
		})
	})

	Context("daemonset proxy-main-failover update scheduled", func() {
		BeforeEach(func() {
			dsProxyMainFailover.Generation = 2
			dsProxyMainFailover.Status.UpdatedNumberScheduled = 1
			pod1ProxyMainFailover.Labels["pod-template-generation"] = "2"

			dsControllerMainYAML, _ := yaml.Marshal(&dsControllerMain)
			dsProxyMainFailoverYAML, _ := yaml.Marshal(&dsProxyMainFailover)
			dsControllerMainFailoverYAML, _ := yaml.Marshal(&dsControllerMainFailover)
			pod1ControllerMainYAML, _ := yaml.Marshal(&pod1ControllerMain)
			pod2ControllerMainYAML, _ := yaml.Marshal(&pod2ControllerMain)
			pod1ProxyMainFailoverYAML, _ := yaml.Marshal(&pod1ProxyMainFailover)
			pod2ProxyMainFailoverYAML, _ := yaml.Marshal(&pod2ProxyMainFailover)
			pod1ControllerMainFailoverYAML, _ := yaml.Marshal(&pod1ControllerMainFailover)
			pod2ControllerMainFailoverYAML, _ := yaml.Marshal(&pod2ControllerMainFailover)

			clusterState := strings.Join([]string{string(dsControllerMainYAML), string(dsProxyMainFailoverYAML), string(dsControllerMainFailoverYAML), string(pod1ControllerMainYAML), string(pod2ControllerMainYAML), string(pod1ProxyMainFailoverYAML), string(pod2ProxyMainFailoverYAML), string(pod1ControllerMainFailoverYAML), string(pod2ControllerMainFailoverYAML)}, "---\n")

			f.BindingContexts.Set(f.KubeStateSet(clusterState))
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			_, _ = f.KubeClient().AppsV1().DaemonSets("d8-ingress-nginx").Create(context.TODO(), dsControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().AppsV1().DaemonSets("d8-ingress-nginx").Create(context.TODO(), dsProxyMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().AppsV1().DaemonSets("d8-ingress-nginx").Create(context.TODO(), dsControllerMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod1ControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod2ControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod1ProxyMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod2ProxyMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod1ControllerMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod2ControllerMainFailover, metav1.CreateOptions{})

			f.RunHook()
		})

		It("pod proxy-main-failover-2 must be deleted", func() {
			Expect(f).To(ExecuteSuccessfully())

			pod1ControllerMainAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod1ControllerMain.Name, metav1.GetOptions{})
			pod2ControllerMainAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod2ControllerMain.Name, metav1.GetOptions{})
			pod1ProxyMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod1ProxyMainFailover.Name, metav1.GetOptions{})
			_, pod2ProxyMainFailoverError := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod2ProxyMainFailover.Name, metav1.GetOptions{})
			Expect(errors.IsNotFound(pod2ProxyMainFailoverError)).To(BeTrue())
			pod1ControllerMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod1ControllerMainFailover.Name, metav1.GetOptions{})
			pod2ControllerMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod2ControllerMainFailover.Name, metav1.GetOptions{})

			Expect(pod1ControllerMainAfterRunHook).To(Equal(pod1ControllerMain))
			Expect(pod2ControllerMainAfterRunHook).To(Equal(pod2ControllerMain))
			Expect(pod1ProxyMainFailoverAfterRunHook).To(Equal(pod1ProxyMainFailover))
			Expect(pod1ControllerMainFailoverAfterRunHook).To(Equal(pod1ControllerMainFailover))
			Expect(pod2ControllerMainFailoverAfterRunHook).To(Equal(pod2ControllerMainFailover))
		})
	})

	Context("all pods with CrashLoopBackOff status", func() {
		BeforeEach(func() {
			containerStatusCrashLoopBackOff := corev1.ContainerStatus{Name: "controller", State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}}}
			pod1ControllerMain.Status.ContainerStatuses = append(pod1ControllerMain.Status.ContainerStatuses, containerStatusCrashLoopBackOff)
			pod2ControllerMain.Status.ContainerStatuses = append(pod2ControllerMain.Status.ContainerStatuses, containerStatusCrashLoopBackOff)
			pod1ProxyMainFailover.Status.ContainerStatuses = append(pod1ProxyMainFailover.Status.ContainerStatuses, containerStatusCrashLoopBackOff)
			pod2ProxyMainFailover.Status.ContainerStatuses = append(pod2ProxyMainFailover.Status.ContainerStatuses, containerStatusCrashLoopBackOff)
			dsControllerMain.Generation = 2
			dsProxyMainFailover.Generation = 2

			dsControllerMainYAML, _ := yaml.Marshal(&dsControllerMain)
			dsProxyMainFailoverYAML, _ := yaml.Marshal(&dsProxyMainFailover)
			dsControllerMainFailoverYAML, _ := yaml.Marshal(&dsControllerMainFailover)
			pod1ControllerMainYAML, _ := yaml.Marshal(&pod1ControllerMain)
			pod2ControllerMainYAML, _ := yaml.Marshal(&pod2ControllerMain)
			pod1ProxyMainFailoverYAML, _ := yaml.Marshal(&pod1ProxyMainFailover)
			pod2ProxyMainFailoverYAML, _ := yaml.Marshal(&pod2ProxyMainFailover)
			pod1ControllerMainFailoverYAML, _ := yaml.Marshal(&pod1ControllerMainFailover)
			pod2ControllerMainFailoverYAML, _ := yaml.Marshal(&pod2ControllerMainFailover)

			clusterState := strings.Join([]string{string(dsControllerMainYAML), string(dsProxyMainFailoverYAML), string(dsControllerMainFailoverYAML), string(pod1ControllerMainYAML), string(pod2ControllerMainYAML), string(pod1ProxyMainFailoverYAML), string(pod2ProxyMainFailoverYAML), string(pod1ControllerMainFailoverYAML), string(pod2ControllerMainFailoverYAML)}, "---\n")

			f.BindingContexts.Set(f.KubeStateSet(clusterState))
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			_, _ = f.KubeClient().AppsV1().DaemonSets("d8-ingress-nginx").Create(context.TODO(), dsControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().AppsV1().DaemonSets("d8-ingress-nginx").Create(context.TODO(), dsProxyMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().AppsV1().DaemonSets("d8-ingress-nginx").Create(context.TODO(), dsControllerMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod1ControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod2ControllerMain, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod1ProxyMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod2ProxyMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod1ControllerMainFailover, metav1.CreateOptions{})
			_, _ = f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Create(context.TODO(), pod2ControllerMainFailover, metav1.CreateOptions{})
			f.RunHook()
		})

		It("all pods must be deleted", func() {
			Expect(f).To(ExecuteSuccessfully())

			pod1ControllerMainAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod1ControllerMain.Name, metav1.GetOptions{})
			pod2ControllerMainAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod2ControllerMain.Name, metav1.GetOptions{})
			pod1ProxyMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod1ProxyMainFailover.Name, metav1.GetOptions{})
			pod2ProxyMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod2ProxyMainFailover.Name, metav1.GetOptions{})
			pod1ControllerMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod1ControllerMainFailover.Name, metav1.GetOptions{})
			pod2ControllerMainFailoverAfterRunHook, _ := f.KubeClient().CoreV1().Pods("d8-ingress-nginx").Get(context.TODO(), pod2ControllerMainFailover.Name, metav1.GetOptions{})

			Expect(pod1ControllerMainAfterRunHook).ToNot(Equal(pod1ControllerMain))
			Expect(pod2ControllerMainAfterRunHook).ToNot(Equal(pod2ControllerMain))
			Expect(pod1ProxyMainFailoverAfterRunHook).ToNot(Equal(pod1ProxyMainFailover))
			Expect(pod2ProxyMainFailoverAfterRunHook).ToNot(Equal(pod2ProxyMainFailover))
			Expect(pod1ControllerMainFailoverAfterRunHook).To(Equal(pod1ControllerMainFailover))
			Expect(pod2ControllerMainFailoverAfterRunHook).To(Equal(pod2ControllerMainFailover))
		})
	})
})
