#!/bin/bash

function helm::adopt() {
  local module=$1
  local manifests_file=$2

  # Удостоверяемся, что релиз еще не установлен
  if [[ "$(kubectl -n antiopa get cm -l NAME=$module,OWNER=TILLER -o name)" != "" ]] ; then
    >&2 echo "Error! Release $module already exists!"
    return 1
  fi

  # Генерируем заглушку соответствующей длины
  local stub=$(printf '#%.0s' $(seq 1 $(cat $manifests_file | wc -c) ) )

  # Создаем временный chart и ставим его
  local chart_dir=$(mktemp -d)
  echo "name: $module" >> $chart_dir/Chart.yaml
  echo "version: 0.1.0" >> $chart_dir/Chart.yaml
  mkdir $chart_dir/templates
  cat > $chart_dir/templates/adopt.yaml <<- YAML
$stub
---
apiVersion: v1
kind: ConfigMap
metadata:
  namespace: antiopa
  name: helm-adoption-$module
YAML

  # Ставим временный chart (и удаляем директорию, она больше не нужна)
  if TILLER_NAMESPACE=antiopa helm install -n $module --namespace antiopa $chart_dir ; then
    rm -rf $chart_dir
  else
    TILLER_NAMESPACE=antiopa helm delete --purge $module
    rm -rf $chart_dir
    return 1
  fi

  # Подменяем содержимое релиза
  local release=$(kubectl -n antiopa get cm/$module.v1 -o json | jq .data.release -r)
  local updated_release=$(echo $release | base64 -d | zcat | sed "s/$stub/$(<$manifests_file sed -e 's/[\&/]/\\&/g' -e 's/$/\\n/' | tr -d '\n')/" | gzip -9 | base64 | tr -d '\n')
  kubectl patch -n antiopa cm/nginx-ingress.v1 -p '{"data":{"release":"'$updated_release'"}}'

  # Проверяем, что ничего не поломалось
  if ! TILLER_NAMESPACE=antiopa helm list -q | grep '^'$module'$' > /dev/null ; then
    >&2 echo "Error! Adoption of $module failed!"

    kubectl -n antiopa delete cm helm-adoption-$module
    kubectl -n antiopa delete cm -l NAME=$module,OWNER=TILLER
    return 1
  fi
}
