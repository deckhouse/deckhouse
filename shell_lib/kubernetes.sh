#!/bin/bash

# stdin resource_spec
function kubernetes::create_json() {
  cat | jq -c '{op: "Create", resourceSpec: (. | tojson)}' >> ${D8_KUBERNETES_PATCH_SET_FILE}
}

# stdin resource_spec
function kubernetes::create_yaml() {
  cat | yq r -j - | jq -c '{op: "Create", resourceSpec: (. | tojson)}' >> ${D8_KUBERNETES_PATCH_SET_FILE}
}

# stdin resource_spec
function kubernetes::create_if_not_exists_json() {
  cat | jq -c '{op: "CreateIfNotExists", resourceSpec: (. | tojson)}' >> ${D8_KUBERNETES_PATCH_SET_FILE}
}

# stdin resource_spec
function kubernetes::create_if_not_exists_yaml() {
  cat | yq r -j - | jq -c '{op: "CreateIfNotExists", resourceSpec: (. | tojson)}' >> ${D8_KUBERNETES_PATCH_SET_FILE}
}

# stdin resource_spec
function kubernetes::replace_or_create_json() {
  cat | jq -c '{op: "ReplaceOrCreate", resourceSpec: (. | tojson)}' >> ${D8_KUBERNETES_PATCH_SET_FILE}
}

# stdin resource_spec
function kubernetes::replace_or_create_yaml() {
  cat | yq r -j - | jq -c '{op: "ReplaceOrCreate", resourceSpec: (. | tojson)}' >> ${D8_KUBERNETES_PATCH_SET_FILE}
}

# $1 namespace
# $2 resource (pod/mypod-aacc12)
# $3 jqFilter
function kubernetes::patch_jq() {
  jq -nc --arg jqFilter "${3}" '{op: "JQPatch", namespace: "'${1}'", resource: "'${2}'", jqFilter: $jqFilter}' >> ${D8_KUBERNETES_PATCH_SET_FILE}
}

# $1 namespace
# $2 resource (pod/mypod-aacc12)
function kubernetes::delete_if_exists() {
  jq -nc '{op: "DeleteIfExists", namespace: "'${1}'", resource: "'${2}'"}' >> ${D8_KUBERNETES_PATCH_SET_FILE}
}

# $1 namespace
# $2 resource (pod/mypod-aacc12)
function kubernetes::delete_if_exists::non_blocking() {
  jq -nc '{op: "DeleteIfExistsNonBlocking", namespace: "'${1}'", resource: "'${2}'"}' >> ${D8_KUBERNETES_PATCH_SET_FILE}
}

# $1 namespace
# $2 apiVersion (i.e. deckhouse.io/v1alpha1)
# $3 plural kind (i.e. openstackmachineclasses)
# $4 resourceName (i.e. some-resource-aabbcc)
# $5 new status json
function kubernetes::status::patch() {
  jq -nc --arg newStatus "${5}" '{op: "StatusPatch", namespace: "'${1}'", apiVersion: "'${2}'", kind: "'${3}'", resourceName: "'${4}'", newStatus: $newStatus}' >> ${D8_KUBERNETES_PATCH_SET_FILE}
}

# $1 namespace
# $2 apiVersion (i.e. deckhouse.io/v1alpha1)
# $3 plural kind (i.e. openstackmachineclasses)
# $4 resourceName (i.e. some-resource-aabbcc)
# $5 new status json
function kubernetes::status::put() {
  jq -nc --arg newStatus "${5}" '{op: "StatusPut", namespace: "'${1}'", apiVersion: "'${2}'", kind: "'${3}'", resourceName: "'${4}'", newStatus: $newStatus}' >> ${D8_KUBERNETES_PATCH_SET_FILE}
}

function kubernetes::_init_patch_set() {
  if [ -n "${D8_TEST_KUBERNETES_PATCH_SET_FILE-}" ]; then
    echo "${D8_TEST_KUBERNETES_PATCH_SET_FILE}"
  else
    mktemp -t kubernetes-patch-set.XXXXXXXXXX
  fi
}

function kubernetes::_destroy_patch_set() {
  if [ -n "${D8_IS_TESTS_ENVIRONMENT-}" ]; then
    return 0
  fi

  if [ -e ${D8_KUBERNETES_PATCH_SET_FILE} ]; then
    rm ${D8_KUBERNETES_PATCH_SET_FILE}
  fi
}

function kubernetes::_apply_patch_set() {
  if [ -n "${D8_IS_TESTS_ENVIRONMENT-}" ]; then
    return 0
  fi

  while read -r line
  do
    case "$(jq -r '.op' <<< ${line})" in
    "Create")
      resourceSpec="$(jq -r '.resourceSpec' <<< ${line})"
      kubectl create -f - <<< "${resourceSpec}" >/dev/null
    ;;
    "CreateIfNotExists")
      resourceSpec="$(jq -r '.resourceSpec' <<< ${line})"
      if ! kubectl get -f - <<< "${resourceSpec}" >/dev/null 2>/dev/null; then
        kubectl create -f - <<< "${resourceSpec}" >/dev/null
      fi
    ;;
    "ReplaceOrCreate")
      resourceSpec="$(jq -r '.resourceSpec' <<< ${line})"
      if ! kubectl get -f - <<< "${resourceSpec}" >/dev/null 2>/dev/null; then
        kubectl create -f - <<< "${resourceSpec}" >/dev/null
      else
        kubectl replace --force -f - <<< "${resourceSpec}" >/dev/null
      fi
    ;;
    "JQPatch")
      kubernetes::_jq_patch "$(jq -r '.namespace' <<< ${line})" "$(jq -r '.resource' <<< ${line})" "$(jq -r '.jqFilter' <<< ${line})"
    ;;
    "DeleteIfExists")
      namespace="$(jq -r '.namespace' <<< ${line})"
      resource="$(jq -r '.resource' <<< ${line})"
      if kubectl -n "${namespace}" get "${resource}" >/dev/null 2>&1; then
        kubectl -n "${namespace}" delete "${resource}" >/dev/null 2>&1
      fi
    ;;
    "DeleteIfExistsNonBlocking")
      namespace="$(jq -r '.namespace' <<< ${line})"
      resource="$(jq -r '.resource' <<< ${line})"
      if kubectl -n "${namespace}" get "${resource}" >/dev/null 2>&1; then
        kubectl -n "${namespace}" delete "${resource}" --wait=false >/dev/null 2>&1
      fi
    ;;
    "StatusPatch")
      namespace="$(jq -r '.namespace' <<< ${line})"
      apiVersion="$(jq -r '.apiVersion' <<< ${line})"
      kind="$(jq -r '.kind' <<< ${line})"
      resourceName="$(jq -r '.resourceName' <<< ${line})"
      newStatus="$(jq -r '.newStatus' <<< ${line})"

      if [ -n "${namespace}" ]; then
        apiURL="https://kubernetes.default.svc/apis/${apiVersion}/namespaces/${namespace}/${kind}/${resourceName}/status"
      else
        apiURL="https://kubernetes.default.svc/apis/${apiVersion}/${kind}/${resourceName}/status"
      fi

      curl -XPATCH \
        --resolve "kubernetes.default.svc:443:$KUBERNETES_SERVICE_HOST" \
        -H "Content-Type: application/merge-patch+json" \
        -H "Accept: application/json" \
        -H "Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" \
        "${apiURL}" \
        --cacert /var/run/secrets/kubernetes.io/serviceaccount/ca.crt \
        --data "$(jo status="${newStatus}")" \
      >/dev/null 2>/dev/null
    ;;
    "StatusPut")
      namespace="$(jq -r '.namespace' <<< ${line})"
      apiVersion="$(jq -r '.apiVersion' <<< ${line})"
      kind="$(jq -r '.kind' <<< ${line})"
      resourceName="$(jq -r '.resourceName' <<< ${line})"
      newStatus="$(jq -r '.newStatus' <<< ${line})"

      if [ -n "${namespace}" ]; then
        apiURL="https://kubernetes.default.svc/apis/${apiVersion}/namespaces/${namespace}/${kind}/${resourceName}/status"
      else
        apiURL="https://kubernetes.default.svc/apis/${apiVersion}/${kind}/${resourceName}/status"
      fi

      curl -XPUT \
        --resolve "kubernetes.default.svc:443:$KUBERNETES_SERVICE_HOST" \
        -H "Content-Type: application/merge-patch+json" \
        -H "Accept: application/json" \
        -H "Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" \
        "${apiURL}" \
        --cacert /var/run/secrets/kubernetes.io/serviceaccount/ca.crt \
        --data "$(jo status="${newStatus}")" \
      >/dev/null 2>/dev/null
    ;;
    esac
  done < ${D8_KUBERNETES_PATCH_SET_FILE}
}

# This function has proven to be safe and reliant:
# 1. resourceVersion is passed from the original object to kubectl replace,
#    so optimistic locking is engaged, therefore this function is free of race
#    conditions.
# 2. kubectl replace, while "replacing" object, doesn't lead to emission of
#    DELETE events. If the object is changed – all watching parties will receive
#    just a single UPDATE event (and not a sequence of DELETE and CREATE events).
# 3. If in between getting and replacing, the original object was changed – the
#    function will automatically make up to 5 retries.
function kubernetes::_jq_patch() {
  local namespace="$1"
  local resource="$2"
  local filter="$3"

  local a=$(mktemp)
  local b=$(mktemp)
  local tmp=$(mktemp)

  local cleanup_filter='. | del(
    .metadata.annotations."kubectl.kubernetes.io/last-applied-configuration"
  )'

  success=false
  err=""
  for attempt in $(seq 1 5) ; do
    if ! kubectl -n "$namespace" get "$resource" -o json > $tmp ||
       ! jq -S "$cleanup_filter" $tmp > $a ||
       ! jq -S "$filter" $a > $b ;
    then
      echo FILTER: "$filter"

      echo "Before JQ"
      cat $a
      echo "After JQ"
      cat $b

      rm $a $b $tmp
      return 1
    fi

    if diff -u $a $b || err=$(kubectl replace -f $b 2>&1 1>/dev/null); then
      success=true
      break
    else
      echo "NOTICE: attempt #$attempt to patch kubernetes resource $namespace/$resource failed, retrying patch"
    fi
  done

  rm $a $b $tmp
  if [[ "$success" == "true" ]]; then
    return 0
  else
    >&2 echo "ERROR: Couldn't patch kubernetes resource $namespace/$resource. $err"
    return 1
  fi
}
