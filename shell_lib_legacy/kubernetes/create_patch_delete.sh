#!/bin/bash

function kubernetes::init_patch_set() {
  if [ -n "${D8_KUBERNETES_PATCH_SET_FILE-}" ]; then
    echo "${D8_KUBERNETES_PATCH_SET_FILE}"
  else
    #mktemp -t "$(module::name)-XXXXXX"
    mktemp
  fi
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
function kubernetes::replace_json() {
  cat | jq -c '{op: "Replace", resourceSpec: (. | tojson)}' >> ${D8_KUBERNETES_PATCH_SET_FILE}
}

# stdin resource_spec
function kubernetes::replace_yaml() {
  cat | yq r -j - | jq -c '{op: "Replace", resourceSpec: (. | tojson)}' >> ${D8_KUBERNETES_PATCH_SET_FILE}
}

# $1 namespace
# $2 resource (pod/mypod-aacc12)
# $3 jqFilter
function kubernetes::patch_jq() {
  jq -nc --arg jqFilter "${3}" '{op: "JQPatch", namespace: "'${1}'", resource: "'${2}'", jqFilter: $jqFilter}' >> ${D8_KUBERNETES_PATCH_SET_FILE}
}

# $1 namespace
# $2 resource (pod/mypod-aacc12)
function kubernetes::delete() {
  jq -nc '{op: "Delete", namespace: "'${1}'", resource: "'${2}'"}' >> ${D8_KUBERNETES_PATCH_SET_FILE}
}

function kubernetes::apply_patch_set() {
  if [ -n "${D8_IS_TESTS_ENVIRONMENT-}" ]; then
    return 0
  fi

  while read -r line
  do
    case "$(jq -r '.op' <<< ${line})" in
    "CreateIfNotExists")
      resourceSpec="$(jq -r '.resourceSpec' <<< ${line})"
      if ! kubectl get -f - <<< "${resourceSpec}" >/dev/null 2>/dev/null; then
        kubectl create -f - <<< "${resourceSpec}" >/dev/null
      fi
    ;;
    "Replace")
      kubectl::replace_or_create <<< "$(jq -r '.resourceSpec' <<< ${line})"
    ;;
    "JQPatch")
      kubectl::jq_patch "$(jq -r '.namespace' <<< ${line})" "$(jq -r '.resource' <<< ${line})" "$(jq -r '.jqFilter' <<< ${line})"
    ;;
    "Delete")
      namespace="$(jq -r '.namespace' <<< ${line})"
      resource="$(jq -r '.resource' <<< ${line})"
      if kubectl -n "${namespace}" get "${resource}" >/dev/null 2>&1; then
        kubectl -n "${namespace}" delete "${resource}" >/dev/null 2>&1
      fi
    ;;
    esac
  done < ${D8_KUBERNETES_PATCH_SET_FILE}
  rm ${D8_KUBERNETES_PATCH_SET_FILE}
}
