#!/bin/bash

function common_hooks::storage_class_change::config() {
  namespace=$1
  label_key="$(awk '{gsub("="," "); print $1}' <<< "$2")"
  label_value="$(awk '{gsub("="," "); print $2}' <<< "$2")"

  cat << EOF
    configVersion: v1
    beforeHelm: 1
    kubernetes:
    - name: pvc
      keepFullObjectsInMemory: false
      group: main
      apiVersion: v1
      kind: PersistentVolumeClaim
      namespace:
        nameSelector:
          matchNames: ["$namespace"]
      labelSelector:
        matchLabels:
          $label_key: "$label_value"
      jqFilter: |
        {
          "pvcName": .metadata.name,
          "storageClassName": .spec.storageClassName
        }
    - name: default_sc
      group: main
      keepFullObjectsInMemory: false
      apiVersion: storage.k8s.io/v1
      kind: Storageclass
      jqFilter: |
        {
          "name": .metadata.name,
          "isDefault": (.metadata.annotations."storageclass.beta.kubernetes.io/is-default-class" == "true" or .metadata.annotations."storageclass.kubernetes.io/is-default-class" == "true")
        }
# pvc_modified
    - name: pvc_modified
      keepFullObjectsInMemory: false
      group: pvc_modified
      executeHookOnEvent: ["Modified"]
      executeHookOnSynchronization: false
      apiVersion: v1
      kind: PersistentVolumeClaim
      namespace:
        nameSelector:
          matchNames: ["$namespace"]
      labelSelector:
        matchLabels:
          $label_key: $label_value
      jqFilter: |
        {
          "pvcName": .metadata.name,
          "isDeleted": (if .metadata | has("deletionTimestamp") then true else false end)
        }
    - name: pods
      keepFullObjectsInMemory: false
      group: pvc_modified
      executeHookOnEvent: []
      executeHookOnSynchronization: false
      apiVersion: v1
      kind: Pod
      namespace:
        nameSelector:
          matchNames: ["$namespace"]
      labelSelector:
        matchLabels:
          $label_key: $label_value
      jqFilter: |
        {
          "podName": .metadata.name,
          "pvcName": ([.spec.volumes // [] | .[] | select(has("persistentVolumeClaim"))] | first.persistentVolumeClaim.claimName)
        }
# pvc_deleted
    - name: pvc_deleted
      group: pvc_deleted
      keepFullObjectsInMemory: false
      executeHookOnEvent: ["Deleted"]
      executeHookOnSynchronization: false
      apiVersion: v1
      kind: PersistentVolumeClaim
      namespace:
        nameSelector:
          matchNames: ["$namespace"]
      labelSelector:
        matchLabels:
          $label_key: $label_value
      jqFilter: |
        {
          "pvcName": .metadata.name
        }
    - name: pods
      group: pvc_deleted
      keepFullObjectsInMemory: false
      executeHookOnEvent: []
      executeHookOnSynchronization: false
      apiVersion: v1
      kind: Pod
      namespace:
        nameSelector:
          matchNames: ["$namespace"]
      labelSelector:
        matchLabels:
          $label_key: $label_value
      jqFilter: |
        {
          "podName": .metadata.name,
          "pvcName": ([.spec.volumes // [] | .[] | select(has("persistentVolumeClaim"))] | first.persistentVolumeClaim.claimName),
          "phase": (.status.phase // "")
        }
EOF
}

function common_hooks::storage_class_change::pvc_modified() {
  namespace=$1
  for pvc_name in $(context::jq -rc '.snapshots.pvc_modified[].filterResult.pvcName'); do
    # If someone deleted pvc then delete the pod.
    if pod_name="$(context::jq -er --arg pvc_name "$pvc_name" '.snapshots.pods[].filterResult | select(.pvcName == $pvc_name) | .podName')" >/dev/null ; then
      kubernetes::delete_if_exists::non_blocking "$namespace" "pod/$pod_name"
      echo "!!! NOTICE: deleting pod/$pod_name because persistentvolumeclaim/$pvc_name stuck in Terminating state !!!"
    fi
  done
}

function common_hooks::storage_class_change::pvc_deleted() {
  namespace=$1
  # If pvc was deleted and pod in phase Pending -- delete him.
  for pod_name in $(context::jq -rc '.snapshots.pods[].filterResult | select(.phase == "Pending") | .podName'); do
    kubernetes::delete_if_exists::non_blocking "$namespace" "pod/$pod_name"
    echo "!!! NOTICE: deleting pod/$pod_name because persistentvolumeclaim was deleted !!!"
  done
}

function common_hooks::storage_class_change::main() {
  namespace="$1"
  object_kind="$2"
  object_name="$3"
  internal_path=""
  if [ $# -gt 3 ]; then
    internal_path=".$4"
  fi
  config_storage_class_param_name="storageClass"
  if [ $# -gt 4 ]; then
    config_storage_class_param_name="$5"
  fi

  module_name="$(module::name::camel_case)"

  effective_storage_class="false"
  current_storage_class="false"

  if context::jq -er '.snapshots.default_sc[] | select(.filterResult.isDefault == true)' >/dev/null; then
    effective_storage_class="$(context::jq -r '[.snapshots.default_sc[] | select(.filterResult.isDefault == true)] | first | .filterResult.name')"
  fi

  if values::has --config global.storageClass; then
    effective_storage_class="$(values::get --config global.storageClass)"
  fi

  if context::has snapshots.pvc.0; then
    effective_storage_class="$(context::get snapshots.pvc.0.filterResult.storageClassName)"
    current_storage_class="$effective_storage_class"
  fi

  if values::has --config $module_name.${config_storage_class_param_name}; then
    effective_storage_class="$(values::get --config $module_name.${config_storage_class_param_name})"
  fi

  values::set ${module_name}.internal${internal_path}.effectiveStorageClass "$effective_storage_class"

  if [ "$current_storage_class" != "$effective_storage_class" ]; then
    for pvc_name in $(context::jq -rc '.snapshots.pvc[].filterResult.pvcName'); do
      kubernetes::delete_if_exists::non_blocking "$namespace" "persistentvolumeclaim/$pvc_name"
      echo "!!! NOTICE: storage class changed, deleting persistentvolumeclaim/$pvc_name !!!"
    done
    kubernetes::delete_if_exists "$namespace" "$object_kind/$object_name"
    echo "!!! NOTICE: storage class changed, deleting $object_kind/$object_name !!!"
  fi
}
