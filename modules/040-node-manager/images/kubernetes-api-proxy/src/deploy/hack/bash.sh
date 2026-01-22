#!/usr/bin/env bash

openssl genrsa -out kap.key 2048
openssl req -new -key kap.key -out kap.csr -subj="/O=system:kubernetes-api-proxy/CN=system:kubernetes-api-proxy"

kubectl delete csr kap-csr || echo "CSR not found"
cat <<EOF | kubectl create -f -
apiVersion: certificates.k8s.io/v1
kind: CertificateSigningRequest
metadata:
  name: kap-csr
spec:
  request: $(cat kap.csr | base64 | tr -d '\n')
  signerName: kubernetes.io/kube-apiserver-client
  usages:
  - client auth
EOF

kubectl certificate approve kap-csr
kubectl get csr kap-csr -o jsonpath='{.status.certificate}' | base64 --decode > kap.crt
