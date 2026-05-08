#!/usr/bin/env bash
set -euo pipefail

NAME="stable-identity-etcd-witness-0"
STS_NAME="stable-identity-etcd-witness"
NAMESPACE="kube-system"

FQDN_1="${NAME}.${STS_NAME}.${NAMESPACE}.svc"
FQDN_2="${FQDN_1}.cluster.local"

OUT_DIR="${PWD}/pki"
CA_CRT="/etc/kubernetes/pki/etcd/ca.crt"
CA_KEY="/etc/kubernetes/pki/etcd/ca.key"
DAYS="3650"

umask 077
mkdir -p "${OUT_DIR}"

cp "${CA_CRT}" "${OUT_DIR}/ca.crt"

cat > "${OUT_DIR}/server-openssl.cnf" <<EOF
[ req ]
distinguished_name = dn
prompt = no
req_extensions = v3_req

[ dn ]
CN = ${NAME}

[ v3_req ]
basicConstraints = CA:FALSE
keyUsage = critical, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[ alt_names ]
DNS.1 = ${NAME}
DNS.2 = ${NAME}.${STS_NAME}
DNS.3 = ${NAME}.${STS_NAME}.${NAMESPACE}
DNS.4 = ${FQDN_1}
DNS.5 = ${FQDN_2}
DNS.6 = localhost
IP.1 = 127.0.0.1
EOF

cat > "${OUT_DIR}/peer-openssl.cnf" <<EOF
[ req ]
distinguished_name = dn
prompt = no
req_extensions = v3_req

[ dn ]
CN = ${NAME}

[ v3_req ]
basicConstraints = CA:FALSE
keyUsage = critical, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth, clientAuth
subjectAltName = @alt_names

[ alt_names ]
DNS.1 = ${NAME}
DNS.2 = ${NAME}.${STS_NAME}
DNS.3 = ${NAME}.${STS_NAME}.${NAMESPACE}
DNS.4 = ${FQDN_1}
DNS.5 = ${FQDN_2}
EOF

openssl genrsa -out "${OUT_DIR}/server.key" 4096
openssl req -new \
  -key "${OUT_DIR}/server.key" \
  -out "${OUT_DIR}/server.csr" \
  -config "${OUT_DIR}/server-openssl.cnf"

openssl x509 -req \
  -in "${OUT_DIR}/server.csr" \
  -CA "${CA_CRT}" \
  -CAkey "${CA_KEY}" \
  -CAcreateserial \
  -out "${OUT_DIR}/server.crt" \
  -days "${DAYS}" \
  -extensions v3_req \
  -extfile "${OUT_DIR}/server-openssl.cnf"

openssl genrsa -out "${OUT_DIR}/peer.key" 4096
openssl req -new \
  -key "${OUT_DIR}/peer.key" \
  -out "${OUT_DIR}/peer.csr" \
  -config "${OUT_DIR}/peer-openssl.cnf"

openssl x509 -req \
  -in "${OUT_DIR}/peer.csr" \
  -CA "${CA_CRT}" \
  -CAkey "${CA_KEY}" \
  -CAcreateserial \
  -out "${OUT_DIR}/peer.crt" \
  -days "${DAYS}" \
  -extensions v3_req \
  -extfile "${OUT_DIR}/peer-openssl.cnf"

rm -f \
  "${OUT_DIR}/server.csr" \
  "${OUT_DIR}/peer.csr" \
  "${OUT_DIR}/server-openssl.cnf" \
  "${OUT_DIR}/peer-openssl.cnf" \
  "${OUT_DIR}/ca.srl"

echo "PKI generated in ${OUT_DIR}"
echo
echo "Verify SANs:"
openssl x509 -in "${OUT_DIR}/server.crt" -noout -text | sed -n '/Subject Alternative Name/,+1p'
echo
openssl x509 -in "${OUT_DIR}/peer.crt" -noout -text | sed -n '/Subject Alternative Name/,+1p'
