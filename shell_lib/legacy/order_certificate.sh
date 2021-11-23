#!/bin/bash

# Copyright 2021 Flant JSC
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

function legacy::common_hooks::certificates::order_certificate::config() {
  cat << EOF
    configVersion: v1
    beforeHelm: 5
    schedule:
    - name: order_certificate
      queue: /modules/$(module::name::kebab_case)/order_certificate
      crontab: "42 4 * * *"
EOF
}

# $1 - имя namespace, для которого надо сгенерировать сертификат
# $2 - название секрета, куда сложить сгенерированный сертификат
# $3 - common_name генерируемого сертификата (или имя пользователя)
# $4 - путь в values, куда необходимо записать сертификат и ключ
# $5 - группа пользователя, в которой он состоит
function legacy::common_hooks::certificates::order_certificate::main() {
  namespace=$1
  secret_name=$2
  common_name=$3
  value_name=$4
  group=${5:-""}

  module_name=$(module::name::camel_case)

  if kubectl -n ${namespace} get secret/${secret_name} > /dev/null 2> /dev/null ; then
    # Проверяем срок действия
    cert=$(kubectl -n ${namespace} get secret/${secret_name} -o json | jq -rc '.data."tls.crt" // .data."client.crt"' | base64 -d)
    not_after=$(echo "$cert" | cfssl certinfo -cert - | jq .not_after -r | sed 's/\([0-9]\{4\}-[0-9]\{2\}-[0-9]\{2\}\)T\([0-9]\{2\}:[0-9]\{2\}:[0-9]\{2\}\).*/\1 \2/')
    valid_for=$(expr $(date --date="$not_after" +%s) - $(date +%s))

    # Если сертификат будет действителен еще 10 дней - пропускаем обновление
    if [[ "$valid_for" -ge 864000 ]] ; then
      values::set ${module_name}.$value_name "{}"
      values::set ${module_name}.$value_name.certificate "$(echo "$cert")"
      values::set ${module_name}.$value_name.key "$(kubectl -n ${namespace} get secret/${secret_name} -o json | jq -rc '.data."tls.key" // .data."client.key"' | base64 -d)"
      values::unset ${module_name}.$value_name.certificate_updated
      return 0
    fi
  fi

  # Удаляем CSR, если существовал раньше
  if kubectl get csr/${common_name} > /dev/null 2> /dev/null ; then
    kubectl delete csr/${common_name}
  fi

  if [[ "$group" != "" ]]; then
    group=$(jq -rcR '[{"O": . }]' <<< ${group})
  fi

  # Генерируем CSR
  cfssl_result=$(jo CN=${common_name} names="$group" key="$(jo algo=ecdsa size=256)" | cfssl genkey -)
  cfssl_result_csr=$(echo "$cfssl_result" | jq .csr -r | base64 | tr -d '\n')
  csr=$(cat <<EOF
apiVersion: certificates.k8s.io/v1
kind: CertificateSigningRequest
metadata:
  name: ${common_name}
spec:
  signerName: kubernetes.io/kube-apiserver-client
  request: ${cfssl_result_csr}
  usages:
  - digital signature
  - key encipherment
  - client auth
EOF
)

  # Создаем CSR и сразу его подтверждаем
  echo "$csr" | kubectl create -f -
  echo "$csr" | kubectl certificate approve -f -

  # Дожидаемся подписанного сертификата, скачеваем его и удаляем CSR
  for i in $(seq 1 120); do
    if [[ "$(kubectl get csr/${common_name} -o json | jq '.status | has("certificate")')" == "true" ]] ; then
      break
    fi

    echo "Wait for csr/${common_name} approval"
    sleep 1
  done
  if [[ $i -gt 120 ]] ; then
    >&2 echo "Timeout waiting for csr/${common_name} approval"
    return 1
  fi
  cert=$(kubectl get csr/${common_name} -o jsonpath='{.status.certificate}')
  kubectl delete csr/${common_name}

  values::set ${module_name}.$value_name "{}"
  values::set ${module_name}.$value_name.certificate "$(echo "$cert" | base64 -d)"
  values::set ${module_name}.$value_name.key "$(echo "$cfssl_result" | jq .key -r)"
  values::set ${module_name}.$value_name.certificate_updated "true"
}
