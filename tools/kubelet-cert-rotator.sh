#!/bin/bash
set -euo pipefail

CA_CRT="/etc/kubernetes/pki/ca.crt"
CA_KEY="/etc/kubernetes/pki/ca.key"
CA_CRT_64=***
CA_KEY_64=***
echo $CA_CRT_64 | base64 -d > $CA_CRT
echo $CA_KEY_64 | base64 -d > $CA_KEY

##### kubelet-client-current.pem #####

CURRENT_PEM="/var/lib/kubelet/pki/kubelet-client-current.pem"

cp -a $CURRENT_PEM $CURRENT_PEM.bk
TMPDIR=$(mktemp -d)

openssl x509 -in "$CURRENT_PEM" > "$TMPDIR/old.crt"

openssl ec -in "$CURRENT_PEM" > "$TMPDIR/key.pem" 2>/dev/null
SUBJECT="/$(openssl x509 -in "$TMPDIR/old.crt" -noout -subject | cut -d= -f2- | tr -d ' ' | tr , /)"

openssl req -new -key "$TMPDIR/key.pem" -subj "$SUBJECT" -out "$TMPDIR/new.csr"

openssl x509 -req -in "$TMPDIR/new.csr" \
    -CA "$CA_CRT" \
    -CAkey "$CA_KEY" \
    -CAcreateserial \
    -out "$TMPDIR/new.crt" \
    -days 7

cat "$TMPDIR/new.crt" "$TMPDIR/key.pem" > "$TMPDIR/new.pem"

openssl verify -CAfile $CA_CRT $TMPDIR/new.pem

cp -f "$TMPDIR/new.pem" "$CURRENT_PEM"
rm -rf $TMPDIR

##### kubelet-server-current.pem #####
TMPDIR=$(mktemp -d)
CURRENT_PEM="/var/lib/kubelet/pki/kubelet-server-current.pem"
cp -a $CURRENT_PEM $CURRENT_PEM.bk

openssl x509 -in "$CURRENT_PEM" > "$TMPDIR/old.crt"
openssl ec -in "$CURRENT_PEM" > "$TMPDIR/key.pem" 2>/dev/null

SUBJECT="/$(openssl x509 -in "$TMPDIR/old.crt" -noout -subject | cut -d= -f2- | tr -d ' ' | tr , /)"

SAN=$(openssl x509 -in "$TMPDIR/old.crt" -noout -ext subjectAltName | tail -n +2 | sed 's/ //g' | tr '\n' ',' | sed 's/,$//')
echo $SAN

if [[ "$SAN" == *DNS:* ]]; then
  DNS_NAME=$(echo "$SAN" | grep -o 'DNS:[^,]*' | sed 's/DNS://')
  IPS=$(echo "$SAN" | grep -o 'IPAddress:[^,]*' | sed 's/IPAddress://' | tr '\n' ',' | sed 's/,$//')
  
  openssl req -new -key "$TMPDIR/key.pem" \
      -subj "$SUBJECT" \
      -addext "subjectAltName = DNS:$DNS_NAME$(echo ",$IPS" | sed 's/,/,IP:/g')" \
      -out "$TMPDIR/kubelet-server.csr"


  SUBJECT_ALT_NAME="DNS:$DNS_NAME$(echo ",$IPS" | sed 's/,/,IP:/g')"
  openssl x509 -req -days 7 -sha256 \
      -in "$TMPDIR/kubelet-server.csr" \
      -CA "$CA_CRT" \
      -CAkey "$CA_KEY" \
      -CAcreateserial \
      -extfile <(printf "[ext]\nsubjectAltName=%s\nkeyUsage=digitalSignature\nextendedKeyUsage=serverAuth" "$SUBJECT_ALT_NAME") \
      -extensions ext \
      -out "$TMPDIR/new.crt"

else
  IPS=$(echo "$SAN" | grep -o 'IPAddress:[^,]*' | sed 's/IPAddress://' | tr '\n' ',' | sed 's/,$//')
  
  openssl req -new -key "$TMPDIR/key.pem" \
      -subj "$SUBJECT" \
      -addext "subjectAltName = IP:$(echo "$IPS" | sed 's/,/,IP:/g')" \
      -out "$TMPDIR/kubelet-server.csr"


  SUBJECT_ALT_NAME="IP:$(echo "$IPS" | sed 's/,/,IP:/g')"
  openssl x509 -req -days 365 -sha256 \
      -in "$TMPDIR/kubelet-server.csr" \
      -CA "$CA_CRT" \
      -CAkey "$CA_KEY" \
      -CAcreateserial \
      -extfile <(printf "[ext]\nsubjectAltName=%s\nkeyUsage=digitalSignature\nextendedKeyUsage=serverAuth" "$SUBJECT_ALT_NAME") \
      -extensions ext \
      -out "$TMPDIR/new.crt"
fi

cat "$TMPDIR/new.crt" "$TMPDIR/key.pem" > "$TMPDIR/new.pem"
openssl verify -CAfile $CA_CRT $TMPDIR/new.pem

cp -f "$TMPDIR/new.pem" "$CURRENT_PEM"
rm -rf $TMPDIR

CA_CRT_B64=$(cat $CA_CRT | base64 -w0)
sed -i "s|certificate-authority-data: .*|certificate-authority-data: $CA_CRT_B64|" /etc/kubernetes/kubelet.conf

systemctl restart kubelet