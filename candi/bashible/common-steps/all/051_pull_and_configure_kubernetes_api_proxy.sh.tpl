# Copyright 2023 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

mkdir -p /etc/kubernetes/manifests

bb-set-proxy

{{- $kubernetes_api_proxy_image := printf "%s@%s" .registry.imagesBase ( index .images.nodeManager "kubernetesApiProxy" ) }}

{{- if or ( eq .cri "Containerd") ( eq .cri "ContainerdV2") }}
  {{- $kubernetes_api_proxy_image = "deckhouse.local/images:kubernetes-api-proxy" }}
{{- end }}

{{- /* TODO: Probably, we can add it only on .runType "Normal", 'cause we don't have certificates already, it will always fail down */ }}
{{- /* TODO: Or we need some tests behavior in scenarios, when no certs is loaded */ }}
{{- /* TODO: Probably, we need to add ability for watch certs in runTime for changing them after time is come */ }}
{{- /* TODO: Also, we don't need in KAP before "Normal", 'cause Kubelet settings for KAP-addressing applies only after switch to .runType "Normal" */ }}

bb-sync-file /etc/kubernetes/manifests/kubernetes-api-proxy.yaml - << EOF
apiVersion: v1
kind: Pod
metadata:
  labels:
    component: kubernetes-api-proxy
    tier: control-plane
  name: kubernetes-api-proxy
  namespace: kube-system
spec:
  priorityClassName: system-node-critical
  priority: 2000001000
  hostNetwork: true
  dnsPolicy: ClusterFirstWithHostNet
  volumes:
    - name: ca-cert
      hostPath:
        path: /etc/kubernetes/kubernetes-api-proxy/ca.crt
        type: File
    - name: cl-cert
      hostPath:
        path: /etc/kubernetes/kubernetes-api-proxy/apl.crt
        type: File
    - name: cl-key
      hostPath:
        path: /etc/kubernetes/kubernetes-api-proxy/apl.key
        type: File
    - name: upstreams
      hostPath:
        path: /etc/kubernetes/kubernetes-api-proxy/upstreams.json
        type: FileOrCreate
  containers:
    - name: kubernetes-api-proxy
      securityContext:
        allowPrivilegeEscalation: false
        capabilities:
          drop:
            - ALL
        readOnlyRootFilesystem: true
        runAsGroup: 0
        runAsNonRoot: false
        runAsUser: 0
        seccompProfile:
          type: RuntimeDefault
      image: kubernetes-api-proxy:0.0.1
      imagePullPolicy: IfNotPresent
      args:
        - "--listen-address=0.0.0.0"
        - "--listen-port=6445"
        - "--health-listen=:6480"
        - "--log-level=debug"
        - "--as-static-pod=true"
        - "--fallback-file=/var/run/kubernetes.io/kubernetes-api-proxy/upstreams.json"
      ports:
        - name: https
          containerPort: 6445
          hostPort: 6445
          protocol: TCP
        - name: health
          containerPort: 6480
          protocol: TCP
      readinessProbe:
        httpGet:
          path: /readyz
          port: health
        initialDelaySeconds: 2
        periodSeconds: 5
      livenessProbe:
        httpGet:
          path: /healthz
          port: health
        initialDelaySeconds: 2
        periodSeconds: 10
      resources:
        requests:
          cpu: 50m
          memory: 64Mi
        limits:
          cpu: 500m
          memory: 256Mi
      volumeMounts:
        - name: cl-cert
          mountPath: /var/run/kubernetes.io/kubernetes-api-proxy/cl.crt
          readOnly: true
        - name: cl-key
          mountPath: /var/run/kubernetes.io/kubernetes-api-proxy/cl.key
          readOnly: true
        - name: ca-cert
          mountPath: /var/run/kubernetes.io/kubernetes-api-proxy/ca.crt
          readOnly: true
        - name: upstreams
          mountPath: /var/run/kubernetes.io/kubernetes-api-proxy/upstreams.json
EOF

bb-unset-proxy
