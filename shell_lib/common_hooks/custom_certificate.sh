#!/bin/bash

function common_hooks::custom_certificate::config() {
  echo '
{
  "afterHelm": 10,
  "onKubernetesEvent": [
    {
      "kind": "secret",
      "event": [
        "update"
      ],
      "namespaceSelector": {
        "matchNames": [
          "antiopa"
        ]
      },
      "jqFilter": "[.data.\"tls.crt\", .data.\"tls.key\"]"
    }
  ]
}'
}

# $1 — имя namespace, куда надо скопировать секрет
function common_hooks::custom_certificate::main() {

  module_name=$(module::name)
  secret_name=$(values::get_first_defined ${module_name}.certificateForIngress.customCertificateSecretName global.certificateForIngress.customCertificateSecretName)

  if [ "${secret_name}" != "false" ] && [ ! -z "${secret_name}" ] ; then
    if kubectl -n antiopa get secret ${secret_name} ; then
      kubectl -n antiopa get secret ${secret_name} -o json | \
        jq -r ".metadata.namespace=\"$1\" | .metadata.name=\"ingress-tls\" |
          .metadata |= with_entries(select([.key] | inside([\"name\", \"namespace\", \"labels\"])))" \
        | kubectl::replace_or_create
    else
      >&2 echo "You use the certificateForIngress.customCertificateSecretName, but there is no ${secret_name} secret in antiopa namespace"
      exit 1
    fi
  fi
 
}

