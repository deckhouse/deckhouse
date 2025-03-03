#!/usr/bin/env bash

# Copyright 2025 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

PVC_NAMESPACE="$1"
PVC_NAME="$2"
FLAG="$3"
MIGRATOR_NAME="migrate-pvc-$PVC_NAME"

if [ -z "$PVC_NAMESPACE" ] || [ -z "$PVC_NAME" ]; then
  echo "Usage: migrate-pvc.sh <pvcNamespace> <pvcName> --yes-i-updated-all-cluster-nodes-to-ubuntu-20-04"
  exit 1
fi

if [ "$FLAG" != "--yes-i-updated-all-cluster-nodes-to-ubuntu-20-04" ]; then
  echo "IMPORTANT!!!"
  echo "Node that will request new CNS volume should be HW Version >=15."
  echo "We have built new templates that are based on hw_version=15: ubuntu-focal-20.04-packer."
  echo "Please make sure that your Nodes were rolled out from that template."
  echo "In other case migration process will stuck and you will get downtime."
  echo ""
  echo "Confirm that by providing a flag: --yes-i-updated-all-cluster-nodes-to-ubuntu-20-04"

  exit 1
fi

echo "Removing migrator pod"
kubectl -n "$PVC_NAMESPACE" delete pod "$MIGRATOR_NAME"

echo "Get controller resource"
CONTROLLER_JSON_TMP_FILE="/tmp/migrate-pvc-$PVC_NAME.controller.tmp"
if [ -f "$CONTROLLER_JSON_TMP_FILE" ]; then
  echo "Reading controller resource from early stored tmp file."
  CONTROLLER_JSON="$(cat "$CONTROLLER_JSON_TMP_FILE")"
  CONTROLLER="$(echo "$CONTROLLER_JSON" | jq '.kind + "/" + .metadata.name' -r)"
else
  CONTROLLER="$(kubectl get pods -n "$PVC_NAMESPACE" -o json | jq --arg pvcName "$PVC_NAME" -c '
  [
    .items[] | . as $pod | .spec
    | select(has("volumes")).volumes[]
    | select(has("persistentVolumeClaim"))
    | select(.persistentVolumeClaim.claimName == $pvcName)
    | [$pod.metadata.ownerReferences[] | select(.controller == true)] | first
    | .kind + "/" + .name
  ] | first' -r)"
  if [ -z "$CONTROLLER" ]; then
    echo "Controller of $PVC_NAME not found in namespace "$PVC_NAMESPACE". Possibly Pod is not running?"
  else
    CONTROLLER_JSON="$(kubectl -n "$PVC_NAMESPACE" get "$CONTROLLER" -o json)"
    if [ -z "$CONTROLLER_JSON" ]; then
      echo "Cant get controller resource from the cluster."
    else
      echo "Storing controller json to tmp file, just in case."
      echo "$CONTROLLER_JSON" > "$CONTROLLER_JSON_TMP_FILE"
    fi
  fi
fi

echo "Get PVC resource"
PVC_JSON_TMP_FILE="/tmp/migrate-pvc-$PVC_NAME.pvc.tmp"
if [ -f "$PVC_JSON_TMP_FILE" ]; then
  echo "Reading PVC resource from early stored tmp file."
  PVC_JSON="$(cat "$PVC_JSON_TMP_FILE")"
else
  PVC_JSON="$(kubectl -n "$PVC_NAMESPACE" get pvc "$PVC_NAME" -o json)"
  if [ -z "$PVC_JSON" ]; then
    echo "Cant get PVC resource from the cluster."
  else
    echo "Storing old pvc to tmp file, just in case."
    echo "$PVC_JSON" > "$PVC_JSON_TMP_FILE"
  fi
fi

if [ -z "$CONTROLLER_JSON" ] || [ -z "$PVC_JSON" ]; then
  if [ -z "$CONTROLLER_JSON" ]; then
    echo "Error, cant get controller resource from cluster"
  fi
  if [ -z "$PVC_JSON" ]; then
    echo "Error, cant get PVC resource from cluster"
  fi
  exit 1
fi

PV_NAME="$(echo "$PVC_JSON" | jq .spec.volumeName -r)"
PV_SIZE="$(kubectl get pv "$PV_NAME" -o json | jq .spec.capacity.storage -r)"

echo "Patch old PV reclamation policy to Retain, to be sure it won't be deleted with PVC"
kubectl patch pv "$PV_NAME" -p '{"spec":{"persistentVolumeReclaimPolicy":"Retain"}}'
echo "Check that PV is actually has Retain reclamation policy to not loose data"
if ! kubectl get pv "$PV_NAME" -o json | jq '.spec.persistentVolumeReclaimPolicy == "Retain"' -e > /dev/null; then
  echo "PV $PV_NAME wasn't successfully patched to Retain reclamation policy. Exiting."
  exit 1
fi

if [ "$PVC_NAMESPACE" == "d8-monitoring" ]; then
  echo "Scaling prometheus-operator to 0 replicas"
  kubectl -n d8-operator-prometheus scale deployment prometheus-operator --replicas=0
  echo "Scaling deckhouse to 0 replicas (to prevent sts deletion)"
  kubectl -n d8-system scale deployment deckhouse --replicas=0
  echo "Waiting 10 seconds to operators shutdown successfully"
  sleep 10
fi
echo "Scaling controller to 0 replicas"
kubectl -n "$PVC_NAMESPACE" scale "$CONTROLLER" --replicas=0


echo "Delete old PVC"
kubectl -n "$PVC_NAMESPACE" delete pvc "$PVC_NAME"

echo "Create tmp PVC targeting to old PV"
TMP_PVC_NAME="$(echo "$PVC_JSON" | jq .metadata.name -r)"
TMP_PVC_NAME="tmp-$TMP_PVC_NAME"
TMP_PVC_JSON_TEMPLATE="$(jq --arg pvSize "$PV_SIZE" --arg tmpPvcName "$TMP_PVC_NAME" '
{
  "apiVersion": "v1",
  "kind": "PersistentVolumeClaim",
  "metadata": {
    "name": $tmpPvcName,
    "namespace": .metadata.namespace,
    "labels": .metadata.labels
  },
  "spec": {
    "accessModes": .spec.accessModes,
    "resources": {
      "requests": {
        "storage": $pvSize
      }
    },
    "storageClassName": .spec.storageClassName,
    "volumeMode": .spec.volumeMode,
    "volumeName": .spec.volumeName
  }
}' <<< "$PVC_JSON")"
echo "$TMP_PVC_JSON_TEMPLATE" | kubectl create -f -

echo "Patch old PV claimRef to tmp PVC"
TMP_PVC_JSON="$(kubectl -n "$PVC_NAMESPACE" get pvc "$TMP_PVC_NAME" -o json)"
PV_PATCH="$(jq '
{
  "spec": {
    "claimRef": {
      "name": .metadata.name,
      "resourceVersion": .metadata.resourceVersion,
      "uid": .metadata.uid
    },
  }
}' <<< "$TMP_PVC_JSON")"
kubectl patch pv "$PV_NAME" -p ''"$PV_PATCH"''

echo "Create new PVC"
NEW_PVC_JSON_TEMPLATE="$(echo "$TMP_PVC_JSON_TEMPLATE" | jq --arg pvcName "$PVC_NAME" '.metadata.name |= $pvcName | del(.spec.volumeName)')"
echo "$NEW_PVC_JSON_TEMPLATE" | kubectl create -f -

echo "Run static Pod to rsync tmp (old) -> new."
MIGRATOR_TEMPLATE="$(jq --arg name "$MIGRATOR_NAME" --arg namespace "$PVC_NAMESPACE" --arg pvcName "$PVC_NAME" --arg tmpPvcName "$TMP_PVC_NAME" '
{
  "apiVersion": "v1",
  "kind": "Pod",
  "metadata": {
    "name": $name,
    "namespace": $namespace
  },
  "spec": {
    "containers": [
      {
        "name": "migrator",
        "image": "gcr.io/google-containers/alpine-with-bash@sha256:0955672451201896cf9e2e5ce30bec0c7f10757af33bf78b7a6afa5672c596f5",
        "command": [
          "/bin/sh",
          "-c"
        ],
        "args": [
          "apk update && apk add rsync && rsync -avP /tmp/pvc-from/ /tmp/pvc-to/"
        ],
        "volumeMounts": [
          {
            "mountPath": "/tmp/pvc-from",
            "name": "pvc-from",
            "readOnly": true
          },
          {
            "mountPath": "/tmp/pvc-to",
            "name": "pvc-to",
            "readOnly": false
          }
        ]
      }
    ],
    "affinity": (.spec.template.spec.affinity // {}),
    "nodeSelector": (.spec.template.spec.nodeSelector // {}),
    "tolerations": (.spec.template.spec.tolerations // []),
    "restartPolicy": "Never",
    "volumes": [
      {
        "name": "pvc-from",
        "persistentVolumeClaim": {
          "claimName": $tmpPvcName,
          "readOnly": true
        }
      },
      {
        "name": "pvc-to",
        "persistentVolumeClaim": {
          "claimName": $pvcName,
          "readOnly": false
        }
      }
    ]
  }
}' <<< "$CONTROLLER_JSON")"
echo "$MIGRATOR_TEMPLATE" | kubectl create -f -

echo "Wait until pod is finished"
n=0
error=0
pod_phase="unknown"
while true ; do
  if pod=$(timeout 10 kubectl -n "$PVC_NAMESPACE" get pod "$MIGRATOR_NAME" -o json 2> /dev/null) ; then
    pod_phase=$(echo "$pod" | jq '.status.phase' -r 2> /dev/null)

    if [[ "$pod_phase" != "Succeeded" ]] && [[ "$pod_phase" != "Failed" ]] ; then
      echo " * Pod $MIGRATOR_NAME phase $pod_phase does not match expected Succeeded|Failed, sleeping for 5 seconds..."
    else
      echo " * Pod $MIGRATOR_NAME reached expected phase $pod_phase"
      break
    fi
  else
    echo " * Failed to get pod $MIGRATOR_NAME"
  fi

  n=$((n + 1))
  if [[ $n -gt 480 ]] ; then
    echo " * Fatal error: Timeout waiting for pod $MIGRATOR_NAME to finish"
    error=1
  fi

  sleep 5
done

echo "Rsync Pod finished in phase $pod_phase with log:"
logs="$(kubectl -n "$PVC_NAMESPACE" logs "$MIGRATOR_NAME")"
echo "$logs"

echo "Delete migrator pod"
kubectl -n "$PVC_NAMESPACE" delete pod "$MIGRATOR_NAME"

if [ "$error" == "1" ]; then
  exit 1
fi

echo "Delete tmp PVC"
kubectl -n "$PVC_NAMESPACE" delete pvc "$TMP_PVC_NAME"

echo "Scale controller to last replicas number"
CONTROLLER_REPLICAS="$(echo "$CONTROLLER_JSON" | jq '.spec.replicas' -r)"
kubectl -n "$PVC_NAMESPACE" scale "$CONTROLLER" --replicas="$CONTROLLER_REPLICAS"
if [ "$PVC_NAMESPACE" == "d8-monitoring" ]; then
  echo "Scaling prometheus-operator to 1 replicas"
  kubectl -n d8-operator-prometheus scale deployment prometheus-operator --replicas=1
  echo "Scaling deckhouse to 1 replicas"
  kubectl -n d8-system scale deployment deckhouse --replicas=1
fi

echo "Migration is finished. Please make sure that application working as expected and then delete old PV with following command:"
echo "kubectl delete pv $PV_NAME"

rm -rf "$PVC_JSON_TMP_FILE"
rm -rf "$CONTROLLER_JSON_TMP_FILE"
