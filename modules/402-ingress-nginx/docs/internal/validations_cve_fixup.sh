#!/bin/bash

# Copyright 2025 Flant JSC
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

set -e
echo "Creating certificates for fixer"

openssl req -x509 -newkey rsa:1024 -days 3650 -nodes -keyout ca.key -out ca.crt -subj "/CN=d8-ingress-validation-cve-fixer.d8-system.svc"

cat >csr.conf<<EOF
default_bits = 1024
prompt = no
default_md = sha256
req_extensions = req_ext
distinguished_name = dn

[ dn ]
CN = d8-ingress-validation-cve-fixer.d8-system.svc

[ req_ext ]
subjectAltName = @alt_names

[ v3_ext ]
authorityKeyIdentifier=keyid,issuer:always
basicConstraints=CA:FALSE
keyUsage=keyEncipherment,dataEncipherment
extendedKeyUsage=serverAuth,clientAuth
subjectAltName=@alt_names
[alt_names]
DNS.1 = d8-ingress-validation-cve-fixer
DNS.2 = d8-ingress-validation-cve-fixer.d8-system
DNS.3 = d8-ingress-validation-cve-fixer.d8-system.svc
DNS.4 = d8-ingress-validation-cve-fixer.d8-system.svc.cluster
EOF

openssl genrsa -out tls.key 1024
openssl req -new -key tls.key -out req.csr -config csr.conf

openssl x509 -req -in req.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out tls.crt -days 3650 -extensions v3_ext -extfile csr.conf -sha256

echo "Server's signed certificate"
openssl x509 -in tls.crt -noout -text

echo "Creating fixer RBAC"

kubectl apply -f <(cat <<EOF
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: d8-ingress-validation-cve-fixer
  namespace: d8-system
EOF
)

kubectl apply -f <(cat <<EOF
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:ingress-validation-cve-fixer
rules:
- apiGroups:
  - admissionregistration.k8s.io
  resources:
  - mutatingwebhookconfigurations
  verbs:
  - create
  - list
  - update
EOF
)

kubectl apply -f <(cat <<EOF
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: d8:ingress-validation-cve-fixer
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: d8:ingress-validation-cve-fixer
subjects:
- kind: ServiceAccount
  name: d8-ingress-validation-cve-fixer
  namespace: d8-system
EOF
)

echo "Creating fixer tls secret"

cacrt=$(cat ca.crt | base64 -w0)
tlscrt=$(cat tls.crt | base64 -w0)
tlskey=$(cat tls.key | base64 -w0)

kubectl apply -f <(cat <<EOF
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-ingress-validation-cve-fixer
  namespace: d8-system
type: kubernetes.io/tls
data:
  ca.crt: $cacrt
  tls.crt: $tlscrt
  tls.key: $tlskey
EOF
)

echo "Creating fixer service"

kubectl apply -f <(cat <<EOF
---
apiVersion: v1
kind: Service
metadata:
  name: d8-ingress-validation-cve-fixer
  namespace: d8-system
spec:
  type: ClusterIP
  ports:
    - name: mutating-http
      port: 443
      targetPort: 9680
      protocol: TCP
  selector:
    app: ingress-validation-cve-fixer
EOF
)

echo "Creating fixer configmap"

kubectl apply -f <(cat <<EOF
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-ingress-validation-cve-fixer
  namespace: d8-system
data:
  disable-ingress-validation.sh: |
    #!/usr/bin/env bash

    source /shell_lib.sh

    function __config__(){
        cat <<EOF
      configVersion: v1
      kubernetesMutating:
      - name: disable.ingress.validation
        namespace:
          labelSelector:
            matchExpressions:
            - key: kubernetes.io/metadata.name
              operator: In
              values:
              - d8-ingress-nginx
        labelSelector:
          matchLabels:
            app: controller
        rules:
        - apiGroups:   [""]
          apiVersions: ["v1"]
          operations:  ["CREATE"]
          resources:   ["pods"]
          scope:       "Namespaced"
    EOF
    }
    
    function __on_mutating::disable.ingress.validation() {
      controllerContainerIndex=\$(context::jq -r '.review.request.object.spec.containers | to_entries | .[] | select(.value.name == "controller") | .key')
      if [[ "\$controllerContainerIndex" != "" ]]; then
        validatingWebhookArgIndex=\$(context::jq --arg i "\$controllerContainerIndex" -r '.review.request.object.spec.containers[\$i | tonumber].args | index("--validating-webhook=:8443")')
        if [[ "\$validatingWebhookArgIndex" != "null" ]]; then
          patch=\$(echo "[{\"op\": \"remove\", \"path\": \"/spec/containers/\$controllerContainerIndex/args/\$validatingWebhookArgIndex\"}]" | base64 -w0)
          cat <<EOF > \$VALIDATING_RESPONSE_PATH
    {"allowed":true, "patch": "\$patch"}
    EOF
        else
          cat <<EOF > \$VALIDATING_RESPONSE_PATH
    {"allowed":true}
    EOF
        fi
      else
        cat <<EOF > \$VALIDATING_RESPONSE_PATH
    {"allowed":true}
    EOF
      fi
    }

    hook::run \$@
EOF
)

registryAddress=$(kubectl -n d8-system get secrets deckhouse-registry -o json | jq .data.address -r | base64 -d)
registryPath=$(kubectl -n d8-system get secrets deckhouse-registry -o json | jq .data.path -r | base64 -d)
shellOperatorHash=$(kubectl -n d8-system exec deployments/deckhouse -- cat modules/040-node-manager/images_digests.json | jq -r .common.shellOperator)
image=$registryAddress$registryPath@$shellOperatorHash

echo "Creating fixer deployment using \"$image\" as the image for the fixer"

kubectl apply -f <(cat <<EOF 
---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: ingress-validation-cve-fixer
  name: d8-ingress-validation-cve-fixer
  namespace: d8-system
spec:
  progressDeadlineSeconds: 600
  replicas: 2
  revisionHistoryLimit: 10
  selector:
    matchLabels:
      app: ingress-validation-cve-fixer
  strategy:
    rollingUpdate:
      maxSurge: 25%
      maxUnavailable: 25%
    type: RollingUpdate
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: ingress-validation-cve-fixer
    spec:
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
          - labelSelector:
              matchLabels:
                app: ingress-validation-cve-fixer
            topologyKey: kubernetes.io/hostname
      containers:
      - image: $image
        imagePullPolicy: IfNotPresent
        name: fixer
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
        command:
        - /shell-operator
        env:
        - name: SHELL_OPERATOR_NAMESPACE
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.namespace
        - name: VALIDATING_WEBHOOK_SERVICE_NAME
          value: d8-ingress-validation-cve-fixer
        - name: VALIDATING_WEBHOOK_CONFIGURATION_NAME
          value: d8-ingress-validation-cve-fixer
        - name: VALIDATING_WEBHOOK_SERVER_CERT
          value: /certs/tls.crt
        - name: VALIDATING_WEBHOOK_SERVER_KEY
          value: /certs/tls.key
        - name: VALIDATING_WEBHOOK_CA
          value: /certs/ca.crt
        livenessProbe:
          failureThreshold: 3
          httpGet:
            path: /healthz
            port: 9680
            scheme: HTTPS
          initialDelaySeconds: 1
          periodSeconds: 10
          successThreshold: 1
          timeoutSeconds: 1
        volumeMounts:
        - mountPath: /certs/
          name: certs
          readOnly: true
        - mountPath: /var/run
          name: run
        - mountPath: /tmp
          name: tmp
        - mountPath: /hooks/disable-ingress-validation.sh
          name: script
          subPath: disable-ingress-validation.sh
        - mountPath: /.kube
          name: kube
        ports:
        - containerPort: 9680
          name: mutating-http
          protocol: TCP
        resources:
          requests:
            cpu: 50m
            ephemeral-storage: 60Mi
            memory: 100Mi
      dnsPolicy: ClusterFirst
      restartPolicy: Always
      schedulerName: default-scheduler
      imagePullSecrets:
      - name: deckhouse-registry
      nodeSelector:
        node-role.kubernetes.io/control-plane: ""
      priorityClassName: system-cluster-critical
      securityContext:
        runAsGroup: 64535
        runAsNonRoot: true
        runAsUser: 64535
      serviceAccount: d8-ingress-validation-cve-fixer
      serviceAccountName: d8-ingress-validation-cve-fixer
      terminationGracePeriodSeconds: 30
      volumes:
      - name: certs
        secret:
          defaultMode: 420
          secretName: d8-ingress-validation-cve-fixer
      - emptyDir: {}
        name: run
      - emptyDir:
          medium: Memory
        name: tmp
      - emptyDir:
          medium: Memory
        name: kube
      - name: script
        configMap:
          name: d8-ingress-validation-cve-fixer
          defaultMode: 493
      tolerations:
      - key: node-role.kubernetes.io/master
      - key: node-role.kubernetes.io/control-plane
      - key: dedicated.deckhouse.io
        operator: Exists
      - key: dedicated
        operator: Exists
      - key: DeletionCandidateOfClusterAutoscaler
      - key: ToBeDeletedByClusterAutoscaler
      - key: drbd.linbit.com/lost-quorum
      - key: drbd.linbit.com/force-io-error
      - key: drbd.linbit.com/ignore-fail-over
      - effect: NoSchedule
        key: node.deckhouse.io/uninitialized
        operator: Exists
      - key: ToBeDeletedTaint
        operator: Exists
      - effect: NoSchedule
        key: node.deckhouse.io/csi-not-bootstrapped
        operator: Exists
      - key: node.kubernetes.io/not-ready
      - key: node.kubernetes.io/out-of-disk
      - key: node.kubernetes.io/memory-pressure
      - key: node.kubernetes.io/disk-pressure
      - key: node.kubernetes.io/pid-pressure
      - key: node.kubernetes.io/unreachable
      - key: node.kubernetes.io/network-unavailable
EOF
)
